package server

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/gtfierro/mortar2/internal/config"
	"github.com/gtfierro/mortar2/internal/database"
	"github.com/gtfierro/mortar2/internal/logging"
	"github.com/knakk/rdf"
)

// Server implements the frontend HTTP interface
type Server struct {
	ctx         context.Context
	db          database.Database
	httpAddress string
}

// NewWithInsecureDefaults creates a new Server with the (insecure) default settings; helpful for debugging, but NEVER run in production!
func NewWithInsecureDefaults(ctx context.Context) (*Server, error) {

	cfg := &config.Config{
		HTTP: config.HTTP{
			ListenAddress: "localhost",
			Port:          "5001",
		},
		Database: config.Database{
			Host:     "localhost",
			Database: "mortar",
			User:     "mortarchangeme",
			Password: "mortarpasswordchangeme",
			Port:     "5434",
		},
	}

	return NewFromConfig(ctx, cfg)
}

// NewFromConfig creates a new server with the given configuration
func NewFromConfig(ctx context.Context, cfg *config.Config) (*Server, error) {
	httpAddress := fmt.Sprintf("%s:%s", cfg.HTTP.ListenAddress, cfg.HTTP.Port)

	db, err := database.NewTimescaleFromConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("Could not connect to database: %w", err)
	}

	srv := &Server{
		ctx:         ctx,
		httpAddress: httpAddress,
		db:          db,
	}

	return srv, nil
}

// Shutdown disconnects from the database and returns
func (srv *Server) Shutdown() error {
	log := logging.FromContext(srv.ctx)
	log.Info("Shutting down server")

	srv.db.Close()
	return nil
}

// ServeHTTP starts the HTTP server listener. Blocks.
func (srv *Server) ServeHTTP() error {
	log := logging.FromContext(srv.ctx)
	mux := http.NewServeMux()
	mux.HandleFunc("/register_stream", requireAuth(addLogger(srv.registerStream)))
	mux.HandleFunc("/insert/data", requireAuth(addLogger(srv.insertJSONData)))
	mux.HandleFunc("/insert/csv", requireAuth(addLogger(srv.insertCSVFile)))
	mux.HandleFunc("/insert/metadata", requireAuth(addLogger(srv.insertTriplesFromFile)))
	mux.HandleFunc("/query", addLogger(srv.readDataChunk))
	mux.HandleFunc("/query/model", requireAuth(addLogger(srv.readModel)))
	mux.HandleFunc("/sparql", addLogger(srv.serveSPARQLQuery))
	mux.HandleFunc("/qualify", addLogger(srv.handleQualify))
	// TODO: data stream statistics (per source, per type, etc)

	server := &http.Server{
		Addr:    srv.httpAddress,
		Handler: mux,
		// https://blog.cloudflare.com/exposing-go-on-the-internet/
		//ReadTimeout:  5 * time.Second,
		//WriteTimeout: 10 * time.Second,
		//IdleTimeout:  120 * time.Second,
	}

	log.Infof("Serving HTTP on %s", srv.httpAddress)
	return server.ListenAndServe()
}

// Done kills the server
func (srv *Server) Done() <-chan struct{} {
	return srv.ctx.Done()
}

