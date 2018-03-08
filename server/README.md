# Server

The server package is responsible for handling webserver requests for map tiles and various JSON endpoints describing the configured server. Example config:

```toml
[webserver]
port = ":9090" # set something different than default ":8080"
```

### Config properties

- `port` (string): [Optional] Port and bind string. For example ":9090" or "127.0.0.1:9090". Defaults to ":8080"
- `hostname` (string): [Optional] The hostname to use in the various JSON endpoints. This is useful if tegola is behind a proxy and can't read the API consumer's request host directly.
- `cors_allowed_origin` (string): [Optional] The value to include with the Cross Origin Resource Sharing (CORS) `Access-Control-Allow-Origin` header. Defaults to `*`.


## Local development of the embedded inspector

tegola's built in viewer code is stored in the static/ directory. To generate a bindata file so the static assets can be compiled into the binary, [bindata-assetfs](https://github.com/elazarl/go-bindata-assetfs) is used. Once bindata-assetfs is installed the following command can be used to generate the file for inclusion:

```
go-bindata-assetfs -pkg=server -ignore=.DS_Store static/...
```

bindata-assetfs also supports a debug mode which is descried as "Do not embed the assets, but provide the embedding API. Contents will still be loaded from disk." This mode is ideal for development and can be configured using the following command:

```
go-bindata-assetfs -debug -pkg=server -ignore=.DS_Store static/...
```