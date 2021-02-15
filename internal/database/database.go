package database

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/knakk/sparql"

	"github.com/apache/arrow/go/arrow"
	"github.com/apache/arrow/go/arrow/array"
	"github.com/apache/arrow/go/arrow/ipc"
	"github.com/apache/arrow/go/arrow/memory"

	"github.com/gtfierro/mortar2/internal/config"
	"github.com/gtfierro/mortar2/internal/logging"
)

// TODO: updating Brick model should update types in the 'stream' table

// Database defines the interface to the underlying data store
type Database interface {
	Close()
	RunAsTransaction(context.Context, func(txn pgx.Tx) error) error
	RegisterStream(context.Context, Stream) error
	InsertHistoricalData(ctx context.Context, ds Dataset) error
	ReadDataChunk(context.Context, io.Writer, *Query) error
	QuerySparqlWriter(context.Context, io.Writer, string, io.Reader) error
	QuerySparql(context.Context, string, string) (*sparql.Results, error)
	Qualify(context.Context, []string) (map[string][]int, error)
	AddTriples(context.Context, TripleDataset) error
}

// TimescaleDatabase is an implementation of Database for TimescaleDB
type TimescaleDatabase struct {
	pool            *pgxpool.Pool
	reasonerAddress string
}

// NewTimescaleInsecureDefaults creates a new TimescaleDatabase with the insecure default settings: (listening localhost:5434 with user/pass = mortarchangeme/mortarpasswordchangeme)
func NewTimescaleInsecureDefaults(ctx context.Context) (Database, error) {
	cfg := &config.Config{
		Database: config.Database{
			Host:     "localhost",
			Database: "mortar",
			User:     "mortarchangeme",
			Password: "mortarpasswordchangeme",
			Port:     "5434",
		},
		Reasoner: config.Reasoner{
			Address: "localhost:3030",
		},
	}
	return NewTimescaleFromConfig(ctx, cfg)
}

// NewTimescaleFromConfig creates a new TimescaleDatabase with the given configuration
func NewTimescaleFromConfig(ctx context.Context, cfg *config.Config) (Database, error) {
	var err error

	if err := checkConfig(cfg); err != nil {
		return nil, fmt.Errorf("Invalid config to connect to database: %w", err)
	}
	// TODO: add the following config instead of a connection URL
	dbURL := fmt.Sprintf("postgres://%s/%s?sslmode=disable&user=%s&password=%s&port=%s",
		cfg.Database.Host, cfg.Database.Database, cfg.Database.User, url.QueryEscape(cfg.Database.Password), cfg.Database.Port)
	connCfg, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		return nil, fmt.Errorf("Invalid config to connect to database: %w", err)
	}
	connCfg.MaxConns = 50
	connCfg.MaxConnIdleTime = 15 * time.Minute
	connCfg.MaxConnLifetime = 15 * time.Minute

	log := logging.FromContext(ctx)
	// loop until database is live
	var pool *pgxpool.Pool
	for {
		pool, err = pgxpool.ConnectConfig(ctx, connCfg)
		if err != nil {
			log.Warnf("Failed to connect to database (%s); retrying in 5 seconds", err.Error())
			time.Sleep(5 * time.Second)
		} else {
			break
		}
	}
	log.Infof("Connected to postgres at %s", cfg.Database.Host)
	return &TimescaleDatabase{
		pool:            pool,
		reasonerAddress: cfg.Reasoner.Address,
	}, nil
}

// Close shuts down the connections to the database
func (db *TimescaleDatabase) Close() {
	db.pool.Close()
}

// RunAsTransaction executes the provided function in a transaction; commits if the function returns nil, and aborts otherwise
func (db *TimescaleDatabase) RunAsTransaction(ctx context.Context, f func(txn pgx.Tx) error) error {
	// start transaction in a new pooled connection
	conn, err := db.pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("Could not acquire connection from pool: %w", err)
	}
	defer conn.Release()
	txn, err := conn.Begin(ctx)
	if err != nil {
		return fmt.Errorf("Could not begin transaction: %w", err)
	}
	if err := f(txn); err != nil {
		if rberr := txn.Rollback(ctx); rberr != nil {
			return fmt.Errorf("Error (%s) occured during transaction. Could not rollback: %s", err, rberr)
		}
		return fmt.Errorf("Error occured during transaction execution: %w", err)
	}
	if err := txn.Commit(ctx); err != nil {
		return fmt.Errorf("Error occured during transaction commit: %w", err)
	}
	return nil
}

