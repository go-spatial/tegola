# RedisCache

This package implements tegola's cache interface for use with Redis.
The connection is configured with the single `uri` property. If a redis
instance is running locally with default configurations solely for tegola,
simply include the following snippet in tegola's config file:

```toml
[cache]
type="redis"
uri="redis://127.0.0.1:6379/0"
```

## Properties

The rediscache config supports the following properties:

> [!IMPORTANT]
> The redis cache is configured with `uri` only. The legacy connection
> properties `network`, `address`, `password`, `db` and `ssl` were **removed**.
> A config that still sets any of them fails at startup with an error showing
> how to migrate. See
> [Migrating from the legacy connection keys](#migrating-from-the-legacy-connection-keys).

- `uri` (string): connection URL, parsed with go-redis
  [`redis.ParseURL`](https://redis.github.io/redis-go/api/#redis.ParseURL).
  Accepted schemes:

  - `redis://[<user>][:<password>@]<host>[:<port>]/<db>`: TCP connection
    without TLS, e.g. `uri="redis://127.0.0.1:6379/0"` or with a password and
    database index `3`: `uri="redis://:secret@127.0.0.1:6379/3"`. With Redis
    ACLs the username goes before the `:`:
    `uri="redis://alice:secret@127.0.0.1:6379/3"`. The host defaults to
    `localhost`, the port to `6379` and the database to `0`; the database
    index can also be given as a `db` query parameter.
  - `rediss://[<user>][:<password>@]<host>[:<port>]/<db>`: same as `redis://`,
    but the connection is TLS encrypted. Use the `skip_verify=true` query
    parameter to skip certificate verification.
  - `unix://[<user>][:<password>@]</path/to/redis.sock>?db=<db>`: unix socket.
    The socket path is mandatory and the database index is set with the `db`
    query parameter, e.g. `uri="unix:///var/run/redis.sock?db=2"`.

  Additional go-redis options can be set as query parameters (e.g.
  `?pool_size=10&dial_timeout=3s&read_timeout=2s`). Unknown query parameters
  and malformed URIs are rejected at startup.
- `max_zoom` (int): [Optional] the max zoom the cache should cache to.
  After this zoom, Set() calls will return before doing work.
- `ttl` (int): [Optional] the key ttl time in seconds. Defaults to 0
  (the key has no expiration time).

## Migrating from the legacy connection keys

The deprecated connection properties were removed. Replace them with the
equivalent `uri`.

Before:

```toml
[cache]
type="redis"
network="tcp"
address="127.0.0.1:6379"
password="secret"
db=3
ssl=true
```

After:

```toml
[cache]
type="redis"
uri="rediss://:secret@127.0.0.1:6379/3"
```

- `ssl=false` maps to the `redis://` scheme, `ssl=true` to the `rediss://`
  scheme.
- With `network="unix"` the socket path moves from `address` into the uri:
  `address="/var/run/redis.sock"` becomes `uri="unix:///var/run/redis.sock"`;
  combine it with `db=2` as `uri="unix:///var/run/redis.sock?db=2"`.
- The legacy connection used `PoolSize=2` and `DialTimeout=3s`. The uri based
  connection uses the go-redis defaults (pool size `10 * runtime.GOMAXPROCS`,
  dial timeout `5s`); tune them with query parameters if needed, e.g.
  `uri="redis://127.0.0.1:6379/0?pool_size=2&dial_timeout=3s"`.
