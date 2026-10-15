# ADR-0006: Exact RFC 3339 query boundaries

Status: accepted on 2026-10-01 after independent mathematical/representation review and authoritative IERS table verification; exact datetime bounds are implemented in the current query contract.

## Context

Core datetime uses RFC 3339 timestamps. RFC 3339 section 5.6 permits arbitrary decimal fractional precision and lower-case t/z. Go time.Time stores nanoseconds and its parser truncates excess fractional digits. Querying a subnanosecond instant must not accidentally select a neighbouring stored timestamp. RFC 3339 also permits actual announced leap seconds; POSIX integer source timestamps cannot identify an instant inside the inserted second.

## Decision

Retain TemporalConstraint.Start/End *time.Time and add StartSubNanosecond and EndSubNanosecond string, plus StartLeapSecond and EndLeapSecond bool. Decimal strings contain only digits and encode the remaining fraction of one nanosecond after the first nine fractional-second digits. Empty/all-zero strings mean zero. They require the corresponding endpoint. They contain neither raw HTTP syntax nor SQL. Validation compares endpoint times and exact decimal remainder without floating point or arbitrary precision-number allocation. Do not truncate nonzero remainders, round query instants or collapse empty instant matches into absent temporal geometry.

For a leap endpoint, Time stores the previous ordinary second with the requested fractional part, and LeapSecond marks that its fraction belongs to the following inserted second. Compare UTC epoch seconds first; within the same ordinary second all normal instants precede leap instants; within each class compare nanoseconds and padded decimal remainder. Flags require the normalized preceding UTC second to be a published positive leap insertion. The HTTP parser admits second60 only on a pinned authoritative positive-insertion date, applying timezone offsets before checking the insertion. Future unannounced leap values are invalid RFC 3339 timestamps. Preserve original valid datetime syntax in paging links.

Source unix_seconds/milliseconds/microseconds/nanoseconds are POSIX timestamps interpreted as ordinary UTC instants. For ordinary bounds, floor remains the integer floor; ceil increments when either discarded nanoseconds or a nonzero subnanosecond remainder exists. For a leap bound, the first representable source instant at or after it is the next ordinary whole second; the last at or before it is the final source tick of the preceding ordinary second. Thus an instant within the inserted second matches no stored POSIX instant, while absent temporal geometry still matches and source intervals spanning the insertion overlap correctly. Inclusive interval predicates use these exact bounds; checked big integer date arithmetic preserves the existing int64-extreme behavior.

Validation and source comparisons remain inclusive, support one open endpoint and reject reversed/both-open ranges. Instant constraints set identical endpoint fractions/flags. No new storage clock scale or leap-coordinate fabrication is introduced.

## Verification

Independent cases cover tenth decimal digit, arbitrarily long zero/nonzero tails, reversed same-nanosecond bounds, negative epochs, all four source units, exact intervals and absent geometry. Leap cases cover the announced 2016 insertion, fractional leap instants, timezone equivalents, ordinary timestamps immediately before/after, an interval spanning the insertion, invalid insertion dates and source int64 edges. Provider parity adapters must implement the same bounds before claiming contract support.

## Sources

- [RFC 3339 sections 5.6–5.7](https://www.rfc-editor.org/rfc/rfc3339.html#section-5.6).
- [IERS Leap_Second.dat](https://hpiers.obspm.fr/iers/bul/bulc/Leap_Second.dat), retrieved 2026-10-01, SHA256 `6cb6f5d4b819f2e568e25db4b0b26d89dedf031fdffb18bc94d40f4e94e268d7`. Bulletin 72 update, expiry 2027-06-28; 1972-01-01 offset10 is the baseline, subsequent 27 positive increments define insertions through 2016-12-31. The local evidence copy is an ignored execution artifact; production uses the reviewed insertion dates directly.
- [IERS Bulletin C72](https://hpiers.obspm.fr/iers/bul/bulc/bulletinc.dat), issued 2026-07-06, confirms no December 2026 insertion. Update the production date table when IERS announces a new insertion; runtime does not fetch a mutable table.
