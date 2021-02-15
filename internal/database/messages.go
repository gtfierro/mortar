package database

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/knakk/rdf"
	"github.com/knakk/sparql"

	"github.com/apache/arrow/go/arrow"
)

// Stream holds metadata associated with a timeseries source
type Stream struct {
	SourceName string
	Units      string
	Name       string
	BrickURI   string
	BrickClass string
	id         int
}

func (s *Stream) String() string {
	return fmt.Sprintf("Stream[SourceName=%s, Name=%s, Units=%s, BrickURI=%v, BrickClass=%v]",
		s.SourceName, s.Name, s.Units, s.BrickURI, s.BrickClass)
}

func (s *Stream) FromURLParams(vals url.Values) error {
	// mandatory parameters
	if source := vals.Get("source"); len(source) > 0 {
		s.SourceName = source
	} else if len(s.SourceName) == 0 {
		return errors.New("Params lacks 'source'")
	}
	if name := vals.Get("name"); len(name) > 0 {
		s.Name = name
	} else if len(s.Name) == 0 {
		return errors.New("Params lacks 'name'")
	}

	// optional (might already be registered)
	if units := vals.Get("units"); len(units) > 0 {
		s.Units = units
	}
	if uri := vals.Get("brick_uri"); len(uri) > 0 {
		s.BrickURI = uri
	}
	if class := vals.Get("brick_class"); len(class) > 0 {
		s.BrickClass = class
	}
	return nil
}

type Reading struct {
	Value float64
	Time  time.Time
}

// UnmarshalJSON unpacks a Reading from a length-2 JSON array
func (rdg *Reading) UnmarshalJSON(data []byte) error {
	var (
		rdgJSON   [2]json.RawMessage
		timestamp string
		value     json.Number
		err       error
	)
	if err := json.Unmarshal(data, &rdgJSON); err != nil {
		return err
	}

	if err := json.Unmarshal(rdgJSON[0], &timestamp); err != nil {
		return errors.New("First item must be a RFC3339 timestamp")
	}
	rdg.Time, err = time.Parse(time.RFC3339, timestamp)
	if err != nil {
		return errors.New("First item must be a RFC3339 timestamp")
	}

	if err := json.Unmarshal(rdgJSON[1], &value); err != nil {
		return errors.New("Second item must be a number")
	}

	rdg.Value, err = value.Float64()
	if err != nil {
		return fmt.Errorf("Second item was not a float: %w", err)
	}

	return err
}

func (rdg *Reading) FromCSVRow(row []string) error {
	var err error

	rdg.Time, err = time.Parse(time.RFC3339, row[0])
	if err != nil {
		return errors.New("First item must be a RFC3339 timestamp")
	}

	rdg.Value, err = strconv.ParseFloat(row[1], 64)
	if err != nil {
		return errors.New("Second item must be a float")
	}

	return nil
}

type Dataset interface {
	GetSource() string
	GetName() string
	GetReadings() chan Reading
	SetId(int)

	// for CopyFromSource
	Next() bool
	Values() ([]interface{}, error)
	Err() error
}

func GetStream(ds Dataset) Stream {
	return Stream{
		SourceName: ds.GetSource(),
		Name:       ds.GetName(),
	}
}

type StreamingDataset struct {
	SourceName string
	Name       string
	id         int
	Readings   chan Reading
	current    *Reading
}

func NewStreamingDataset(source string, name string, c chan Reading) *StreamingDataset {
	ds := &StreamingDataset{
		SourceName: source,
		Name:       name,
		Readings:   c,
		current:    nil,
		id:         -1,
	}

	return ds
}

func (d *StreamingDataset) SetId(id int) {
	d.id = id
}

func (d *StreamingDataset) String() string {
	return fmt.Sprintf("Dataset[SourceName=%s, Name=%s, # Readings=?]", d.SourceName, d.Name)
}

func (d *StreamingDataset) GetSource() string {
	return d.SourceName
}

func (d *StreamingDataset) GetName() string {
	return d.Name
}

func (d *StreamingDataset) GetReadings() chan Reading {
	return d.Readings
}

// implementing https://godoc.org/github.com/jackc/pgx#CopyFromSource
func (d *StreamingDataset) Next() bool {
	rdg, more := <-d.Readings
	if more {
		d.current = &rdg
		return d.current != nil
	}
	return false
}

func (d *StreamingDataset) Values() ([]interface{}, error) {
	if d.id == -1 {
		return nil, errors.New("Need to set ID")
	}
	return []interface{}{d.current.Time, d.id, d.current.Value}, nil
}

func (d *StreamingDataset) Err() error {
	return nil
}

type ArrayDataset struct {
	SourceName string
	Name       string
	id         int
	Readings   []Reading
	idx        int
}