func (db *TimescaleDatabase) RegisterStream(ctx context.Context, stream Stream) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	log := logging.FromContext(ctx)

	if err := checkStream(&stream); err != nil {
		return fmt.Errorf("Cannot register invalid stream: %w", err)
	}

	var registered = false
	err := db.RunAsTransaction(ctx, func(txn pgx.Tx) error {
		var (
			brickURI   *string
			brickClass *string
		)
		if len(stream.BrickURI) > 0 {
			brickURI = &stream.BrickURI
		}
		if len(stream.BrickClass) > 0 {
			brickClass = &stream.BrickClass
		}

		res, err := txn.Exec(ctx, `INSERT INTO streams(id, name, source, units, brick_uri, brick_class)
								 VALUES(DEFAULT, $1, $2, $3, $4, $5) ON CONFLICT (source, name) DO UPDATE
								 SET brick_uri = EXCLUDED.brick_uri,
								     brick_class = EXCLUDED.brick_class,
									 units = EXCLUDED.units`,
			stream.Name, stream.SourceName, stream.Units, brickURI, brickClass)
		if err != nil {
			return fmt.Errorf("Could not register stream: %w", err)
		}
		registered = res.RowsAffected() > 0

		// TODO: register as a Triple
		if brickURI != nil && len(*brickURI) > 0 {
			s := fmt.Sprintf("<%s>", *brickURI)
			p := "<http://www.w3.org/1999/02/22-rdf-syntax-ns#type>"
			o := "<https://brickschema.org/schema/Brick#Point>"
			if brickClass != nil && len(*brickClass) > 0 {
				o = fmt.Sprintf("<%s>", *brickClass)
			}
			res, err = txn.Exec(ctx, `INSERT INTO triples(source, origin, time, s, p, o)
								 VALUES($1, $2, $3, $4, $5, $6) ON CONFLICT DO NOTHING`,
				stream.SourceName, "stream_registration", time.Now(), s, p, o)
			if err != nil {
				return fmt.Errorf("Could not register stream: %w", err)
			}
		}

		return nil
	})

	if err == nil && registered {
		log.Infof("Registered Stream %s", stream.String())
	}
	return err
}

func (db *TimescaleDatabase) InsertHistoricalData(ctx context.Context, ds Dataset) error {
	ctx, cancel := context.WithTimeout(ctx, config.DataWriteTimeout)
	defer cancel()

	log := logging.FromContext(ctx)

	if err := checkDataset(ds); err != nil {
		return fmt.Errorf("Cannot handle invalid dataset: %w", err)
	}

	var num int64 = 0
	err := db.RunAsTransaction(ctx, func(txn pgx.Tx) error {
		// check valid stream
		row := txn.QueryRow(ctx, `SELECT id FROM streams WHERE source=$1 AND name=$2`, ds.GetSource(), ds.GetName())
		var stream_id int
		err := row.Scan(&stream_id)
		if err != nil {
			return fmt.Errorf("No such stream (SourceName: %s, Name: %s): %w", ds.GetSource(), ds.GetName(), err)
		}

		ds.SetId(stream_id)
		_, err = txn.Exec(ctx, "CREATE TEMP TABLE datat(time TIMESTAMPTZ, stream_id INTEGER, value FLOAT)")
		if err != nil {
			return fmt.Errorf("Cannot insert readings for id %d: %w", stream_id, err)
		}

		num, err = txn.CopyFrom(ctx, pgx.Identifier{"datat"}, []string{"time", "stream_id", "value"}, ds)
		if err != nil {
			return fmt.Errorf("Cannot insert readings for id %d: %w", stream_id, err)
		}

		_, err = txn.Exec(ctx, "INSERT INTO data SELECT * FROM datat ON CONFLICT (time, stream_id) DO UPDATE SET value = EXCLUDED.value")
		if err != nil {
			return fmt.Errorf("Cannot insert readings for id %d: %w", stream_id, err)
		}

		_, err = txn.Exec(ctx, "DROP TABLE datat")
		if err != nil {
			return fmt.Errorf("Cannot insert readings for id %d: %w", stream_id, err)
		}

		//for rdg := range ds.GetReadings() {
		//	_, err := txn.Exec(ctx, `INSERT INTO data(time, stream_id, value) VALUES($1, $2, $3)  ON CONFLICT (time, stream_id) DO UPDATE SET value = EXCLUDED.value;`, rdg.Time, stream_id, rdg.Value)
		//	if err != nil {
		//		return fmt.Errorf("Cannot insert reading %v for id %d: %w", rdg, stream_id, err)
		//	}
		//	num++
		//}

		return nil

	})

	if err == nil {
		log.Infof("Inserted %5d readings: %s", num, ds)
	}
	return err
}

