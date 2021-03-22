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
      timescaledb.compress_orderby = 'time DESC',
      timescaledb.compress_segmentby = 'stream_id');
-- Timescale 1.x
-- SELECT add_compress_chunks_policy('data', INTERVAL '14 days');

-- Timescale 2.x
SELECT add_compression_policy('data', INTERVAL '14 days');

CREATE VIEW unified AS
    SELECT time, value, stream_id, name, source, units, brick_uri, brick_class
    FROM data LEFT JOIN streams ON data.stream_id = streams.id;


-- https://docs.timescale.com/latest/using-timescaledb/continuous-aggregates
-- use MATERIALIZED for Timescale 2.x
CREATE MATERIALIZED VIEW hourly_summaries
 WITH (timescaledb.continuous) AS
 SELECT stream_id,
        time_bucket(INTERVAL '1 hour', time) AS bucket,
        COUNT(value) as count,
        MAX(value) as max,
        MIN(value) as min,
        AVG(value) as mean
 FROM data
 GROUP BY stream_id, bucket;
-- timescale 1.x
-- ALTER VIEW hourly_summaries SET (timescaledb.refresh_interval = '30 min');

-- timescale 2.x
SELECT add_continuous_aggregate_policy('hourly_summaries',
    start_offset => NULL,
    end_offset => INTERVAL '1 h',
    schedule_interval => INTERVAL '1 h');


-- handle creation of triplestore
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


-- for notification when triples changes
-- from https://citizen428.net/blog/asynchronous-notifications-in-postgres/access
CREATE OR REPLACE FUNCTION notify_event() RETURNS TRIGGER AS $$
  DECLARE
    record RECORD;
    payload JSON;
  BEGIN
    IF (TG_OP = 'DELETE') THEN
      record = OLD;
    ELSE
      record = NEW;
    END IF;

    payload = json_build_object('table', TG_TABLE_NAME,
                                'action', TG_OP,
                                'data', row_to_json(record));

    PERFORM pg_notify('events', payload::text);

    RETURN NULL;
  END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER notify_triple_change
AFTER INSERT OR UPDATE OR DELETE ON triples
  FOR EACH ROW EXECUTE PROCEDURE notify_event();


-- authorization stuff
CREATE TABLE apikeys(
    apikey TEXT PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE authorizations(
    apikey TEXT REFERENCES apikeys(apikey),
    source TEXT NOT NULL,
    permission TEXT NOT NULL,
    granted_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE OR REPLACE FUNCTION new_apikey() RETURNS TEXT AS $$
  DECLARE
    apikey UUID;
  BEGIN
    apikey = gen_random_uuid();
    INSERT INTO apikeys(apikey) VALUES (apikey);
  RETURN apikey;
  END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION revoke_apikey(to_revoke TEXT) RETURNS VOID AS $$
  BEGIN
    DELETE FROM authorizations WHERE apikey = to_revoke;
    DELETE FROM apikeys WHERE apikey = to_revoke;
  END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION authorize_write(key TEXT, auth_source TEXT) RETURNS VOID AS $$
  BEGIN
    INSERT INTO authorizations(apikey, source, permission) VALUES (key, auth_source, 'write');
  END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION unauthorize_write(to_revoke TEXT) RETURNS VOID AS $$
  BEGIN
    DELETE FROM authorizations WHERE apikey = to_revoke AND permission = 'write';
  END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION authorize_read(key TEXT, auth_source TEXT) RETURNS VOID AS $$
  BEGIN
    INSERT INTO authorizations(apikey, source, permission) VALUES (key, auth_source, 'read');
  END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION unauthorize_read(to_revoke TEXT) RETURNS VOID AS $$
  BEGIN
    DELETE FROM authorizations WHERE apikey = to_revoke AND permission = 'read';
  END;
$$ LANGUAGE plpgsql;
