# Connect and the kitchen ontology

Ontology: `agentic-salmon-kitchen`, version `1.0.0`.

| Subject | Relationship | Object |
| --- | --- | --- |
| food_portion | located_in | cooking_equipment |
| food_portion | has_visible_feature | observable_feature |

Perceive emits unresolved mentions. Connect resolves the two food mentions to
the event-scoped `food_portions_1` entity, aggregates distinct evidence claims,
validates relationship endpoint kinds, and constructs one food-preparation
assessment. Empty/duplicate IDs, unknown kinds, missing endpoints, and invalid
relationships are rejected. A canonical ID supplied by the fixture is rejected
as an unknown field; Connect owns entity resolution.

Reason evaluates candidates after connection. The unsupported soup-food
hypothesis is unknown; observed skin-like evidence supports the human fish
hypothesis, while the earlier model assertion of salmon remains plausible.
None is promoted to verified identity.