func (db *TimescaleDatabase) writeMetadataArrow(ctx context.Context, w io.Writer, q *Query) error {
	// if a sparql query is provided, then execute it, join on 'streams' to get all of the ids
	// implied by the query, and use those to determine the ids in the 'data' table
	var err error
	if len(q.Sparql) > 0 {
		fmt.Println("SPARQL", q.Sparql)
		// TODO: get all graphs
		res, err := db.QuerySparql(ctx, "all", q.Sparql)
		if err != nil {
			return err
		}

		var uris []string
		for _, row := range res.Results.Bindings {
			for _, value := range row {
				if value.Type == "uri" {
					uris = append(uris, value.Value)
				}
			}
		}
		// get ids from the uris
		var rows pgx.Rows
		if len(q.Sources) > 0 {
			rows, err = db.pool.Query(ctx, `SELECT id from streams WHERE (name = ANY($1) OR brick_uri = ANY($1)) AND source = ANY($2)`, uris, q.Sources)
		} else {
			rows, err = db.pool.Query(ctx, `SELECT id from streams WHERE (name = ANY($1) OR brick_uri = ANY($1))`, uris)
		}
		if err != nil {
			return err
		}
		for rows.Next() {
			var i int64
			if err := rows.Scan(&i); err != nil {
				return fmt.Errorf("Could not query: %w", err)
			}
			q.Ids = append(q.Ids, i)
		}
	} else if len(q.Uris) > 0 {
		var rows pgx.Rows
		if len(q.Sources) > 0 {
			rows, err = db.pool.Query(ctx, `SELECT id from streams WHERE (name = ANY($1) OR brick_uri = ANY($1)) AND source = ANY($2)`, q.Uris, q.Sources)
		} else {
			rows, err = db.pool.Query(ctx, `SELECT id from streams WHERE (name = ANY($1) OR brick_uri = ANY($1))`, q.Uris)
		}
		if err != nil {
			return err
		}
		for rows.Next() {
			var i int64
			if err := rows.Scan(&i); err != nil {
				return fmt.Errorf("Could not query: %w", err)
			}
			q.Ids = append(q.Ids, i)
		}
	}

	metadataFields := []arrow.Field{
		{Name: "brick_class", Type: arrow.BinaryTypes.String, Nullable: true},
		{Name: "brick_uri", Type: arrow.BinaryTypes.String, Nullable: true},
		{Name: "units", Type: arrow.BinaryTypes.String, Nullable: true},
		{Name: "name", Type: arrow.BinaryTypes.String, Nullable: true},
		{Name: "stream_id", Type: arrow.PrimitiveTypes.Int64, Nullable: false},
	}
	mdsch := arrow.NewSchema(metadataFields, nil)
	mdbldr := array.NewRecordBuilder(memory.DefaultAllocator, mdsch)
	defer mdbldr.Release()

	classes := mdbldr.Field(0).(*array.StringBuilder)
	uris := mdbldr.Field(1).(*array.StringBuilder)
	units := mdbldr.Field(2).(*array.StringBuilder)
	names := mdbldr.Field(3).(*array.StringBuilder)
	ids := mdbldr.Field(4).(*array.Int64Builder)
	mdWriter := ipc.NewWriter(w, ipc.WithSchema(mdbldr.Schema()))

	// query stream metadata
	rows, err := db.pool.Query(ctx, `SELECT DISTINCT stream_id, brick_class, brick_uri, units, name FROM unified WHERE stream_id = ANY($1)`, q.Ids)
	if err != nil {
		return err
	}
	for rows.Next() {
		var id int64
		var brick_class string
		var brick_uri string
		var unit string
		var name string
		if err := rows.Scan(&id, &brick_class, &brick_uri, &unit, &name); err != nil {
			return fmt.Errorf("Could not query: %w", err)
		}
		classes.Append(brick_class)
		uris.Append(brick_uri)
		units.Append(unit)
		names.Append(name)
		ids.Append(id)
	}

	mdrec := mdbldr.NewRecord()
	if err := mdWriter.Write(mdrec); err != nil {
		return fmt.Errorf("Could not write record %w", err)
	}
	defer mdrec.Release()

	// finish sending metadata
	return mdWriter.Close()
}

