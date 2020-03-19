module.exports = {
	// publicPath is configured to use a relative URI path during production
	// so the viewer works when behind a proxy
	publicPath: process.env.NODE_ENV === 'production'? '.': '/',

	// chainWebpack allows invoking webpack specific functionality.
	// it's used here to define external libs which occupy a global and are used
	// by require('global_var') statements in components
  chainWebpack: config => {
    config.externals({
      'mapboxgl': 'mapboxgl'
    })
  }
}