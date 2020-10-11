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
