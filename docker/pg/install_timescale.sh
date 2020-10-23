echo "shared_preload_libraries = 'timescaledb'" >> /data/postgresql.conf
pg_ctl restart
psql -U ${POSTGRES_USER} ${POSTGRES_DB} -c "CREATE EXTENSION IF NOT EXISTS timescaledb CASCADE;"
