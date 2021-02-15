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

Inserting Metadata
==================

## Inserting Brick Metadata

Turtle files can be POSTed to the `/insert/metadata` API endpoint. Each upload must be qualified by:

- `source`, the SourceName for which this metadata is for (this is analogous to the graph name in RDF)
- `origin`: this is a unique name for this SourceName which represents the point of origin of some Brick metadata. Example origins might be `brick` for the Brick ontology, and `building` for the Brick model of a building. The triples from all origins are merged together (the contents of the most recently uploaded file for each origin are included) and the resulting graph is used by Mortar.

Example:

```{code-cell} Python
import requests

#TODO: this code doesn't work in the notebook yet
with open('Brick.ttl', 'rb') as f:
    url = 'http://localhost:5001/insert/metadata?source=test&origin=brick'
    resp = requests.post(url, data=f.read())
    if not resp.ok:
        print(resp.content)
```

**TODO**: insert triples directly
