# Prometheus Observability Provider

The Prometheus Observability provider manages the collection of various
metrics for Tegola's various subsystems.

The connection between Tegola and Prometheus is configured in the Tegola config
file (`tegola.toml`.) An example of minimum configuration :

```toml

[observer]
type = "prometheus"

```

The metrics will be exposed on the `/metrics` end point.

### Configuration Properties

- `type` (string): [Required] the type of the observer, must be "`prometheus`".
- `variables` (array of strings): [Optional] A set of tokens that should be replaced in the url for HTTP or additional labels for cache providers. Currently, supported tokenks are:
  * `:map_name` [Default]
  * `:layer_name` [Default]
  * `:z` [Default]
  * `:x`
  * `:y`
- `push_url` (string) : [Optional] To push to a Prometheus Gateway, set the push_url to the gateway's URL. Note: this should only be used for ephemeral jobs, such as `tegola cache seed` or `tegola cache pruge` commands.
- `push_cadence` (int) : [Optional] How often to push to the Prometheus Gateway. Defaults to 10 secs. Use a zero or less to only push at the end of the process.


### Metrics exposed

#### tegola build info

#####  tegola_build_info 

A gauge with build information from the running command

###### Labels

* branch - the git branch this binary was built on if set
* revision - the git short revision number
* version - the set version number
* command - the running command
* tags - the build tags that were used to build the Tegola Binary


Use this in conjunction with other command to compare metrics across
versions of the application.

#### tegola server api http handlers

##### tegola_api_duration_seconds

A histogram of latencies for requests.

As part of a histogram include the support tags:
* tegola_api_duration_seconds_sum
* tegola_api_duration_seconds_count

###### labels

* handler is the URL with variables substituted in as requested.
* method is the HTTP method used to request the resource
* le is the buckets in seconds

##### tegola_api_flight_requests

A gauge of requests currently being served by the wrapped handler.

##### tegola_api_requests_total

A counter for requests to the wrapped handler.

###### labels

* code  is the returned HTTP code

##### tegola_api_response_size_bytes

A histogram of response sizes in bytes for requests.

As part of a histogram include the support tags:
* tegola_api_response_size_sum
* tegola_api_response_size_count

###### labels

* le is the buckets in bytes

#### tegola server viewer http handlers

##### tegola_viewer_duration_seconds

A histogram of latencies for requests.

As part of a histogram include the support tags:
* tegola_viewer_duration_seconds_sum
* tegola_viewer_duration_seconds_count

###### labels

* handler is the URL for the requested resource 
* method is the HTTP method used to request the resource
* le is the buckets in seconds

##### tegola_viewer_flight_requests

A gauge of requests currently being served by the wrapped handler.

##### tegola_viewer_requests_total

A counter for requests to the wrapped handler.

###### labels

* code  is the returned HTTP code

##### tegola_viewer_response_size_bytes

A histogram of response sizes in bytes for requests.

As part of a histogram include the support tags:
* tegola_viewer_response_size_sum
* tegola_viewer_response_size_count

###### labels

* le is the buckets in bytes


#### tegola cache 

##### tegola_cache_flight_requests

A gauge of requests currently being handled by the cache

##### tegola_cache_hits_total

A counter of the number of tile hits

###### labels

* sub_command is the cache command which is one of "get","set", or "purge"
* layer_name is an optional label, that is the layer_name ; this is only presetn if configured via `variables` config option.
* map_name is an optional label, that is the map_name; this is only present if configured via `variables` config option.
* z is an optional label, that is the z coordinate; this is only present if configured via `variables` config option.
* x is an optional label, that is the x coordinate; this is only present if configured via `variables` config option.
* y is an optional label, that is the y coordinate; this is only present if configured via `variables` config option.

##### tegola_cache_misses_total

A counter of the number of tile hits

###### labels

