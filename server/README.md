# Sever


## Local development of the embeded inspector

tegola's built in viewer code is stored in the static/ directory. To generate a bindata file so the static assets can be compiled into the binary, [bindata-assetfs](https://github.com/elazarl/go-bindata-assetfs) is used. Once bindata-assetfs is installed the following command can be used to generate the file for inclusion:

```
go-bindata-assetfs -pkg=server -ignore=.DS_Store static/...
```

bindata-assetfs also supports a debug mode which is descriped as "Do not embed the assets, but provide the embedding API. Contents will still be loaded from disk." This mode is ideal for development and can be configured using the following command:

```
go-bindata-assetfs -debug -pkg=server -ignore=.DS_Store static/...
```