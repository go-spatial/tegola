<template>
  <div id='map'></div>
</template>

<script>
import { store, mutations } from "@/globals/store";
import { mapSetters } from "@/globals/map";
import ToggleTileBoundariesControl from "./MapboxControls"
import mapboxgl from "mapbox-gl";

export default {
  name: 'Mapbox',
  mounted () {
    // build the style url
    let url = store.apiRoot + 'maps/' + store.activeMap.name + '/style.json'

    // instantiate Mapbox GL
    let m = new mapboxgl.Map({
      container: 'map',
      style: url,
      hash: true
    });

    m.on('load', function(){
      // add navigation control
      let nav = new mapboxgl.NavigationControl();
      m.addControl(nav, 'bottom-right');

      // custom controls
      let debugLines = new ToggleTileBoundariesControl();
      m.addControl(debugLines, 'bottom-right');
    });

    m.on('styledata', function(){
      if (!store.mbglIsReady){
        mutations.setMbglIsReady(true);
      }
    });

    mapSetters.map(m);
  }
}

</script>

<!-- Add "scoped" attribute to limit CSS to this component only -->
<style scoped>
#map {
  position:absolute;
  top:0;
  bottom:0;
  width:100%;
}
</style>
