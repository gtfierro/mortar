__version__ = '0.1.0'

import io
import re
import functools
import csv
import os
from datetime import datetime
import requests
from requests.utils import quote
from rdflib.plugins.stores.sparqlstore import SPARQLStore
import pyarrow as pa
import pandas as pd
import logging
from pymortar.mortar_pb2 import QualifyRequest, FetchRequest, View, DataFrame, Timeseries
from pymortar.mortar_pb2 import AGG_FUNC_RAW  as RAW
from pymortar.mortar_pb2 import AGG_FUNC_MEAN as MEAN
from pymortar.mortar_pb2 import AGG_FUNC_MIN as MIN
from pymortar.mortar_pb2 import AGG_FUNC_MAX as MAX
from pymortar.mortar_pb2 import AGG_FUNC_COUNT as COUNT
from pymortar.mortar_pb2 import AGG_FUNC_SUM as SUM

logging.basicConfig(level=logging.DEBUG)

# TODO: allow prefixes to be defined so that the big long URIs don't show up


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
    def __init__(self, endpoint):
        self._endpoint = endpoint.strip('/')
        self._sparql_endpoint = SPARQLStore(f"{self._endpoint}/sparql")

    def load_csv(self, filename):
        logging.info(f"Uploading {filename} to {self._endpoint}/insert_streaming")
        with open(filename, 'r') as f:
            with io.StringIO() as buf:
                w = csv.writer(buf)
                r = csv.DictReader(f)

                registered = False
                for row in r:
                    if not registered:
                        source = quote(row['site'])
                        name = quote(row['label'])
                        uri = quote(row['id'])
                        btype = quote(row['type'])
                        units = 'degF'
                        registered = True
                    w.writerow([row['time'], row['value']])

                url = f'{self._endpoint}/insert_streaming?source={source}&name={name}&brick_uri={uri}&units={units}&brick_class={btype}'

                b = io.BytesIO(buf.getvalue().encode('utf8'))
                resp = requests.post(url, data=b, headers={'Content-Type': 'text/csv'})
                if not resp.ok:
                    raise Exception(resp.content)

    def load_triple_file(self, source, filename):
        logging.info(f"Uploading {filename} to {self._endpoint}/insert_triple_file")
        basename = os.path.basename(filename).strip('.ttl')
        with open(filename, 'rb') as f:
            resp = requests.post(f'{self._endpoint}/insert_triple_file?source={source}&origin={basename}', data=f.read())
            if not resp.ok:
                raise Exception(resp.content)

    def sparql(self, query, sites=None):
        if sites is None:
            res = self._sparql_endpoint.query(query)
            return pd.DataFrame.from_records(list(res), columns=[str(c) for c in res.vars])
        dfs = []
        for site in sites:
            ep = SPARQLStore(f"{self._endpoint}/sparql?site={site}")
            res = ep.query(query)
            df = pd.DataFrame.from_records(list(res), columns=[str(c) for c in res.vars])
            df['site'] = site
            dfs.append(df)
        if len(dfs) == 0:
            return pd.DataFrame()
        return functools.reduce(lambda x, y: pd.concat([x, y], axis=0), dfs)

    # def get_data_ids(self, ids, source=None, start=None, end=None):
    #     resp = requests.get(f'http://localhost:5001/query?sparql={sparql}&start={start}')
    #     r = pa.ipc.open_stream(resp.content)

    def get_data_sparql(self, sparql, source=None, start=None, end=None, agg=None, window=None):
        parts = []
        if start is not None:
            if isinstance(start, datetime):
                parts.append(f"start={start.localize().strftime('%Y-%m-%dT%H:%M:%SZ')}")
            else:
                parts.append(f"start={start}")
        else:
            parts.append("start=1970-01-01T00:00:00Z")

        if source is not None:
            parts.append(f"source={source}")

        metadata = self.sparql(sparql, sites=[source] if source is not None else None)

        query_string = '&'.join(parts)
        if agg is not None and window is not None:
            resp = requests.get(f'{self._endpoint}/query?sparql={sparql}&{query_string}&agg={agg}&window={window}')
        else:
            resp = requests.get(f'{self._endpoint}/query?sparql={sparql}&{query_string}')
        r = pa.ipc.open_stream(resp.content)
        return metadata, r.read_pandas()

    def qualify(self, required_queries):
        """
        Calls the Mortar API Qualify command

        Args:
            required_queries (list of str): list of queries we want to use to filter sites

        Returns:
            sites (list of str): List of site names to be used in a subsequent fetch command
        """
        res = requests.post(f'{self._endpoint}/qualify', json=required_queries)
        return QualifyResult(res.json())

    def fetch(self, query):
        views = {}
        dfs = {}
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
                viewquery = views[ts.view]['definition']
                datavars = [x.strip('?') for x in ts.dataVars]
                viewvars = views[ts.view]['results'].columns
                removevars = set(viewvars).difference(set(datavars))
                for var in removevars:
                    viewquery = viewquery.replace(f'?{var}', '', 1)
                _, newdf = self.get_data_sparql(viewquery, agg=parse_aggfunc(df.aggregation), window=df.window)
                newdfs.append(newdf)
            dfs[df.name] = functools.reduce(lambda x, y: pd.concat([x, y], axis=0), newdfs)
        return views, dfs


class QualifyResult:
    def __init__(self, response):
        self.resp = response
        num_queries = len(list(self.resp.values())[0])
        columns = [f"Query_{i}" for i in range(num_queries)]
        self._df = pd.DataFrame(self.resp.values(), columns=columns, index=self.resp.keys()).__repr__()

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
