# Write ADR-0011: Schema, identity and revision mapping

Status: accepted on 2026-10-04.
Date: 2026-10-04

## Context

Writes need a stricter model than read queryables: input validation, XSD /
JSON Schema generation, UI forms, optimistic concurrency and audit all
derive from one schema source. Identity must be stable across protocols.

## Decision

1. `SchemaDescriptor` is the single source for public properties, storage
   mapping, logical types, precision, nullable/required/default,
   readOnly/writeOnly, lengths and allowed values, geometry/CRS/dimension,
   identity, revision strategy and system fields. One schema generates
   DescribeFeatureType XSD, the `/schema` JSON Schema, input validation
   rules and editor forms. Queryables stays a separate read projection.
2. Four states are distinct: absent, explicit NULL, empty value, DEFAULT.
   Integer/decimal values never pass through float64. Unknown properties
   are a schema error before any write.
3. `PhysicalFeatureKey` = transaction domain + physical relation + primary
   key. Locks, revisions and audit use it, so one object published under
   two collection names or via two protocols stays one object.
4. WFS FID reversibly encodes collection + ID and satisfies XML identifier
   constraints. The starting profile supports existing nonnegative integer
   IDs; UUID/negative/composite PKs are explicit future extensions.
5. Row revision is an explicit column (or other backend-proven mechanism),
   never implicitly `xmin`/timestamps. HTTP ETags are strong validators
   computed from a canonical representation; weak tags are rejected for
   If-Match.

## Consequences

Schema drift after startup fails closed until re-admission. Attribute-only
updates never re-encode stored geometry bytes.

## Verification and acceptance record

Schema and optimistic-concurrency tests cover schema parity, precision,
absent-vs-null and validator strength.