func (db *TimescaleDatabase) ReadDataChunk(ctx context.Context, w io.Writer, q *Query) error {
	ctx, cancel := context.WithTimeout(ctx, config.DataReadTimeout)
	defer cancel()

	if err := db.writeMetadataArrow(ctx, w, q); err != nil {
		return fmt.Errorf("Error processing metadata: %w", err)
	}

	// TODO: need to do a better job of streaming this data out

	sch := arrow.NewSchema([]arrow.Field{
		{Name: "time", Type: arrow.FixedWidthTypes.Timestamp_ns, Nullable: false},
		{Name: "value", Type: arrow.PrimitiveTypes.Float64, Nullable: false},
		{Name: "id", Type: arrow.BinaryTypes.String, Nullable: false},
	}, nil)
	bldr := array.NewRecordBuilder(memory.DefaultAllocator, sch)
	defer bldr.Release()

	rTimes := bldr.Field(0).(*array.TimestampBuilder)
	rValues := bldr.Field(1).(*array.Float64Builder)
	rNames := bldr.Field(2).(*array.StringBuilder)

	arrowWriter := ipc.NewWriter(w, ipc.WithSchema(bldr.Schema()))

	var (
		rows pgx.Rows
		err  error
	)
	// write aggregation query if Query contains it
	if q.AggregationFunc != nil && q.AggregationWindow != nil {
		sql := fmt.Sprintf(`SELECT time_bucket('%s', time) as time, %s, COALESCE(brick_uri, name)
							FROM unified WHERE time>=$1 and time <=$2 and stream_id = ANY($3)
							GROUP BY time, stream_id
							ORDER BY stream_id, time`, *q.AggregationWindow, q.AggregationFunc.toSQL("value"))
		rows, err = db.pool.Query(ctx, sql, q.Start.Format(time.RFC3339), q.End.Format(time.RFC3339), q.Ids)
	} else {
		rows, err = db.pool.Query(ctx, `SELECT time, value, COALESCE(brick_uri, name)
										FROM unified WHERE time>=$1 and time <=$2 and stream_id = ANY($3)
										ORDER BY stream_id, time`, q.Start.Format(time.RFC3339), q.End.Format(time.RFC3339), q.Ids)
	}

	if err != nil {
		return fmt.Errorf("Could not query %w", err)
	}
	for rows.Next() {
		var (
			t time.Time
			v float64
			s string
		)
		if err := rows.Scan(&t, &v, &s); err != nil {
			return fmt.Errorf("Could not query %w", err)
		}
		rTimes.Append(arrow.Timestamp(t.UnixNano()))
		rValues.Append(v)
		rNames.Append(s)

		// TODO: measure/estimate size
		if rValues.Len() > 2000000 { // 2 million readings
			rec := bldr.NewRecord()

			if err := arrowWriter.Write(rec); err != nil {
				return fmt.Errorf("Could not write record %w", err)
			}
			rec.Release()
		}
	}

	rec := bldr.NewRecord()
	defer rec.Release()

	if err := arrowWriter.Write(rec); err != nil {
		return fmt.Errorf("Could not write record %w", err)
	}

	return arrowWriter.Close()
}

