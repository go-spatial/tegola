<template>
  <div id="header">
    <h1>Tegola ({{capabilities.version}})</h1>
  </div>
</template>

<script>
export default {
  name: 'Header',
  props:{
    capabilities: Object
  },
  data(){
    return {}
  },
}
</script>

<!-- Add "scoped" attribute to limit CSS to this component only -->
<style scoped>
#header {
  width: 100%;
  z-index: 100;
  /*position: fixed;*/
  position: absolute;
  border-bottom: 1px solid #eaeaea;
  background-color: rgba(0,0,0,0.5);
}
h1 {
  padding: 8px;
  top: 0;
  line-height: 40px;
  font-weight: bold;
  font-size: 16px;
  margin: 0;
}

h2 {
  padding: 10px;
  margin: 0;
  font-size: 14px;
  border-bottom: 1px solid #ccc;
}
</style>
