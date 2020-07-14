<template>
  <li v-on:click="toggleLayerVisibility(layer.name)">
    <div class="layer-color"
      v-bind:style="{'background-color': getLayerColor(layer.name), visibility: visibility}"></div>
    <div class="layer-name">
     {{layer.name}}
    </div>
    <div class="layer-zoom-range">{{zoomRange}}</div>
  </li>
</template>

<script>
import { map } from "@/globals/map";

export default {
  name: 'MapLayerRow',
  props: {
    layer: Object
  },
  computed:{
    zoomRange(){
      return 'z' + this.layer.minzoom + '-' + this.layer.maxzoom;
    }
  },
  data(){
    return {
      visibility: 'visible'
    }
  },
  methods:{
    // toggleLayerVisibility will toggle a layers visibility between on and off
    toggleLayerVisibility(layerName){
      var visibility = map.getLayoutProperty(layerName, 'visibility');
      if (visibility === 'visible') {
        map.setLayoutProperty(layerName, 'visibility', 'none');
        this.visibility = 'hidden';
      } else {
        map.setLayoutProperty(layerName, 'visibility', 'visible');
        this.visibility = 'visible';
      }
    },

    // getLayerColor returns the color of a layer in hex value. this method
    // uses mapbox's getPaintProperty. a different paint property is used
    // for each layer type: circle, line, fill.
    getLayerColor(layerName){
      let paintPropName;

      let layer = map.getLayer(layerName);
      switch (layer.type){
        case 'fill':
          paintPropName = 'fill-outline-color'
          break;
        case 'line':
          paintPropName = 'line-color'
          break;
        case 'circle':
          paintPropName = 'circle-color'
          break;
        default:
          console.log('unsupported layer type: '+ layer.type);
      }

      if (!paintPropName) {
        return "#000";
      }

      return map.getPaintProperty(layerName, paintPropName);
    }
  }
}
</script>

<!-- Add "scoped" attribute to limit CSS to this component only -->
<style scoped>
li {
  list-style: none;
  padding: 10px;
  border-radius: 3px;
  cursor: pointer;
  -webkit-user-select: none;
  -moz-user-select: none;
  -ms-user-select: none;
  user-select: none;
  white-space: nowrap;
  border-bottom: 1px solid #000;

  display: flex;
  flex-direction: row;
  flex-wrap: nowrap;
  justify-content: flex-start;
  align-items: stretch;
  align-content: stretch;

}
li:hover {
  color: #fff;
  background-color: #60bb1b10;
}

.layer-color {
  flex: 0 1 0.25em;
  height: 1.1em;
  display: inline-block;
  border-radius: .5em;
  border: 1px;
}

.layer-name {
  flex: 1 1 auto;
  overflow: hidden;
  padding: 0 .5em;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.layer-zoom-range{
  flex: 0 1 10%;
  height: 1em;
  border-radius: .5em;
  border: 1px;
  padding: 2px 3px;
  font-size: 12px;
  /*font-size: 1em;*/
  background-color: rgba(74, 73, 73, 0.25);
  float: right;
}
</style>
