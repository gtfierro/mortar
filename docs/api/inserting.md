---
jupytext:
  formats: md:myst
  text_representation:
    extension: .md
    format_name: myst
kernelspec:
  display_name: Python 3
  language: python
  name: python3
---

Inserting Data
==============

There are two primary ways to insert data into Mortar: POSTing JSON-encoded blobs and streaming/uploading CSV files.

:::{warning}
Mortar (v2) described in this document does not yet support securing the ability to insert/query data streams. Status on this feature is tracked [here](https://github.com/gtfierro/mortar/issues/)
:::

## Inserting with HTTP POST

POSTing data to Mortar is recommended for small or incremental updates, on the order of 1000 readings or less

**TODO: check; it might have only loaded in 4999 records?**

```{code-cell}
import requests
from datetime import datetime, timedelta
import random

def generate_n(n):
    st = datetime.now()
    return [
        [(st + timedelta(seconds=x)).strftime("%Y-%m-%dT%H:%M:%SZ%Z"),
         random.randint(0, 100)]
        for x in range(n)
    ]
 
stream1 = {
    "SourceName": "testsource1",
    "Units": "degF",
    "Name": "stream1"
}

resp = requests.post("http://localhost:5001/register_stream", json=stream1)
if not resp.ok:
    print(resp.content)

readings = generate_n(5000)
ds = {
    "SourceName": "testsource1",
    "Name": "stream1",
    "Readings": readings
}

resp = requests.post('http://localhost:5001/insert_bulk', json=ds)
if not resp.ok:
    print(resp.content)
```
