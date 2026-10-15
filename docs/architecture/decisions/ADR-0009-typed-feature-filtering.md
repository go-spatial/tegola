[Documentation index](../../README.md)

# Read ADR-0009: Typed feature filtering and Queryables

Status: accepted architecture; implementation and provider admission require independent verification.

## Context

Feature queries already combine identity, bbox and exact Core datetime constraints.
Filtering adds public property predicates without exposing configured SQL or
changing tile queries. Tile field metadata does not establish raw property types,
publication lineage or comparison semantics.

## Decision

### Public catalog and neutral expressions

An optional provider capability supplies a detached, registration-finalized
Queryables catalog. Each entry maps a published raw property name to a proven
logical scalar type. Physical identifiers remain private to the provider.
Geometry storage, raw bounds and operational fields remain private. Feature
identity is not automatically a queryable; existing identity selectors remain
available. Public epoch-valued temporal properties remain integers.

The service copies catalogs during construction. Providers independently resolve
every filter against their own catalog and physical projection lineage. Caller
catalogs cannot authorize physical columns. Missing optional metadata leaves Core
publication usable and makes the additional capability explicitly unsupported.

The initial service transport catalog retains only public aliases expressible
as quoted identifiers in the selected CQL2 text grammar. Quoting does not admit
spaces or arbitrary strings under Annex B. A shared lexer name validator defines
this subset, without rewriting aliases or restricting the generic provider AST.
An empty eligible subset remains an available catalog. Core property output and
the provider's broader neutral catalog remain unchanged.

The neutral immutable AST contains enum operators, public names and validated
literals. It contains no SQL fragments. The initial expression profile supports
TRUE/FALSE, AND/OR/NOT, six scalar comparisons and IS NULL/IS NOT NULL. It uses
property-left/literal-right operands under CQL2 Permission 1. DATE and TIMESTAMP
are scalar types; advanced temporal functions are a separate capability. NULL
is source data, not a scalar literal in this text profile.

### Scalar semantics

- Numeric literals retain exact decimal/exponent values. Integer and number
  queryables share mathematical comparison semantics, including fractional
  bounds, unsigned values and values outside a physical column's range.
- An admitted floating-point field must compare its exact finite stored value
  against the mathematical literal. Silent literal rounding is forbidden.
  Unproved physical floating-point profiles are excluded from Queryables.
- Strings use Unicode scalar ordinal order, with significant case, accents and
  trailing spaces. They are not implicitly normalized or trimmed. Physical
  padding, collation and encoding must support these semantics before admission.
- Booleans are logical values, ordered false before true. Integer storage does
  not establish a boolean domain without independent proof.
- Dates are valid civil dates. Timestamps are exact UTC instants, preserving
  fractional tails and validated leap-second position. Core integer temporal
  mappings are unchanged. Native date/time profiles require separate physical,
  precision, session and serialization proofs.
- Comparisons with NULL yield UNKNOWN. Logical operators preserve three-valued
  logic; only TRUE selects a feature. Out-of-range simplification must preserve
  UNKNOWN under NOT, rather than replacing a nullable comparison with a constant.

Every advertised field supports all initial scalar comparisons and NULL tests.
Unsupported storage types are omitted rather than inferred from sampled rows.
Invalid source values produce source-data errors where the admitted profile
requires validation; coercion cannot conceal invalid storage.

### Compilation and execution

Each provider owns its dialect compiler. Only enum operators, frozen physical
identifiers and bound values enter generated SQL. Trusted parameter casts derive
from catalog metadata. Argument ordinals account for configured selection and
other existing parameters.

The configured feature selection and request filter are separate parenthesized
conjuncts, combined with identity, bbox and datetime restrictions. Filtering
precedes paging and exact counts. There is no collection-wide in-memory filter
fallback. Source integrity and catalog checks use the existing protected snapshot;
HANA retains its transaction-owned lock, cancellation and cleanup protocol.

SQLite's TEXT storage class does not establish valid UTF-8. The GPKG filtering
profile therefore uses a fixed, deterministic UTF-8 integrity function on
provider-owned connections, registered through their connection hook. It does
not register caller functions or replace the global SQLite driver. A lazy SQL
CASE checks storage class and a 1 MiB source-cell byte limit before passing text
to Go. The database encoding must be UTF-8. A same-snapshot SQL integrity probe
runs before the client filter, so that invalid text cannot be hidden by the
predicate. Invalid storage, UTF-8 or oversized admitted text fails with a source
data error. These probes can scan the source in SQL; they do not materialize a
collection in Go. Existing tile and unfiltered Core paths retain their behavior.

### Transport and limits

The initial transport is CQL2 text. Other encodings and advanced operators are
explicitly unsupported. Queryables uses Draft 2020-12, `application/schema+json`,
a canonical query-free `$id`, discovery links and `additionalProperties:false`.
Unknown/private properties, invalid syntax or incompatible literals are client
errors with value-free messages. Part 2 filter CRS support is separately gated.

Limits are checked before copying or allocating derived structures: 64 KiB input
and aggregate AST text, 8192 tokens, 4096 nodes, depth 32, 1024-byte property names,
16 KiB literal text, 1024 numeric digits and absolute numeric exponent 4096.
Timestamp fractions are bounded to 1024 digits. Catalogs allow at most 4096
entries, 1024 bytes per name and 64 KiB aggregate names. Exceeding a limit rejects
the request; no expression is truncated.

## Verification and consequences

Independent static-oracle tests cover three-valued logic, exact numbers, Unicode,
trailing spaces, booleans, scalar dates/timestamps, aliases and private fields.
Actual ordinary/custom provider fixtures verify domain composition, filtering
before paging/counts, bbox/datetime combinations, cancellation and injection.
Parser fuzzing and independent security review precede acceptance.

An implemented subset does not establish an OGC conformance claim. The discovery
declaration remains governed by the selected official verification gates.

## Sources

- [CQL2 1.0.0, Basic CQL2 and text encoding](https://docs.ogc.org/is/21-065r2/21-065r2.html)
- [OGC API Features Part 3, Queryables and filtering](https://docs.ogc.org/is/19-079r2/19-079r2.html)
- [Safe provider selection](ADR-0007-safe-provider-feature-sql.md)
- [HANA source protection](ADR-0008-hana-feature-source-lock.md)