* sub_command is the cache command which is one of "get","set", or "purge"
* layer_name is an optional label, that is the layer_name ; this is only presetn if configured via `variables` config option.
* map_name is an optional label, that is the map_name; this is only present if configured via `variables` config option.
* z is an optional label, that is the z coordinate; this is only present if configured via `variables` config option.
* x is an optional label, that is the x coordinate; this is only present if configured via `variables` config option.
* y is an optional label, that is the y coordinate; this is only present if configured via `variables` config option.

##### tegola_cache_duration_seconds

A histogram of latencies for requests.

As part of a histogram include the support tags:
* tegola_cache_duration_seconds_sum
* tegola_cache_duration_seconds_count

###### labels

* sub_command is the cache command which is one of "get","set", or "purge"
* layer_name is an optional label, that is the layer_name ; this is only presetn if configured via `variables` config option.
* map_name is an optional label, that is the map_name; this is only present if configured via `variables` config option.
* z is an optional label, that is the z coordinate; this is only present if configured via `variables` config option.
* x is an optional label, that is the x coordinate; this is only present if configured via `variables` config option.
* y is an optional label, that is the y coordinate; this is only present if configured via `variables` config option.
* le is the buckets in seconds

##### tegola_cache_response_size_bytes

A histogram of response sizes for requests.

As part of a histogram include the support tags:
* tegola_cache_response_size_sum
* tegola_cache_response_size_count

###### labels

* sub_command is the cache command which is one of "get","set", or "purge"
* layer_name is an optional label, that is the layer_name ; this is only presetn if configured via `variables` config option.
* map_name is an optional label, that is the map_name; this is only present if configured via `variables` config option.
* z is an optional label, that is the z coordinate; this is only present if configured via `variables` config option.
* x is an optional label, that is the x coordinate; this is only present if configured via `variables` config option.
* y is an optional label, that is the y coordinate; this is only present if configured via `variables` config option.
* le is the buckets in bytes

#### go runtime information

##### go_gc_duration_seconds

A summary of the pause duration of garbage collection cycles.

##### go_goroutines

Number of goroutines that currently exist.

##### go_info

Information about the Go environment.

##### go_memstats_alloc_bytes

Number of bytes allocated and still in use.

##### go_memstats_alloc_bytes_total

Total number of bytes allocated, even if freed.

##### go_memstats_buck_hash_sys_bytes

Number of bytes used by the profiling bucket hash table.

##### go_memstats_frees_total

Total number of frees.

##### go_memstats_gc_cpu_fraction

The fraction of this program's available CPU time used by the GC since the program started.

##### go_memstats_gc_sys_bytes

Number of bytes used for garbage collection system metadata.

##### go_memstats_heap_alloc_bytes

Number of heap bytes allocated and still in use.

##### go_memstats_heap_idle_bytes

Number of heap bytes waiting to be used.

##### go_memstats_heap_inuse_bytes

Number of heap bytes that are in use.

##### go_memstats_heap_objects

Number of allocated objects.

##### go_memstats_heap_released_bytes

Number of heap bytes released to OS.

##### go_memstats_heap_sys_bytes

Number of heap bytes obtained from system.

##### go_memstats_last_gc_time_seconds

Number of seconds since 1970 of last garbage collection.

##### go_memstats_lookups_total

Total number of pointer lookups.

##### go_memstats_mallocs_total

Total number of mallocs.

##### go_memstats_mcache_inuse_bytes

Number of bytes in use by mcache structures.

##### go_memstats_mcache_sys_bytes

Number of bytes used for mcache structures obtained from system.

##### go_memstats_mspan_inuse_bytes

Number of bytes in use by mspan structures.

##### go_memstats_mspan_sys_bytes

Number of bytes used for mspan structures obtained from system.

##### go_memstats_next_gc_bytes

Number of heap bytes when next garbage collection will take place.

##### go_memstats_other_sys_bytes

Number of bytes used for other system allocations.

##### go_memstats_stack_inuse_bytes

Number of bytes in use by the stack allocator.

##### go_memstats_stack_sys_bytes

Number of bytes obtained from system for stack allocator.

##### go_memstats_sys_bytes

Number of bytes obtained from system.

##### go_threads

Number of OS threads created.
