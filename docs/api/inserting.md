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

:::{warning}
**TODO**: add documentation on available units
:::


## Inserting with HTTP POST

POSTing data to Mortar is recommended for small or incremental updates, on the order of 1000 readings or less. A POST will add data to Mortar for a particular timeseries stream. 

### Registering Timeseries Streams


Streams can be pre-registered with Mortar using the `/register_stream` API endpoint. Registration is idempotent.

```{code-cell} Python
import requests

stream1 = {
    "SourceName": "testsource1",
    "Units": "degF",
    "Name": "stream1",
    "BrickURI": "mybuilding#stream1",
    "BrickClass": "https://brickschema.org/ontology/1.1/Brick#Air_Temperature_Sensor"
}

resp = requests.post("http://mortar-server:5001/register_stream", json=stream1)
if not resp.ok:
    print(resp.content)
print("Registered!")
```

Valid keys are:
- `SourceName` (required): a common namespace for a group of related streams
- `Name` (required): a name for this stream that is unique to this `SourceName`
- `BrickURI` (optional): a RDF IRI for this entity, to be used in a related Brick model
- `BrickClass` (optional): the Brick type for this entity
- `Units` (optional): the unit of measure for this stream; this will need to be pulled from the QUDT dictionary
 
### Inserting Data

Timeseries data can be added to Mortar by POSTing JSON to the `/insert_bulk` endpoint. The JSON can contain the following fields:

- `SourceName` (required): a common namespace for a group of related streams
- `Name` (required): a name for this stream that is unique to this `SourceName`
- `Readings` (required): an array of `[timestamp, value]` pairs. Timestamps should be in RFC3339 format (e.g. `2020-12-31T13:14:15Z`). Values can be integers or floats.
- `BrickURI` (optional): a RDF IRI for this entity, to be used in a related Brick model
- `BrickClass` (optional): the Brick type for this entity
- `Units` (optional): the unit of measure for this stream; this will need to be pulled from the QUDT dictionary

If the stream is not registered, the server will attempt to register it. If you are not preregistering the streams, you can include the additional metadata here instead.

```{code-cell} Python
import requests

readings = [
  ("2020-11-03T00:00:00Z", 71.0),
  ("2020-11-03T00:01:00Z", 72),
  ("2020-11-03T00:02:00Z", 73.0),
  ("2020-11-03T00:03:00Z", 72.5),
]
ds = {
    "SourceName": "testsource1",
    "Name": "stream1",
    "Units": "degF",
    "Readings": readings
}

resp = requests.post('http://mortar-server:5001/insert/data', json=ds)
if not resp.ok:
    print(resp.content)
print("Inserted!")
```

## Inserting a CSV File

Mortar supports ingesting CSV files using a streaming mechanism that is efficient and performant for large datasets. Mortar requires that a CSV file only contain metadata for a single stream, and that the CSV file has the columns:

- `time`: an RFC3339-encoded timestamp
- `value`: a float or integer

This file should be POSTed to the `/insert/csv` API endpoint with a `Content-Type` of `text/csv`. This can be done in a streaming manner using a Python script to be provided

The `/insert/csv` API endpoint accepts several URL parameters which provide metadata of the stream being uploaded:

- `source` (required): the SourceName of the stream
- `name` (required): the name of the stream
- `brick_uri` (optional): the URL-encoded IRI of the stream, as a Brick entity
- `brick_class` (optional): the URL-encoded Brick class name as a URI

Providing these parameters will register the stream automatically.
