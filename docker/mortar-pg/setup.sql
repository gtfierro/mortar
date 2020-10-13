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