func (d *ArrayDataset) String() string {
	return fmt.Sprintf("Dataset[SourceName=%s, Name=%s, # Readings=%d]", d.SourceName, d.Name, len(d.Readings))
}

func (d *ArrayDataset) GetSource() string {
	return d.SourceName
}

func (d *ArrayDataset) GetName() string {
	return d.Name
}

func (d *ArrayDataset) GetReadings() chan Reading {
	c := make(chan Reading)
	go func() {
		for _, r := range d.Readings {
			c <- r
		}
		close(c)
	}()
	return c
}

func (d *ArrayDataset) SetId(id int) {
	d.id = id
}

func (d *ArrayDataset) Next() bool {
	d.idx++
	return d.idx < len(d.Readings)
}

func (d *ArrayDataset) Values() ([]interface{}, error) {
	if d.id == -1 {
		return nil, errors.New("Need to set ID")
	}
	rdg := d.Readings[d.idx]
	return []interface{}{rdg.Time, d.id, rdg.Value}, nil
}

func (d *ArrayDataset) Err() error {
	return nil
}

type AggregationType uint

const (
	AggregationMean AggregationType = iota + 1
	AggregationMax
	AggregationMin
	AggregationSum
	AggregationCount
)

func ParseAggregationType(s string) (AggregationType, error) {
	switch strings.ToLower(s) {
	case "mean":
		return AggregationMean, nil
	case "max":
		return AggregationMax, nil
	case "min":
		return AggregationMin, nil
	case "sum":
		return AggregationSum, nil
	case "count":
		return AggregationCount, nil
	default:
		return 0, fmt.Errorf("Aggregation type %s unknown", s)
	}
}

func (agg AggregationType) toSQL(field string) string {
	switch agg {
	case AggregationMean:
		return fmt.Sprintf("avg(%s)", field)
	case AggregationMax:
		return fmt.Sprintf("max(%s)", field)
	case AggregationMin:
		return fmt.Sprintf("min(%s)", field)
	case AggregationSum:
		return fmt.Sprintf("sum(%s)", field)
	case AggregationCount:
		return fmt.Sprintf("count(%s)", field)
	}
	panic("Invalid Aggregation Function")
}

type Query struct {
	Ids               []int64
	Sources           []string
	Sparql            string
	Start             time.Time
	End               time.Time
	AggregationFunc   *AggregationType
	AggregationWindow *time.Duration
}

func (q *Query) FromURLParams(vals url.Values) error {
	var (
		err error
	)

	_sparql := vals.Get("sparql")
	if len(_sparql) > 0 {
		q.Sparql, err = url.QueryUnescape(_sparql)
		if err != nil {
			return fmt.Errorf("Invalid query '%s': %w", _sparql, err)
		}
	}
	// TODO: sparql, err := url.QueryUnescape(q.Sparql)

	_ids, ok := vals["id"]
	if !ok && len(q.Sparql) == 0 {
		return errors.New("Query needs ids")
	}
	q.Ids = make([]int64, len(_ids))
	for idx, _id := range _ids {
		q.Ids[idx], err = strconv.ParseInt(_id, 10, 64)
		if err != nil {
			return fmt.Errorf("Invalid id %s: %w", _id, err)
		}
	}

	if _start := vals.Get("start"); len(_start) > 0 {
		q.Start, err = time.Parse(time.RFC3339, _start)
		if err != nil {
			return fmt.Errorf("Invalid start time %s: %w", _start, err)
		}
	} else {
		return errors.New("Query needs a start time in RFC3339")
	}

	if _end := vals.Get("end"); len(_end) > 0 {
		q.End, err = time.Parse(time.RFC3339, _end)
		if err != nil {
			return fmt.Errorf("Invalid end time %s: %w", _end, err)
		}
	} else {
		q.End = time.Now()
	}

	if _aggfunc := vals.Get("agg"); len(_aggfunc) > 0 {
		aggfunc, err := ParseAggregationType(_aggfunc)
		if err != nil {
			return fmt.Errorf("Invalid aggregation function %s: %w", _aggfunc, err)
		}
		q.AggregationFunc = &aggfunc
	}

	if _window := vals.Get("window"); len(_window) > 0 {
		window, err := ParseDuration(_window)
		if err != nil {
			return fmt.Errorf("Invalid window size %s: %w", _window, err)
		}
		q.AggregationWindow = &window
	}

	q.Sources = vals["source"]

	return nil
}

type TripleSource struct {
	Source string
	Origin string
	Format rdf.Format
	Time   time.Time
}

