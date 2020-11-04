Mortar Data Model
=================


This document presents an overview of the Mortar data model: how it combine Brick metadata with timeseries data, and how these two bodies of information are presented and managed.

```{image} /img/model-with-data.png
:alt: Brick model with links to data
:align: center
```

## Brick Schema

The Brick schema, also referred to as the Brick ontology, is a formal data model for describing data sources and their context in a building. Refer to the [Brick Ontology documentation page](https://brickschema.org/ontology) for details on the contents of the Brick ontology.

## Brick Model

A Brick model is a graph-based representation of a particular building and its subsystems, processes and data sources. The terms, relationships, classes and  other contents of a Brick model are drawn from the Brick schema.

## Stream

Timeseries data is organized into *streams*; a stream is a sequence of timeseries data from a single source (e.g. a sensor channel). Streams are grouped by a `SourceName`, and are uniquely identified within that group by a `name`. This `name` will be the link between the timeseries data and the metadata which is captured in a Brick model. The Brick model will provide additional context such as units, type, location and related equipment and locations.


:::{info}
Some vocabulary:
- **Site**: a single facility; any building with its own street address
- **Timeseries**: a sequence of values with associated times, e.g. the readings of a temperature sensor
- **Brick schema**: the class hierarchy and relationships defined by Brick
- **Brick model**: a directed graph representing the entities (class instances) and relationships present in a building and its subsystems
:::
