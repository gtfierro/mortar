__version__ = "2.0.0"

import io
import re
import functools
import csv
import os
import urllib.parse

# import snappy
# import sqlite3
from datetime import datetime
import requests
from requests.utils import quote
from rdflib.plugins.stores.sparqlstore import SPARQLStore
import rdflib
import pyarrow as pa
import pandas as pd
from brickschema.namespaces import BRICK, RDF, TAG
import logging
from pymortar.mortar_pb2 import (
    QualifyRequest,
    FetchRequest,
    View,
    DataFrame,
    Timeseries,
)
from pymortar.mortar_pb2 import AGG_FUNC_RAW as RAW
from pymortar.mortar_pb2 import AGG_FUNC_MEAN as MEAN
from pymortar.mortar_pb2 import AGG_FUNC_MIN as MIN
from pymortar.mortar_pb2 import AGG_FUNC_MAX as MAX
from pymortar.mortar_pb2 import AGG_FUNC_COUNT as COUNT
from pymortar.mortar_pb2 import AGG_FUNC_SUM as SUM

logging.basicConfig(level=logging.INFO)

# TODO: allow prefixes to be defined so that the big long URIs don't show up

# _mempool = pa.default_memory_pool()


def parse_aggfunc(aggfunc):
    if aggfunc == MAX:
        return "max"
    elif aggfunc == MIN:
        return "min"
    elif aggfunc == COUNT:
        return "count"
    elif aggfunc == SUM:
        return "sum"
    elif aggfunc == MEAN:
        return "mean"