func (ts *TripleSource) FromURLParams(vals url.Values) error {
	var err error
	// mandatory parameters
	if source := vals.Get("source"); len(source) > 0 {
		ts.Source = source
	} else {
		return errors.New("Params lacks 'source'")
	}
	if origin := vals.Get("origin"); len(origin) > 0 {
		ts.Origin = origin
	} else {
		return errors.New("Params lacks 'origin'")
	}

	// optional
	ts.Format = rdf.Turtle
	if _format := vals.Get("format"); len(_format) > 0 {
		switch strings.ToLower(_format) {
		case "ntriples", "n3":
			ts.Format = rdf.NTriples
		case "turtle", "ttl":
			ts.Format = rdf.Turtle
		case "xml", "rdfxml":
			ts.Format = rdf.RDFXML
		}
	}

	if _time := vals.Get("time"); len(_time) > 0 {
		if ts.Time, err = time.Parse(time.RFC3339, _time); err != nil {
			return fmt.Errorf("Invalid timestamp %s: %w", _time, err)
		}
	}

	return nil
}

type TripleDataset interface {
	GetSource() string
	GetOrigin() string
	GetTime() time.Time
	GetTriples() chan rdf.Triple

	// for CopyFromSource
	Next() bool
	Values() ([]interface{}, error)
	Err() error
}

type StreamingTripleDataset struct {
	dec     rdf.TripleDecoder
	source  string
	origin  string
	time    time.Time
	current *rdf.Triple
}

func NewStreamingTripleDataset(source, origin string, t time.Time, dec rdf.TripleDecoder) *StreamingTripleDataset {
	ds := &StreamingTripleDataset{
		source:  source,
		origin:  origin,
		time:    t,
		dec:     dec,
		current: nil,
	}
	return ds
}

func (ds *StreamingTripleDataset) GetSource() string {
	return ds.source
}

func (ds *StreamingTripleDataset) GetOrigin() string {
	return ds.origin
}

func (ds *StreamingTripleDataset) GetTime() time.Time {
	return ds.time
}

func (ds *StreamingTripleDataset) GetTriples() chan rdf.Triple {
	c := make(chan rdf.Triple)
	go func() {
		for {
			triple, err := ds.dec.Decode()
			if err == io.EOF {
				break
			}
			c <- triple
		}
		close(c)
	}()
	return c
}

func (ds *StreamingTripleDataset) Next() bool {
	triple, err := ds.dec.Decode()
	// sometimes the triple is not nil, but all the fields are nil; this probably happens
	// because of a parse error
	if err == io.EOF || triple.Subj == nil || triple.Pred == nil || triple.Obj == nil {
		return false
	}
	ds.current = &triple
	return ds.current != nil
}

func (ds *StreamingTripleDataset) Values() ([]interface{}, error) {
	if ds.current == nil {
		return nil, errors.New("No value")
	}
	return []interface{}{ds.source, ds.origin, ds.time, ds.current.Subj.Serialize(rdf.NTriples),
		ds.current.Pred.Serialize(rdf.NTriples), ds.current.Obj.Serialize(rdf.NTriples)}, nil
}

func (ds *StreamingTripleDataset) Err() error {
	return nil
}

var dur_re = regexp.MustCompile(`(\d+)(\w+)`)

func ParseDuration(expr string) (time.Duration, error) {
	var d time.Duration
	results := dur_re.FindAllStringSubmatch(expr, -1)
	if len(results) == 0 {
		return d, errors.New("Invalid. Must be Number followed by h,m,s,us,ms,ns,d")
	}
	num := results[0][1]
	units := results[0][2]
	i, err := strconv.ParseInt(num, 10, 64)
	if err != nil {
		return d, err
	}
	d = time.Duration(i)
	switch units {
	case "h", "hr", "hour", "hours":
		d *= time.Hour
	case "m", "min", "minute", "minutes":
		d *= time.Minute
	case "s", "sec", "second", "seconds":
		d *= time.Second
	case "us", "usec", "microsecond", "microseconds":
		d *= time.Microsecond
	case "ms", "msec", "millisecond", "milliseconds":
		d *= time.Millisecond
	case "ns", "nsec", "nanosecond", "nanoseconds":
		d *= time.Nanosecond
	case "d", "day", "days":
		d *= 24 * time.Hour
	default:
		err = fmt.Errorf("Invalid unit %v. Must be h,m,s,us,ms,ns,d", units)
	}
	return d, err
}

func makeStringArrowSchema(names []string) *arrow.Schema {
	var fields []arrow.Field
	for _, varname := range names {
		fields = append(fields, arrow.Field{Name: varname, Type: arrow.BinaryTypes.String, Nullable: true})
	}
	return arrow.NewSchema(fields, nil)
}

// build an arrow schema from a sparql query result
func makeArrowSchemaFromSPARQL(res *sparql.Results) *arrow.Schema {
	return makeStringArrowSchema(res.Head.Vars)
}