func (db *TimescaleDatabase) QuerySparqlWriter(ctx context.Context, w io.Writer, graph string, query io.Reader) error {
	ctx, cancel := context.WithTimeout(ctx, config.DataReadTimeout)
	defer cancel()
	if len(graph) == 0 {
		graph = "default"
	}

	queryURL := fmt.Sprintf("http://%s/query/%s", db.reasonerAddress, graph)
	resp, err := http.Post(queryURL, "application/json", query)
	if err != nil {
		return fmt.Errorf("Could not query %w", err)
	}
	defer resp.Body.Close()
	_, err = io.Copy(w, resp.Body)
	return err
}

func (db *TimescaleDatabase) QuerySparql(ctx context.Context, graph string, queryString string) (*sparql.Results, error) {
	ctx, cancel := context.WithTimeout(ctx, config.DataReadTimeout)
	defer cancel()

	if len(graph) == 0 {
		graph = "default"
	}
	repo, err := sparql.NewRepo(fmt.Sprintf("http://%s/query/%s", db.reasonerAddress, graph))
	if err != nil {
		return nil, fmt.Errorf("Could not connect to SPARQL endpoint: %w", err)
	}
	return repo.Query(queryString)
}

func (db *TimescaleDatabase) AddTriples(ctx context.Context, ds TripleDataset) error {
	ctx, cancel := context.WithTimeout(ctx, config.DataWriteTimeout)
	defer cancel()

	log := logging.FromContext(ctx)
	if err := checkTripleDataset(ds); err != nil {
		return fmt.Errorf("Cannot handle invalid dataset: %w", err)
	}

	var num int64 = 0

	err := db.RunAsTransaction(ctx, func(txn pgx.Tx) error {
		_, err := txn.Exec(ctx, "CREATE TEMP TABLE triplet(source TEXT, origin TEXT, time TIMESTAMPTZ, s TEXT, p TEXT, o TEXT)")
		if err != nil {
			return fmt.Errorf("Cannot insert triples for source %s: %w", ds.GetSource(), err)
		}

		num, err = txn.CopyFrom(ctx, pgx.Identifier{"triplet"}, []string{"source", "origin", "time", "s", "p", "o"}, ds)
		if err != nil {
			return fmt.Errorf("Cannot insert triples for source %s: %w", ds.GetSource(), err)
		}

		_, err = txn.Exec(ctx, "INSERT INTO triples SELECT * FROM triplet ON CONFLICT (source, origin, time, s, p, o) DO NOTHING")
		if err != nil {
			return fmt.Errorf("Cannot insert triples for source %s: %w", ds.GetSource(), err)
		}

		_, err = txn.Exec(ctx, "DROP TABLE triplet")
		if err != nil {
			return fmt.Errorf("Cannot insert triples for source %s: %w", ds.GetSource(), err)
		}

		return nil
	})
	if err == nil {
		log.Infof("Inserted %5d triples", num)
	}
	return err
}

func (db *TimescaleDatabase) graphs(ctx context.Context) ([]string, error) {
	// get graph names
	var graphs []string
	rows, err := db.pool.Query(ctx, `SELECT distinct source from triples`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var g string
		if err := rows.Scan(&g); err != nil {
			return nil, err
		}
		graphs = append(graphs, g)
	}
	return graphs, nil
}

func (db *TimescaleDatabase) Qualify(ctx context.Context, qualifyQueryList []string) (map[string][]int, error) {
	log := logging.FromContext(ctx)

	var querySiteCounts = make(map[string][]int)

	graphs, err := db.graphs(ctx)
	if err != nil {
		return querySiteCounts, err
	}

	for queryIdx, queryString := range qualifyQueryList {
		for _, graph := range graphs {
			res, err := db.QuerySparql(ctx, graph, queryString)
			if err != nil {
				log.Errorf("Could not evaluate query %s: %w", queryString, err)
				return querySiteCounts, err
			}
			log.Infof("Graph %s, Query %d, # results %d", graph, queryIdx, len(res.Solutions()))

			if _, ok := querySiteCounts[graph]; !ok {
				querySiteCounts[graph] = make([]int, len(qualifyQueryList))
			}
			querySiteCounts[graph][queryIdx] = len(res.Solutions())

		}
	}
	log.Infof("Qualify result: %+v", querySiteCounts)
	return querySiteCounts, nil
}
