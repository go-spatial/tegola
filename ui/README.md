# ui

The client side code for tegola's internal viewer. This codebase is built using vue.js 2.6 and uses [vite](https://vite.dev/) for building. The following npm commands can be used for basic operations:

## Project setup

```shell
npm install
```

### Compiles and hot-reloads for development

```shell
npm run dev
```

### Compiles and minifies for production

```shell
npm run build
```

## Building for inclusion in tegola

In order to compile the UI for inclusion in tegola, run the following commands from the `ui` folder:

```shell
go run build.go
```

