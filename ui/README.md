# ui

The client side code for tegola's internal viewer. This codebase is built using vue.js 2.6 and requires installing the [vue-cli](https://cli.vuejs.org/). After installing vue-cli the following npm commands can be used for basic operations:

## Project setup
```
npm install
```

### Compiles and hot-reloads for development
```
npm run serve
```

### Compiles and minifies for production
```
npm run build
```

### Lints and fixes files
```
npm run lint
```

### Customize configuration
See [Configuration Reference](https://cli.vuejs.org/config/).


## Building for inclusion in tegola

In order to compile the UI for inclusion in tegola, run the following commands from the `ui` folder:

```console
$ go run build.go
```

