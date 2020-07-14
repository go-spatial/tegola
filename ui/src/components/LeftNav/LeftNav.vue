<template>
  <div id="left-nav" class="sidebar" :class="sidebartoggle">
    <div class="toggle" @click="togglesidebar">
      <svg
        class="toggle-arrow"
        width="2em"
        height="2em"
        viewBox="0 0 16 16"
        fill="currentColor"
        xmlns="http://www.w3.org/2000/svg"
      >
        <path
          fill-rule="evenodd"
          d="M5.854 4.646a.5.5 0 0 0-.708 0l-3 3a.5.5 0 0 0 0 .708l3 3a.5.5 0 0 0 .708-.708L3.207 8l2.647-2.646a.5.5 0 0 0 0-.708z"
        />
        <path
          fill-rule="evenodd"
          d="M10 8a.5.5 0 0 0-.5-.5H3a.5.5 0 0 0 0 1h6.5A.5.5 0 0 0 10 8zm2.5 6a.5.5 0 0 1-.5-.5v-11a.5.5 0 0 1 1 0v11a.5.5 0 0 1-.5.5z"
        />
      </svg>
    </div>
    <h2>
      <span v-on:click="showAllMaps">Maps</span>
      <span v-if="activeMap">/ {{ activeMap.name }}</span>
    </h2>
    <div class="container">
      <ul id="maps-list" v-if="!activeMap">
        <MapRow
          v-for="map in capabilities.maps"
          v-bind:key="map.name"
          v-bind:map="map"
        />
      </ul>
      <ul id="map-layers-list" v-if="activeMap && mapIsReady">
        <MapLayerRow
          v-for="layer in activeMap.layers"
          v-bind:key="layer.name"
          v-bind:layer="layer"
        />
      </ul>
    </div>
    <div id="left-nav-footer" v-if="activeMap">
      <div
        class="btn"
        v-on:click="toggleFeatureInspector"
        v-bind:class="{ active: inspectorIsActive }"
      >
        <span class="dot"></span>Inspect Features
      </div>
    </div>
  </div>
</template>

<script>
import MapRow from "./MapRow.vue";
import MapLayerRow from "./MapLayerRow.vue";
import { store, mutations } from "@/globals/store";
import { map } from "@/globals/map";

// const mapboxgl = require('mapboxgl');
import mapboxgl from "mapbox-gl";

export default {
  name: "LeftNav",
  components: {
    MapRow,
    MapLayerRow,
  },
  props: {
    capabilities: Object,
  },
  data() {
    return {
      inspectorIsActive: false,
      inspector: null,
      sidebartoggle: "",
    };
  },
  computed: {
    activeMap() {
      return store.activeMap;
    },
    mapIsReady() {
      return store.mbglIsReady;
    },
  },
  methods: {
    togglesidebar() {
      if (this.sidebartoggle == "sidebar-collapsed") {
        this.sidebartoggle = "sidebar-expanded";
      } else {
        this.sidebartoggle = "sidebar-collapsed";
      }

      // this.sidebartoggle = "sidebar-collapsed2"
      this.expanded = !this.expanded;
      this.collapsed = !this.collapsed;
    },

    // toggleFeatureInspector handles binding and unbinding the mouse events
    // necessary for the feature inspector
    toggleFeatureInspector() {
      if (!this.inspector) {
        // new popup instance
        this.inspector = new mapboxgl.Popup();
      }

      if (!this.inspectorIsActive) {
        map.on("mousemove", this.inspectFeatures);
        this.inspectorIsActive = true;
      } else {
        map.off("mousemove", this.inspectFeatures);

        this.inspectorIsActive = false;
        if (this.inspector.isOpen()) {
          this.inspector.remove();
          this.inspector = null;
        }
      }
    },

    // inspectFeatures handles querying the map instance at the position of the cursor
    // sorting the returned feature keys, building up the HTML fragments and injecting
    // the HTML into a mapbox GL popup instance.
    //
    // TODO (arolek): this should be refactored. It was ported from the original tegola viewer
    // and is quity ugly to look at and maintain. The UX would be better if no popup was used
    // as the feature properties often produce a list longer than the screen.
    inspectFeatures(e) {
      var html = "";
      var bbox = {
        width: 10,
        height: 10,
      };

      // query within a few pixels of the mouse to give us some tolerance to work with
      var features = map.queryRenderedFeatures([
        [e.point.x - bbox.width / 2, e.point.y - bbox.height / 2],
        [e.point.x + bbox.width / 2, e.point.y + bbox.height / 2],
      ]);

      // everPresent contains the keys that should be "pinned" to the top of the feature inspector. Others
      // will follow and simply be ordered by alpha. See https://github.com/go-spatial/tegola/issues/367
      var everPresent = ["name", "type", "featurecla"];
      for (var i = 0, l = features.length; i < l; i++) {
        html += "<h4>" + features[i].layer.id + "</h4>";
        html += "<ul>";
        html +=
          '<li>feature id <span class="float-r">' +
          features[i].id +
          "</span></li>";
        everPresent.forEach(function (key) {
          if (typeof features[i].properties[key] !== "undefined") {
            html +=
              "<li>" +
              key +
              '<span class="float-r">' +
              features[i].properties[key] +
              "</span></li>";
          }
        });
        Object.keys(features[i].properties)
          .sort()
          .forEach(function (key) {
            if (everPresent.indexOf(key) < 0) {
              html +=
                "<li>" +
                key +
                '<span class="float-r">' +
                features[i].properties[key] +
                "</span></li>";
            }
          });
        html += "</ul>";
      }

      if (html != "") {
        this.inspector.setLngLat(e.lngLat).setHTML(html).addTo(map);
      } else {
        if (this.inspector.isOpen()) {
          this.inspector.remove();
        }
      }
    },

    showAllMaps() {
      // remove the URL hash so the next map load does not use the current map
      // position but rather the init position for that map
      this.removeHash();

      // remove the current active map
      mutations.setActiveMap(null);
    },

    // removes the hash (#) from the URL
    // https://stackoverflow.com/questions/1397329/how-to-remove-the-hash-from-window-location-url-with-javascript-without-page-r/5298684#5298684
    removeHash() {
      history.pushState(
        "",
        document.title,
        window.location.pathname + window.location.search
      );
    },
  },
};
</script>

