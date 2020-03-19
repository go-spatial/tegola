import Vue from "vue";

export const store = Vue.observable({
  // activeMap is the map the user is currently interacting with
  activeMap: null,

  // apiRoot is the prefix to append to app API requests
  apiRoot: null,

  // capabilities holds the TileJSON returned by tegola on load
  capabilities: null,

  // mbglIsReady is a flag to indicate that Mapbox GL is loaded and the style is loaded
  mbglIsReady: false
});

export const mutations = {
  setActiveMap(map){
    store.mbglIsReady = false;
    store.activeMap = map;
  },
  setApiRoot(apiRoot){
    store.apiRoot = apiRoot;
  },
  setCapabilities(capabilities){
    store.capabilities = capabilities;
  },
  setMbglIsReady(val){
    store.mbglIsReady = val;
  }
};