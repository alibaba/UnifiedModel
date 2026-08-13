# Use stable, domain-scoped identity

Identify entities by stable, domain-scoped fields — not by display names or volatile attributes.

## Why

An entity's ID is derived from its `primary_key_fields` through the entity set's `id_generator`. If identity depends on a display name or a value that changes (a label, a mutable status), the same real-world object produces a different ID over time — breaking topology edges, deduplication, and historical continuity. Stable identity keeps an entity the *same* entity across observations, documents, tests, and screenshots.

## Do / Don't

| Do | Don't |
|---|---|
| Choose `primary_key_fields` that are immutable and unique within the domain (e.g. a resource ARN, or a `cluster` + `namespace` + `name` tuple). | Use `display_name` or human-facing labels as identity. |
| Keep the entity set `name` stable and domain-scoped (`devops.service`). | Rename an entity set to change its meaning, or reuse one name across domains. |
| Reuse the same IDs across sample data, tests, and docs. | Regenerate IDs per environment. |

## See also

- [Model Authoring](/en/guides/model-authoring)
- [Entity Sets](/en/concepts/entity-sets)
- [Entity And Relation Writes](/en/guides/entity-relation-writes)