<!-- Add "scoped" attribute to limit CSS to this component only -->
<style scoped>


.sidebar {
  z-index: 100;
  width: 300px;
  position: fixed;
  /*position: absolute;*/
  background-color: rgba(0, 0, 0, 0.5);
  display: flex;
  flex-flow: column;
  height: 90%;
  top: 57px;
}

.toggle {
  position: absolute;
  width: 32px;
  height: 32px;
  right: -7px;
  top: 2px;
  cursor: pointer;
  margin: 0px;
  fill: white;
}

.toggle-arrow {
  fill: whitesmoke;

}

.sidebar {
  transition: all 0.6s ease-in-out;
}

.sidebar-collapsed {
  transform: translateX(-100%);
}


.sidebar-collapsed .toggle {
  animation-name: slindeinearrow;
  animation-duration: 0.6s;
  animation-delay: 0.1s;
  animation-fill-mode: forwards;
  animation-direction: normal;
}

.sidebar-expanded .toggle {
  animation-name: slideoutarrow;
  animation-duration: 0.6s;
  animation-delay: 0.1s;
  animation-fill-mode: forwards;
  animation-direction: normal;
  transform: translateX(20px) rotate(180deg);
}

@keyframes slindeinearrow {
  0% {

  }
  100% {

    transform: translateX(20px) rotate(180deg);
  }
}

@keyframes slideoutarrow {
  0% {

  }
  100% {

    transform: rotate(0deg);
  }
}

@keyframes slidein {
  0% {

  }
  100% {

    transform: translateX(20px) rotate(180deg);
  }
}

@keyframes slideback {
  0% {
    transform: rotate(180deg);
  }
  100% {
    transform: rotate(0deg);
  }
}

.container {
  width: 100%;
  flex: 1 1 auto;
  overflow-y: auto;
}

#left-nav-footer {
  flex: 0 1 40px;
}

h2 {
  padding: 10px;
  margin: 0;
  font-size: 14px;
  border-bottom: 1px solid #ccc;
}
h2 span {
  cursor: pointer;
}

#maps-list {
  margin: 0;
  padding: 0;
  font-size: 14px;
  height: 100%;
}

#map-layers-list {
  display: flex;
  flex-flow: column;
  height: 100%;
  margin: 0;
  padding: 0;
  list-style: none;
  font-size: 14px;
}

.btn {
  display: block;
  padding: 6px 12px;
  margin: 5px;
  font-size: 14px;
  font-weight: 400;
  line-height: 1.42857143;
  text-align: center;
  white-space: nowrap;
  vertical-align: middle;
  cursor: pointer;
  user-select: none;
  border: 1px solid #444;
  border-radius: 4px;
}
.btn:hover {
  border-color: #666;
  color: #eee;
}
.btn .dot {
  border-radius: 2px;
  width: 8px;
  height: 8px;
  display: inline-block;
  background-color: #333;
  margin-right: 6px;
}
.btn.active .dot {
  background-color: #259b24;
}
</style>
