# RedisCache

This package implements tegola's cache interface for use with Redis. If a redis instance is running locally with default configurations solely for tegola simply include the following snippet in tegola's config file:

```toml
[cache]
type="redis"
```

## Properties
The rediscache config supports the following properties:

- `network` (string): [Optional] Network type, either `tcp` or `unix`. Defaults to 'tcp'.
- `address` (string): [Optional] the address of the Redis instance in form of `ip:port`. Defaults to '127.0.0.1:6379'.
- `password` (string): [Optional] password for the Redis instance. Defaults to '' (no password).
- `db` (int): [Optional] the database within the Redis instance to cache to.
- `max_zoom` (int): [Optional] the max zoom the cache should cache to. After this zoom, Set() calls will return before doing work.
- `ttl` (int): [Optional] the key ttl time in seconds. Defaults to 0 (the key has no expiration time).
- `ssl` (bool): [Optional] whether to use SSL/TLS to encrypt the connection to the server. Defaults to false (no encryption)
