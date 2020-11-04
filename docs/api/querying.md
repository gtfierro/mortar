Querying Data
=============

Mortar offers 2 different APIs for retrieving metadata and timeseries data.

## HTTP API

Mortar implements a basic HTTP API for querying timeseries data. Clients execute a HTTP GET on the `/query` endpoint with the following URL parameters:
- `start`: the lower bound on the temporal range of data that is returned by the server. Specified as an RFC3339 timestamp; defaults to Jan 1 1970.
- `end`: the upper bound on the temporal range of data that is returned by the server. Specified as an RFC3339 timestamp; defaults to the current time.
- `source`: the list of sources whose data we want. Specifying a `source` will return all streams registered with that `source`. More than one source can be specified (just include another `source` key in the URL params)
- `sparql`: executes a SPARQL query and returns data for all streams that are included in the query results
