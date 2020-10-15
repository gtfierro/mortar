package database

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"time"

	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"

	"github.com/apache/arrow/go/arrow"
	"github.com/apache/arrow/go/arrow/array"
	"github.com/apache/arrow/go/arrow/ipc"
	"github.com/apache/arrow/go/arrow/memory"

	"github.com/gtfierro/mortar2/internal/config"
	"github.com/gtfierro/mortar2/internal/logging"
)

// Database defines the interface to the underlying data store
type Database interface {
	Close()
	RunAsTransaction(context.Context, func(txn pgx.Tx) error) error
	RegisterStream(context.Context, Stream) error
	InsertHistoricalData(ctx context.Context, ds Dataset) error
	ReadDataChunk(context.Context, io.Writer, *Query) error
	AddTriples(context.Context, TripleDataset) error
}

// TimescaleDatabase is an implementation of Database for TimescaleDB
type TimescaleDatabase struct {
	pool *pgxpool.Pool
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
		}
		break
	}
	log.Infof("Connected to postgres at %s", cfg.Database.Host)
	return &TimescaleDatabase{
		pool: pool,
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
								 VALUES(DEFAULT, $1, $2, $3, $4, $5) ON CONFLICT DO NOTHING`,
			stream.Name, stream.SourceName, stream.Units, brickURI, brickClass)
		if err != nil {
			return fmt.Errorf("Could not register stream: %w", err)
		}
		registered = res.RowsAffected() > 0

		return nil
	})

	if err == nil && registered {
		log.Infof("Registered Stream %s", stream.String())
	}
	return err
}

func (db *TimescaleDatabase) InsertHistoricalData(ctx context.Context, ds Dataset) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	log := logging.FromContext(ctx)

	if err := checkDataset(ds); err != nil {
		return fmt.Errorf("Cannot handle invalid dataset: %w", err)
	}

	// TODO: does dataset need to be streamed? (probably)
	// TODO: how to insert into historical data --- need to disable compression?
	var num int64 = 0
	err := db.RunAsTransaction(ctx, func(txn pgx.Tx) error {
		// check valid stream
		row := txn.QueryRow(ctx, `SELECT id FROM streams WHERE source=$1 AND name=$2`, ds.GetSource(), ds.GetName())
		var stream_id int
		err := row.Scan(&stream_id)
		if err != nil {
			return fmt.Errorf("No such stream (SourceName: %s, Name: %s): %w", ds.GetSource(), ds.GetName(), err)
		}

		// TODO: use jackx CopyFrom, https://godoc.org/github.com/jackc/pgx#CopyFromSource
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

func (db *TimescaleDatabase) ReadDataChunk(ctx context.Context, w io.Writer, q *Query) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	sch := arrow.NewSchema([]arrow.Field{
		{Name: "time", Type: arrow.FixedWidthTypes.Timestamp_ns, Nullable: false},
		{Name: "value", Type: arrow.PrimitiveTypes.Float64, Nullable: false},
		{Name: "id", Type: arrow.PrimitiveTypes.Int64, Nullable: false},
	}, nil)
	bldr := array.NewRecordBuilder(memory.DefaultAllocator, sch)
	defer bldr.Release()

	r_times := bldr.Field(0).(*array.TimestampBuilder)
	r_values := bldr.Field(1).(*array.Float64Builder)
	r_ids := bldr.Field(2).(*array.Int64Builder)

	rows, err := db.pool.Query(ctx, `SELECT time, value, stream_id FROM data WHERE time>=$1 and time <=$2 and stream_id = ANY($3)`, q.Start.Format(time.RFC3339), q.End.Format(time.RFC3339), q.Ids)
	if err != nil {
		return fmt.Errorf("Could not query %w", err)
	}
	for rows.Next() {
		var (
			t time.Time
			v float64
			i int64
		)
		if err := rows.Scan(&t, &v, &i); err != nil {
			return fmt.Errorf("Could not query %w", err)
		}
		r_times.Append(arrow.Timestamp(t.UnixNano()))
		r_values.Append(v)
		r_ids.Append(i)
	}

	rec := bldr.NewRecord()
	defer rec.Release()

	arrow_w := ipc.NewWriter(w, ipc.WithSchema(rec.Schema()))
	if err := arrow_w.Write(rec); err != nil {
		return fmt.Errorf("Could not write record %w", err)
	}

	return arrow_w.Close()
}

func (db *TimescaleDatabase) AddTriples(ctx context.Context, ds TripleDataset) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
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
