<template>
  <div id="app">
    <Mapbox v-if="capabilities && activeMap"/>
    <Header
      v-if="capabilities"
      v-bind:capabilities="capabilities"/>
    <LeftNav
      v-if="capabilities"
      v-bind:capabilities="capabilities"/>
  </div>
</template>

<script>
import "mapbox-gl/dist/mapbox-gl.css";
import Header from './components/Header.vue';
import LeftNav from './components/LeftNav/LeftNav.vue';
import Mapbox from './components/Mapbox.vue';
import { store, mutations } from "./globals/store";

const axios = require('axios');

// for production the apiRoot is empty so relative URLs are used
let apiRoot = '';
if (process.env.NODE_ENV != 'production'){
  // for development it's easier to use a remote capabilities endpoint
  apiRoot = 'https://osm-lambda.tegola.io/v1/';
}

export default {
  name: 'App',
  components: {
    Header,
    LeftNav,
    Mapbox
  },
  computed: {
    activeMap(){
      return store.activeMap
    },
    capabilities() {
      return store.capabilities
    }
  },
  methods:{
    // compareMaps compares two map objects for the sake of sorting alphabetically
    compareMaps(a, b){
      // ignore character casing
      const mapA = a.name.toLowerCase();
      const mapB = b.name.toLowerCase();

      let comparison = 0;
      if (mapA > mapB) {
        comparison = 1;
      } else if (mapA < mapB) {
        comparison = -1;
      }

      return comparison;
    }
  },
  created: function(){
    const me = this;

    // update the global store with the API root
    mutations.setApiRoot(apiRoot);

    // fetch the tegola capabilities endpoint. this is the root driver of
    // all subsequent steps
    axios.get(apiRoot + 'capabilities')
      .then(function (resp) {
        // sort our map list alphabetically
        resp.data.maps.sort(me.compareMaps);

        // on success update the capabilities data in the global store
        mutations.setCapabilities(resp.data);

        // check that we have maps configured in the response data
        if(resp.data.maps.length === 1){
          // find the first map in the capabilities and set it to the activeMap
          mutations.setActiveMap(resp.data.maps[0]);
        }
      })
      .catch(function (error) {
        // handle error
        console.log(error);
      })
  },
  data: function(){
    return {}
  }
}
</script>

<style>
body, html {
  font-family: Helvetica, Arial, sans-serif;
  background-color: #000;
  color: #ccc;
  margin: 0;
}


.float-r {
  float: right;
  padding-left: 1rem;
}

#app {
  -webkit-font-smoothing: antialiased;
  -moz-osx-font-smoothing: grayscale;
}

.mapboxgl-ctrl-toggle-tile-boundaries {
  background-image: url("data:image/svg+xml,%3C%3Fxml version='1.0' encoding='utf-8'%3F%3E%3C!-- Svg Vector Icons : http://www.onlinewebfonts.com/icon --%3E%3C!DOCTYPE svg PUBLIC '-//W3C//DTD SVG 1.1//EN' 'http://www.w3.org/Graphics/SVG/1.1/DTD/svg11.dtd'%3E%3Csvg version='1.1' xmlns='http://www.w3.org/2000/svg' xmlns:xlink='http://www.w3.org/1999/xlink' x='0px' y='0px' viewBox='0 0 1000 1000' enable-background='new 0 0 1000 1000' xml:space='preserve'%3E%3Cmetadata%3E Svg Vector Icons : http://www.onlinewebfonts.com/icon %3C/metadata%3E%3Cg%3E%3Cg transform='translate(0.000000,511.000000) scale(0.100000,-0.100000)'%3E%3Cpath d='M2060,4030v-980h-980H100v-326.7v-326.7h980h980v-980v-980h-980H100V110v-326.7h980h980v-980v-980h-980H100v-326.7V-2830h980h980v-980v-980h326.7h326.7v980v980h980h980v-980v-980H5000h326.7v980v980h980h980v-980v-980h326.7H7940v980v980h980h980v326.7v326.7h-980h-980v980v980h980h980V110v326.7h-980h-980v980v980h980h980v326.7V3050h-980h-980v980v980h-326.7h-326.7v-980v-980h-980h-980v980v980H5000h-326.7v-980v-980h-980h-980v980v980h-326.7H2060V4030z M4673.3,1416.7v-980h-980h-980v980v980h980h980V1416.7z M7286.7,1416.7v-980h-980h-980v980v980h980h980V1416.7z M4673.3-1196.7v-980h-980h-980v980v980h980h980V-1196.7z M7286.7-1196.7v-980h-980h-980v980v980h980h980V-1196.7z'/%3E%3C/g%3E%3C/g%3E%3C/svg%3E");
}

.mapboxgl-popup-content {
  position: relative;
  background-color: rgba(0,0,0,0.75);
  color: #ccc;
    border-radius: 3px;
    border: 1px solid #ccc;
    box-shadow: 0 1px 2px rgba(0,0,0,0.10);
    padding: 10px;
    pointer-events: auto;
}
.mapboxgl-popup-content h4 {
    margin: 0 0 .5em 0;
    border-bottom: 1px solid #ccc;
}
.mapboxgl-popup-content ul {
  margin: 0 0 1em 0;
  list-style: none;
  padding: 0;
}
.mapboxgl-popup-content ul>li {
  list-style: none;
  padding: 0;
}
.mapboxgl-popup-content ul:last-child {
  margin-bottom: 0;
}
</style>
