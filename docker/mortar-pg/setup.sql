CREATE TABLE streams(
    id      SERIAL PRIMARY KEY,
    name    TEXT NOT NULL,
    source  TEXT NOT NULL,
    units   TEXT NOT NULL,
    brick_uri   TEXT,
    brick_class TEXT
);
CREATE UNIQUE INDEX ON streams(source, name);


CREATE TABLE data(
    time        TIMESTAMPTZ,
    stream_id   INTEGER REFERENCES streams(id),
    value       FLOAT NOT NULL,
    PRIMARY KEY(time, stream_id)
);
CREATE INDEX ON data (stream_id, time DESC);
SELECT * FROM create_hypertable('data', 'time');

ALTER TABLE  data
  SET (timescaledb.compress,
      -- timescaledb.compress_orderby = 'time DESC',
      timescaledb.compress_segmentby = 'stream_id');
SELECT add_compress_chunks_policy('data', INTERVAL '14 days');

CREATE VIEW unified AS 
    SELECT time, value, stream_id, name, source, units, brick_uri, brick_class
    FROM data LEFT JOIN streams ON data.stream_id = streams.id;


-- https://docs.timescale.com/latest/using-timescaledb/continuous-aggregates
CREATE VIEW hourly_summaries
 WITH (timescaledb.continuous) AS
 SELECT stream_id,
        time_bucket(INTERVAL '1 hour', time) AS bucket,
        COUNT(value) as count,
        MAX(value) as max,
        MIN(value) as min,
        AVG(value) as mean
 FROM data
 GROUP BY stream_id, bucket;
ALTER VIEW hourly_summaries SET (timescaledb.refresh_interval = '30 min');


-- handle creation of triplestore
-- TODO: maybe want 2 levels here: 1 is the source (site), the other is the 'origin' so we can have multiple sources that all change?

CREATE TABLE triples(
    source TEXT NOT NULL,
    origin TEXT NOT NULL,
    time TIMESTAMPTZ NOT NULL,
    s TEXT NOT NULL,
    p TEXT NOT NULL,
    o TEXT NOT NULL
);
CREATE UNIQUE INDEX ON triples(source, origin, time, s, p, o);

CREATE VIEW latest_triples AS
    WITH lts AS (
        SELECT source, origin, MAX(time) as time
        FROM triples
        GROUP BY source, origin
    )
    SELECT triples.source, s, p, o 
    FROM triples
    LEFT JOIN lts USING(source, origin, time);
