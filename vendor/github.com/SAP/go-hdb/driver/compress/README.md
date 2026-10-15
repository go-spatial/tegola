# Compression

go-hdb can compress the data exchanged with a HANA database using LZ4. When the
server advertises compression support and a [Compressor](#the-compressor-interface)
is configured on the connector, request and response payloads are compressed on
the wire, reducing network traffic.

## Design goals

go-hdb follows a **pure Go, no CGO, no third-party runtime dependencies** approach.
LZ4 compression is inherently in tension with this: the reference C library
(`liblz4`) is the canonical implementation, and pulling a compression library
into go-hdb would work against several of its principles at once:

- **Attack surface** — a bundled dependency (and its transitive dependencies)
  becomes part of the supply-chain and security surface of every go-hdb consumer,
  whether or not they ever enable compression.
- **Dependency weight** — importing a third-party LZ4 package adds `go.mod`/`go.sum`
  entries, license/REUSE obligations, and version/vulnerability churn that every
  consumer inherits, again independent of whether they use compression.
- **Neutrality** — go-hdb does not want to bless, pin, or take responsibility on
  its consumers' behalf for the correctness and security of one particular
  third-party compression library.

To avoid all of this, go-hdb ships **no built-in compressor**. Instead it defines
a small [`Compressor`](#the-compressor-interface) interface and lets the
application **bring its own implementation**. If no compressor is configured,
the driver behaves exactly as before — no compression, no extra dependencies.

## The `Compressor` interface

```go
type Compressor interface {
    EnableWrite() bool                       // send data compressed if the server supports it
    Decompress(src, dst []byte) (int, error) // decompress src into dst, returning bytes written
    CompressBound(n int) int                 // upper bound of compressed size for n input bytes
    Compress(src, dst []byte) (int, error)   // compress src into dst, returning bytes written
}
```

- `EnableWrite` controls the *outbound* direction only. Compression is negotiated
  at connect time via two independent flags:
  - Configuring **any** compressor makes the driver advertise that it can *receive*
    compressed packets. Inbound packets are then decompressed whenever the
    server marks them compressed (each packet carries an `isCompressed` header
    bit that the driver honours individually) — regardless of `EnableWrite`.
  - `EnableWrite` additionally advertises that the driver wants to *send*
    compressed packets. Return `false` to decompress inbound traffic but never
    compress outbound traffic.
- `CompressBound` must return a size large enough to hold the compressed output of
  an `n`-byte input; the driver uses it to size the destination buffer before
  calling `Compress`. A valid LZ4 bound is always at least `n` (incompressible
  input expands). If the returned value is smaller than `n` — including the `0`
  that the reference C `LZ4_compressBound` yields for input above its maximum
  block size — the driver treats the bound as invalid and sends the packet
  uncompressed instead of calling `Compress`.
- `Compress` / `Decompress` operate on caller-provided buffers and return the
  number of bytes written to `dst`.

Outbound compression is not unconditional. Mirroring the SAP HANA C++ client,
the driver sends a packet uncompressed when either the payload is below a size
threshold (small payloads aren't worth compressing) or compression fails to
save enough to be worthwhile; in those cases `Compress` output is discarded and
the original bytes are sent.

The wire format must be compatible with HANA's LZ4 compression, which uses the
LZ4 **block** format (not the LZ4 frame format). Any library implementing
standard LZ4 block (de)compression — for example a pure Go LZ4 package — can be
adapted to this interface.

## Configuring a compressor

Set the compressor on the [`Connector`](https://pkg.go.dev/github.com/SAP/go-hdb/driver#Connector):

```go
connector.SetCompressor(myCompressor)
```

Passing `nil` resets the connector to the package default
(`compress.DefaultCompressor`). In the default build that default is `nil`, i.e.
compression is disabled.

## Reference implementation

The package includes a reference `Compressor` backed by the C `liblz4` library.

**It is not intended for production use** — it depends on CGO and a system
`liblz4`, which violates go-hdb's pure Go approach and is exactly the dependency
we do not want to impose on consumers. It exists solely as a **known-correct
reference** for the wire format and to drive the package tests against the
canonical LZ4 implementation.

It is guarded by the `liblz4` build tag, so neither CGO nor `liblz4` are
dependencies of the default build path or of go-hdb consumers:

- Default build (no tag): `DefaultCompressor == nil` — compression disabled.
- With `-tags=liblz4`: `DefaultCompressor` is the `liblz4`-backed compressor.

Developer-machine prerequisites for building/testing with the tag:

- macOS: `brew install lz4`
- Linux: `apt install liblz4-dev` (or the distribution equivalent)

Run the tests with:

```sh
go test -v -tags=liblz4
```

For your own production deployments, provide a `Compressor` implementation that
fits your dependency and security requirements rather than relying on the
reference implementation.