class Client:
    def __init__(self, endpoint, apikey=None):
        self._endpoint = endpoint.strip("/")
        self._sparql_endpoint = SPARQLStore(f"{self._endpoint}/sparql")
        self._apikey = apikey
        # self._cache = sqlite3.connect(".mortar_cache.db")
        # cur = self._cache.cursor()
        # cur.execute('''CREATE TABLE IF NOT EXISTS downloaded(time TIMESTAMP, query STRING, data BLOB)''')

    def load_csv(self, filename):
        logging.info(f"Uploading {filename} to {self._endpoint}/insert_streaming")
        with open(filename, "r") as f:
            with io.StringIO() as buf:
                w = csv.writer(buf)
                r = csv.DictReader(f)

                registered = False
                for row in r:
                    if not registered:
                        source = quote(row["site"])
                        name = quote(row["label"])
                        uri = quote(row["id"])
                        btype = quote(row.get("type", BRICK.Point))
                        units = quote(row.get("units", "unknown"))
                        registered = True
                    w.writerow([row["time"], row["value"]])

                if self._apikey:
                    url = f"{self._endpoint}/insert/csv?source={source}&name={name}&brick_uri={uri}&units={units}&brick_class={btype}&apikey={self._apikey}"
                else:
                    url = f"{self._endpoint}/insert/csv?source={source}&name={name}&brick_uri={uri}&units={units}&brick_class={btype}"

                b = io.BytesIO(buf.getvalue().encode("utf8"))
                resp = requests.post(url, data=b, headers={"Content-Type": "text/csv"})
                if not resp.ok:
                    raise Exception(resp.content)

    def new_stream(self, sourcename, name, units, brick_uri=None, brick_class=None):
        """
        Idempotently registers a new stream and returns a reference to that stream
        """
        d = {
            "SourceName": sourcename,
            "Name": name,
            "Units": units,
        }
        if brick_uri is not None:
            d["BrickURI"] = brick_uri
        if brick_class is not None:
            d["BrickClass"] = brick_class
        logging.info(f"Registering new stream {d} to {self._endpoint}/register_stream")
        if self._apikey:
            r = requests.post(
                f"{self._endpoint}/register_stream?apikey={self._apikey}", json=d
            )
        else:
            r = requests.post(f"{self._endpoint}/register_stream", json=d)
        if not r.ok:
            raise Exception(r.content)
        return Stream(self, d)

    def add_data(self, sourcename, name, readings):
        """
        Adds data to the stream with the given name

        Args:
            sourcename (str): name of the "group" for this name
            name (str): name of the st ream
            readings (list): each entry is a (RFC 3339 timestamp, float value) tuple
        """
        logging.info(
            f"Uploading {len(readings)} readings to {self._endpoint}/insert/data"
        )
        d = {
            "SourceName": sourcename,
            "Name": name,
            "Readings": readings,
        }
        if self._apikey:
            resp = requests.post(
                f"{self._endpoint}/insert/data?apikey={self._apikey}", json=d
            )
        else:
            resp = requests.post(f"{self._endpoint}/insert/data", json=d)
        if not resp.ok:
            raise Exception(resp.content)

    def load_triple_file(self, source, filename):
        logging.info(f"Uploading {filename} to {self._endpoint}/insert/metadata")
        basename = os.path.basename(filename)
        _, fformat = os.path.splitext(basename)
        with open(filename, "rb") as f:
            if self._apikey:
                resp = requests.post(
                    f"{self._endpoint}/insert/metadata?source={source}&origin={basename}&format={fformat}&apikey={self._apikey}",
                    data=f.read(),
                )
            else:
                resp = requests.post(
                    f"{self._endpoint}/insert/metadata?source={source}&origin={basename}&format={fformat}",
                    data=f.read(),
                )

            if not resp.ok:
                raise Exception(resp.content)

    # def load_graph(self, source, graph):
    #     """
    #     Args:
    #         graph (rdflib.Graph): graph of triples to insert
    #     """
    #     logging.info(f"Uploading {filename} to {self._endpoint}/insert/metadata")
    #     basename = os.path.basename(filename)
    #     _, fformat = os.path.splitext(basename)
    #     with open(filename, "rb") as f:
    #         resp = requests.post(
    #             f"{self._endpoint}/insert/metadata?source={source}&origin={basename}&format={format}",
    #             data=f.read(),
    #         )
    #         if not resp.ok:
    #             raise Exception(resp.content)

    def sparql(self, query, sites=None):
        if sites is None:
            res = self._sparql_endpoint.query(query)
            return pd.DataFrame.from_records(
                list(res), columns=[str(c) for c in res.vars]
            )
        dfs = []
        for site in sites:
            ep = SPARQLStore(f"{self._endpoint}/sparql?site={site}")
            res = ep.query(query)
            df = pd.DataFrame.from_records(
                list(res), columns=[str(c) for c in res.vars]
            )
            df["site"] = site
            dfs.append(df)
        if len(dfs) == 0:
            return pd.DataFrame()
        return functools.reduce(lambda x, y: pd.concat([x, y], axis=0), dfs)

    # def get_data_ids(self, ids, source=None, start=None, end=None):
    #     resp = requests.get(f'http://localhost:5001/query?sparql={sparql}&start={start}')
    #     r = pa.ipc.open_stream(resp.content)

    def data_uris(self, uris, start=None, end=None, agg=None, window=None):
        parts = []
        if start is not None:
            if isinstance(start, datetime):
                parts.append(f"start={start.localize().strftime('%Y-%m-%dT%H:%M:%SZ')}")
            else:
                parts.append(f"start={start}")
        else:
            parts.append("start=1970-01-01T00:00:00Z")

        for uri in uris:
            uri = urllib.parse.quote_plus(uri)
            parts.append(f"uri={uri}")

        query_string = "&".join(parts)
        if agg is not None and window is not None:
            resp = requests.get(
                f"{self._endpoint}/query?{query_string}&agg={agg}&window={window}"
            )
        else:
            resp = requests.get(f"{self._endpoint}/query?{query_string}")

        if not resp.ok:
            logging.error("Error getting data %s" % resp.content)
            raise Exception(resp.content)

        buf = pa.decompress(resp.content, decompressed_size=4e10, codec='lz4', asbytes=True)
        buf = io.BytesIO(buf)
        # read metadata first
        try:
            r = pa.ipc.open_stream(buf)
        except pa.ArrowInvalid as e:
            logging.error("Error deserializing metadata %s" % e)
            raise Exception(e)
        md = r.read_pandas()

        # then read data
        try:
            r = pa.ipc.open_stream(buf)
        except pa.ArrowInvalid as e:
            logging.error("Error deserializing data %s" % e)
            raise Exception(e)
        df = r.read_pandas()
        return Dataset(None, md, df)

    def data_sparql(
        self, sparql, source=None, start=None, end=None, agg=None, window=None
    ):
        params = {"sparql": sparql}
        if agg is not None and window is not None:
            params["agg"] = agg
            params["window"] = window
        if start is not None:
            if isinstance(start, datetime):
                params["start"] = start.localize().strftime("%Y-%m-%dT%H:%M:%SZ")
            else:
                params["start"] = start
        else:
            params["start"] = "1970-01-01T00:00:00Z"

        if end is not None:
            if isinstance(end, datetime):
                params["end"] = end.localize().strftime("%Y-%m-%dT%H:%M:%SZ")
            else:
                params["end"] = end
        else:
            params["end"] = "2100-01-01T00:00:00Z"

        if source is not None:
            params["source"] = source

        metadata = self.sparql(sparql, sites=[source] if source is not None else None)

        resp = requests.get(f"{self._endpoint}/query", params=params)
        if not resp.ok:
            logging.error("Error getting data %s" % resp.content)
            raise Exception(resp.content)
        # print(len(resp.content))

        buf = pa.decompress(resp.content, decompressed_size=4e10, codec='lz4', asbytes=True)
        buf = io.BytesIO(buf)
        # # before: no compression
        # buf = io.BytesIO(resp.content)
        # read metadata first
        try:
            rdr = pa.ipc.open_stream(buf)
        except pa.ArrowInvalid as e:
            logging.error("Error deserializing metadata %s" % e)
            raise Exception(e)
        md = rdr.read_pandas()

        # then read data
        try:
            rdr = pa.ipc.open_stream(buf)
        except pa.ArrowInvalid as e:
            logging.error("Error deserializing data %s" % e)
            raise Exception(e)
        df = rdr.read_pandas()
        return Dataset(metadata, md, df)

    def qualify(self, required_queries):
        """
        Calls the Mortar API Qualify command

        Args:
            required_queries (list of str): list of queries we want to use to filter sites

        Returns:
            sites (list of str): List of site names to be used in a subsequent fetch command
        """
        if isinstance(required_queries, dict):
            names = list(required_queries.keys())
            required_queries = [required_queries[q] for q in names]
        elif isinstance(required_queries, list):
            names = None
        else:
            raise TypeError("Argument must be a list of queries")
        res = requests.post(f"{self._endpoint}/qualify", json=required_queries)
        if not res.ok:
            logging.error("Error getting metadata %s" % res.content)
            raise Exception(res.content)
        return QualifyResult(res.json(), names=names)

    def get_graph(self, name, timestamp=None):
        now = datetime.now().strftime('%Y-%m-%dT%H:%M:%SZ')
        req = {
            "graph": name,
            "timestamp": timestamp if timestamp is not None else now,
        }
        res = requests.post(f"{self._endpoint}/query/model?apikey={self._apikey}", json=req)
        # TODO: fix up the parsing so that it can return a graph
        return res.content
        g = rdflib.Graph()
        g.parse(source=io.BytesIO(res.content), format="ttl")
        return g

    def fetch(self, query):
        """
        Calls the Mortar API Fetch command

        Args:
            query (pymortar.FetchRequest): Mortar API fetch struct

        Returns:
            views (dict of name to DataFrame): SPARQL query results from FetchRequest views
            metadata (dict of name to DataFrame): Metadata table describing all data streams
            dataframes (dict of name to DataFrame): Actual timeseries data
        """
        views = {}
        dfs = {}
        metadata = {}
        for view in query.views:
            # view.name
            # view.definition
            views[view.name] = {
                "results": self.sparql(view.definition, sites=query.sites),
                "definition": view.definition,
            }
        for df in query.dataFrames:
            newdfs = []
            for ts in df.timeseries:
                viewquery = views[ts.view]["definition"]
                datavars = [x.strip("?") for x in ts.dataVars]
                viewvars = views[ts.view]["results"].columns
                removevars = set(viewvars).difference(set(datavars))
                for var in removevars:
                    viewquery = viewquery.replace(f"?{var}", "", 1)
                res = self.data_sparql(
                    viewquery, agg=parse_aggfunc(df.aggregation), window=df.window
                )
                newdfs.append(res.data)
            metadata[df.name] = res.streams
            dfs[df.name] = functools.reduce(
                lambda x, y: pd.concat([x, y], axis=0), newdfs
            )
        return views, metadata, dfs


