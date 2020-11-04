Brick Metadata
========================

## Definitions

An **Entity** is a digital representation of any physical, logical or virtual item; the actual "things" in and around a building.  Brick defines how entities can be classified and related to one another. There are several flavors of entities:

- **Physical Entities**: anything that has a physical presence in the world. Examples are:
    - mechanical equipment such as air handling units, variable air volume boxes, luminaires and lighting systems
    - networked devices like electric meters, thermostats, electric vehicle chargers
    - spatial elements like buildings, floors and rooms
- **Virtual Entities**: anything whose representation is based in software. Examples are:
    - sensing and status points which allow software to read the current state of the world (such as the value of a temperature sensor, the speed of a fan or the energy consumption of a space heater)
    - actuation points which allow software to write values (such as temperature setpoints or brightness of a lighting fixture)
    - computed points such as average temperatures, electric meter aggregates
- **Logical Entities**: entities or collections of entities defined by a set of rules. Examples are HVAC zones and Lighting zones. Concepts which help to define Brick also fall into this category such as class names and tags

**Relationships** express how entities, classes, tags and other "things" interact and are associated with each other. More formally, a relationship defines the nature of a link between two related entities. The purpose of this document is to provide greater clarity on:

- the broad categories of relationships
- names and definitions of the specific relationships defined by the Brick ontology
- guidelines, idioms and examples for how to apply these relationships in practice

## Philosophy of Brick Relationships

There are many possible perspectives on how a building may be described. The relationships defined by Brick outline several of these:

**Composition**: informally, what "things" can be assembled to make other "things", or what "things" make up other "things". There are several flavors of this. Physical composition describes what equipment can be composed of other equipment (e.g. a VAV may be made up of a damper, fan, reheat coil and so on), and how locations can be composed of other locations (e.g. a building is made up of floors and spaces). Logical composition describes how concepts can be broken down: an HVAC zone consists of a set of rooms, for example.

**Topology**: the way in which "things" are connected or arranged. This includes how equipment are connected and in what order they affect or modulate some media as it flows through the building, such as air or water. The topological perspective of a building also describes what spaces or rooms or zones are connected and which are next to each other.

**Telemetry**: the data *sources* associated or attached to various "things", be they logical, physical or virtual. In BMS-parlance, these are called "Points", and consist of the *digital* representations of the sensors, setpoints, commands, alarms and parameters that constitute the data produced by, for and on behalf of a building.

Brick provides a way to describe a building and its subsystems along each of these perspectives.

## Defining Brick Relationships

We list each of the Brick relationships related to each of the modeling perspectives described above. Each relationship has a *subject* (the "thing" owning the relationship, or the "thing" that the relationship is about) and an *object* (the "thing" that is the value of the relationship).

### Composition

`brick:hasPart`: the *subject* has some component or part identified by *object*; used to describe both physical and logical composition. This relationship is not typically used to desscribe the physical location of the *object* except in the case where the location of the *object* is fundamental to the identity of the *subject*. For example, a chair being located in a room is not fundamental to the definition of a room because a room can exist independent of whether or not a chair is located in it -- here, we would use the `brick:hasLocation` relationship (see below). However, a damper being "located" in a VAV is fundamental to the definition of a VAV because a VAV must be able to modulate the volume of air. In this case, we would use the `brick:hasPart` relationship.

### Topology

`brick:feeds`: the *subject* is arranged upstream of *object*, implying that some media flows from *subject* into *object*.

`brick:hasLocation`: the *subject* has a location given by *object*; this is the spatial notion of "location" and is not related to composition. See the definition of `brick:hasPart` above for a discussion of the difference


### Telemetry

`brick:hasPoint`: the *subject* has a source of telemetry identified by *object*. Generally this means that some aspect of *subject* is measured, controlled, configured or monitored, and the generated telemetry is identified by *object*. The type and definition of *object* dictates what aspect of *subject* is being represented by data.

## How and When to Use Brick Relationships

When your *subject* is a...

- **Location**:
  - `hasPart` describes the components of that location
    - Floor `hasPart` Room
    - HVAC Zone `hasPart` Room
- **Point**:
  - `hasLocation` describes where the point is physically located
    - Sensor `hasLocation` Room
- **Equipment**:
  - `hasPoint` describes telemetry associated with the equipment:
    - VAV `hasPoint` Temperature Sensor
    - Damper `hasPoint` Damper Position Command
  - `hasLocation` describes where the equipment is physically located
    - Thermostat `hasLocation` Room
  - `hasPart` describes the components of the equipment
    - VAV `hasPart` Damper
    - VAV `hasPart` Heating Coil
    - AHU `hasPart` Supply Fan
  - `feeds` describes downstream equipment and locations
    - AHU `feeds` VAV
    - VAV `feeds` HVAC Zone

### Quick Note on "Inverse" Relationships

Brick allows many relationships to be defined in two different directions through the use of an "inverse" relationship. This lends some flexibility to the modeler, and the vast majority of Brick-related software, databases and tooling will support the use of either direction.

| Relationship | Inverse |
|--------------|---------|
| `hasPoint`   | `isPointOf` |
| `hasPart`    | `isPartOf` |
| `hasLocation`| `isLocationOf` |
| `hasFeeds`   | `isFedBy` |

In all cases where we have `subject relationship object`, an equvalent statement is `object inverse-relationship subject`.
