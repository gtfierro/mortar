Mortar Data Model
=================


This document presents an overview of the Mortar data model: how it combine Brick metadata with timeseries data, and how these two bodies of information are presented and managed.

```{image} /img/model-with-data.png
:alt: Brick model with links to data
:align: center
```

:::{info}
Some vocabulary:
- **Site**: a single facility; any building with its own street address
- **Timeseries**: a sequence of values with associated times, e.g. the readings of a temperature sensor
- **Brick schema**: the class hierarchy and relationships defined by Brick
- **Brick model**: a directed graph representing the entities (class instances) and relationships present in a building and its subsystems
:::
