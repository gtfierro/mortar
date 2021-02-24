FROM postgres:12

ARG MORTAR_DB_USER=mortarchangeme
ARG MORTAR_DB_PASSWORD=mortarpasswordchangeme

# Silence debconf TERM messages
RUN echo "debconf debconf/frontend select Noninteractive" | debconf-set-selections
RUN apt-get update && apt-get install -y \
      postgresql-plpython3-12 \
      python3-dev \
      python3-pip \
      # Utilities
      software-properties-common \
      apt-transport-https \
      ca-certificates \
      gnupg \
      wget

# Setup postgresql data dir and config
RUN mkdir /data/
RUN cat /usr/share/postgresql/postgresql.conf.sample > /data/postgresql.conf

# PostgreSQL ENV variables
ENV POSTGRES_USER=$MORTAR_DB_USER
ENV POSTGRES_PASSWORD=$MORTAR_DB_PASSWORD
ENV POSTGRES_HOST_AUTH_METHOD=password
ENV POSTGRES_DB=mortar
ENV PGDATA=/data/

# Add timescale
RUN sh -c "echo 'deb https://packagecloud.io/timescale/timescaledb/debian/ `lsb_release -c -s` main' > /etc/apt/sources.list.d/timescaledb.list"
RUN wget --quiet -O - https://packagecloud.io/timescale/timescaledb/gpgkey | apt-key add -
RUN apt-get update && apt-get install -y timescaledb-2-postgresql-12
RUN timescaledb-tune --quiet --yes --conf-path=/data/postgresql.conf

# Add initialization scripts
ADD install_timescale.sh /docker-entrypoint-initdb.d/001_install.sh
ADD setup.sql /docker-entrypoint-initdb.d/002_setup.sql
# from https://github.com/timescale/timescaledb-extras/blob/master/backfill.sql
ADD backfill.sql /docker-entrypoint-initdb.d/003_backfill.sql

RUN pg_lsclusters


VOLUME /data
EXPOSE 5432
