"""
Header fields:
- time
- id (brick uri) / name
- value
- type (brick type)
- site (source)

"""

import io
import csv
import requests
from requests.utils import quote
import sys

if len(sys.argv) != 2:
    print("Usage: python load_csv.py <path to csv file>")
    sys.exit(1)


def register(source, name, uri, btype, units):
    d = {
        'SourceName': source,
        'Name': name,
        'Units': units,
        'BrickURI': uri,
        'BrickClass': btype
    }
    resp = requests.post("http://localhost:5001/register_stream", json=d)
    if not resp.ok:
        print(resp.content)


# TODO: need to split the files by source!!
with open(sys.argv[1], 'r') as f:
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

        url = f'http://localhost:5001/insert/csv?source={source}&\
name={name}&brick_uri={uri}&units={units}&brick_class={btype}&apikey=f7851e93-5717-4921-a978-26c5c550e0a5'

        print(url)
        b = io.BytesIO(buf.getvalue().encode('utf8'))
        resp = requests.post(url, data=b, headers={'Content-Type': 'text/csv'})
        if not resp.ok:
            print(resp.content)
