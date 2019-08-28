# Server

The server package is responsible for handling webserver requests for map tiles and various JSON endpoints describing the configured server. Example config:

```toml
[webserver]
port = ":9090" # set something different than default ":8080"

[webserver.headers]
Access-Control-Allow-Origin = "*"
```

### Config properties

- `port` (string): [Optional] Port and bind string. For example ":9090" or "127.0.0.1:9090". Defaults to ":8080"
- `hostname` (string): [Optional] The hostname to use in the various JSON endpoints. This is useful if tegola is behind a proxy and can't read the API consumer's request host directly.
- `uri_prefix` (string): [Optional] A prefix to add to all API routes. This is useful when tegola is behind a proxy (i.e. example.com/tegola). The prexfix will be added to all URLs included in the capabilities endpoint responses. 

## Local development of the embedded viewer

Tegola's built in viewer code is stored in the `static/` directory. In order to embed the static files into the tegola binary the package [go-bindata](github.com/jteeuwen/go-bindata) is used. Once `go-bindata` is installed the following command can be used to generate a .go file for inclusion in the tegola binary:

```
go-bindata -pkg=bindata -o=bindata/bindata.go -ignore=.DS_Store static/...
```

go-bindata also supports a debug mode which is descried as "Do not embed the assets, but provide the embedding API. Contents will still be loaded from disk." This mode is ideal for development and can be configured using the following command:

```
go-bindata -debug -pkg=bindata -o=bindata/bindata.go -ignore=.DS_Store static/...
```

## Disabling the viewer

The viewer can be excluded during building by using the build flag `noViewer`. For example, building tegola from the `cmd/tegola` directory:

```bash
go build -tags "noViewer"
```