func (srv *Server) registerStream(w http.ResponseWriter, r *http.Request) {
	log := logging.FromContext(srv.ctx)

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	var stream database.Stream
	if err := json.NewDecoder(r.Body).Decode(&stream); err != nil {
		log.Errorf("Could not parse stream %s", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := srv.db.RegisterStream(ctx, stream); err != nil {
		log.Errorf("Could not register stream %s", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (srv *Server) insertJSONData(w http.ResponseWriter, r *http.Request) {
	log := logging.FromContext(srv.ctx)

	ctx, cancel := context.WithTimeout(r.Context(), config.DataWriteTimeout)
	defer cancel()
	defer r.Body.Close()

	var ds = database.NewArrayDataset()
	if err := json.NewDecoder(r.Body).Decode(ds); err != nil {
		log.Errorf("Could not parse dataset %s", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	//// TODO: register stream if necessary
	//stream := database.GetStream(&ds)
	//if err := stream.FromURLParams(r.URL.Query()); err != nil {
	//	rerr := fmt.Errorf("Could not read source from params: %w", err)
	//	log.Error(rerr)
	//	http.Error(w, rerr.Error(), http.StatusBadRequest)
	//	return
	//}
	//if err := srv.db.RegisterStream(ctx, stream); err != nil {
	//	log.Errorf("Could not register stream %s", err)
	//	http.Error(w, err.Error(), http.StatusInternalServerError)
	//	return
	//}

	// insert data
	if err := srv.db.InsertHistoricalData(ctx, ds); err != nil {
		log.Errorf("Could not insert data %s", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (srv *Server) insertCSVFile(w http.ResponseWriter, r *http.Request) {
	log := logging.FromContext(srv.ctx)
	ctx, cancel := context.WithTimeout(r.Context(), config.DataWriteTimeout)
	defer cancel()
	defer r.Body.Close()

	var (
		rdg    database.Reading
		stream database.Stream
		rowNum = 0
	)

	if err := stream.FromURLParams(r.URL.Query()); err != nil {
		rerr := fmt.Errorf("Could not read source from params: %w", err)
		log.Error(rerr)
		http.Error(w, rerr.Error(), http.StatusBadRequest)
		return
	}
	//log.Infof("%+v\n", stream)

	if err := srv.db.RegisterStream(ctx, stream); err != nil {
		log.Errorf("Could not register stream %s", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// try out csv decoder
	csvr := csv.NewReader(r.Body)
	readings := make(chan database.Reading)
	errc := make(chan error)
	ds := database.NewStreamingDataset(stream.SourceName, stream.Name, readings)

	go func() {
		for {
			row, err := csvr.Read()
			if err == io.EOF {
				//log.Info("End of file")
				close(readings)
				break
			} else if err != nil {
				log.Errorf("Got error reading CSV file: %s", err)
				cancel()
				errc <- fmt.Errorf("Error reading CSV file: %w", err)
				return
			}

			if err := rdg.FromCSVRow(row); err != nil {
				log.Errorf("Bad row %d in CSV file: %w", rowNum, err)
				cancel()
				errc <- fmt.Errorf("Bad row %d in CSV file: %w", rowNum, err)
				return
			}
			ds.GetReadings() <- rdg
			rowNum++
		}
		errc <- nil
	}()

	if err := srv.db.InsertHistoricalData(ctx, ds); err != nil {
		log.Errorf("Could not insert data %s", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	err := <-errc
	if err != nil {
		log.Errorf("Problem inserting CSV file: %s", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// TODO: get metadat as well: units, SPARQL query results, etc
func (srv *Server) readDataChunk(w http.ResponseWriter, r *http.Request) {
	log := logging.FromContext(srv.ctx)
	ctx, cancel := context.WithTimeout(r.Context(), config.DataReadTimeout)
	defer cancel()
	defer r.Body.Close()

	start := time.Now()

	var query database.Query
	if err := query.FromURLParams(r.URL.Query()); err != nil {
		rerr := fmt.Errorf("Could not read source from params: %w", err)
		log.Error(rerr)
		http.Error(w, rerr.Error(), http.StatusBadRequest)
		return
	}

	// TODO: is there a (standard) way to communicate errors over the Arrow IPC
	// mechanism rather than falling back to an HTTP status code?
	log.Infof("Read data chunk %+v", query)
	err := srv.db.ReadDataChunk(ctx, w, &query)
	if err != nil {
		log.Errorf("Problem querying data: %s", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	fmt.Println("Query took", time.Since(start))
}

func (srv *Server) insertTriplesFromFile(w http.ResponseWriter, r *http.Request) {
	log := logging.FromContext(srv.ctx)
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	defer r.Body.Close()

	var tripSrc database.TripleSource
	if err := tripSrc.FromURLParams(r.URL.Query()); err != nil {
		rerr := fmt.Errorf("Could not read source from params: %w", err)
		log.Error(rerr)
		http.Error(w, rerr.Error(), http.StatusBadRequest)
		return
	}

	dec := rdf.NewTripleDecoder(r.Body, tripSrc.Format)
	ds := database.NewStreamingTripleDataset(tripSrc.Source, tripSrc.Origin, time.Now(), dec)

	err := srv.db.AddTriples(ctx, ds)
	if err != nil {
		log.Errorf("Problem inserting triples: %s", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (srv *Server) serveSPARQLQuery(w http.ResponseWriter, r *http.Request) {
	log := logging.FromContext(srv.ctx)
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	defer r.Body.Close()

	var (
		sparqlQuery []byte
		err         error
	)
	w.Header().Add("Content-Type", "application/sparql-results+json")

	// check query parameters
	// get first 'site'
	q := r.URL.Query()
	log.Infof("Query: %+v", q)
	site := q.Get("site")
	if queryString := q.Get("query"); len(queryString) > 0 {
		sparqlQuery = []byte(queryString)
	} else if sparqlQuery, err = ioutil.ReadAll(r.Body); err != nil {
		rerr := fmt.Errorf("Bad SPARQL query: %w", err)
		log.Error(rerr)
		http.Error(w, rerr.Error(), http.StatusBadRequest)
		return
	} else if len(sparqlQuery) == 0 {
		rerr := fmt.Errorf("Bad SPARQL query: %w", err)
		log.Error(rerr)
		http.Error(w, rerr.Error(), http.StatusBadRequest)
		return
	}

	log.Infof("Query SPARQL: %s %s", site, string(sparqlQuery))
	if err := srv.db.QuerySparqlWriter(ctx, w, site, string(sparqlQuery)); err != nil {
		rerr := fmt.Errorf("Bad SPARQL query: %w", err)
		log.Error(rerr)
		http.Error(w, rerr.Error(), http.StatusInternalServerError)
		return
	}
}

func (srv *Server) handleQualify(w http.ResponseWriter, r *http.Request) {
	// TODO: JSON-encoded list of sparql queries
	log := logging.FromContext(srv.ctx)

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	defer r.Body.Close()

	var qualifyQueryList []string
	if err := json.NewDecoder(r.Body).Decode(&qualifyQueryList); err != nil {
		log.Errorf("Could not parse list of queries %s", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	results, err := srv.db.Qualify(ctx, qualifyQueryList)
	if err != nil {
		rerr := fmt.Errorf("Could not qualify: %w", err)
		log.Error(rerr)
		http.Error(w, rerr.Error(), http.StatusInternalServerError)
		return
	}
	enc := json.NewEncoder(w)
	if err := enc.Encode(results); err != nil {
		rerr := fmt.Errorf("Could not serialize qualify results: %w", err)
		log.Error(rerr)
		http.Error(w, rerr.Error(), http.StatusInternalServerError)
		return
	}
}

func (srv *Server) readModel(w http.ResponseWriter, r *http.Request) {
	log := logging.FromContext(srv.ctx)

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	defer r.Body.Close()

	var request database.ModelRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		log.Errorf("Could not read model request %s", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// w.Header().Add("Content-Type", "text/turtle")
	if err := srv.db.GetGraph(ctx, &request, w); err != nil {
		rerr := fmt.Errorf("Could not write graph: %w", err)
		log.Error(rerr)
		http.Error(w, rerr.Error(), http.StatusInternalServerError)
		return
	}
}
