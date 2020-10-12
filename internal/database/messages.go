package database

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"time"
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
	} else {
		return errors.New("Params lacks 'source'")
	}
	if name := vals.Get("name"); len(name) > 0 {
		s.Name = name
	} else {
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