class FetchResult:
    pass


class QualifyResult:
    def __init__(self, response, names):
        self.resp = response
        vals = list(self.resp.values())
        if len(vals) == 0:
            raise Exception("Empty results")
        num_queries = len(vals[0])
        if names is None:
            columns = [f"Query_{i}" for i in range(num_queries)]
        else:
            columns = names
        self._df = pd.DataFrame(
            self.resp.values(), columns=columns, index=self.resp.keys()
        ).__repr__()

    @property
    def sites(self):
        return [site for site, values in self.resp.items() if all(values)]

    @property
    def df(self):
        return self._df

    def __repr__(self):
        if len(self.resp) == 0:
            return "<No qualify results>"
        return str(self._df)


class Dataset:
    def __init__(self, sparqlMetadata, streamMetadata, df):
        self._sparql = sparqlMetadata
        self._stream = streamMetadata
        self._data = df

    @property
    def streams(self):
        return self._stream

    @property
    def metadata(self):
        return self._sparql

    @property
    def data(self):
        return self._data


class Stream:
    def __init__(self, client, defn):
        self._srcname = defn.get("SourceName")
        self._name = defn.get("Name")
        self._units = defn.get("Units")
        self._uri = defn.get("BrickURI")
        self._class = defn.get("BrickClass")
        self.client = client

    @property
    def source(self):
        return self._srcname

    @property
    def name(self):
        return self._name

    @property
    def units(self):
        return self._units

    @property
    def uri(self):
        return self._uri

    @property
    def type(self):
        return self._class

    def add_data(self, readings):
        """
        Uploads new timeseries data to the server

        Args:
            readings (list): each entry is a (RFC 3339 timestamp, float value) tuple
        """
        self.client.add_data(self.source, self.name, readings)

    def get_data(self, start=None, end=None, agg=None, window=None):
        return self.client.data_uris([self.uri], start, end, agg, window)
