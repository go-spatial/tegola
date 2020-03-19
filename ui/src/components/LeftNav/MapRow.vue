<template>
    <li v-on:click.self="setActiveMap(map)">
      {{map.name}}<img class="arrow" src="@/assets/svg/arrow.svg"/>
    </li>
</template>

<script>
import { mutations } from "@/globals/store";

export default {
  name: 'MapRow',
  props: {
    map: Object
  },
  methods:{
    setActiveMap: mutations.setActiveMap
  }
}
</script>

<!-- Add "scoped" attribute to limit CSS to this component only -->
<style scoped>
li {
  list-style: none;
  padding: 10px;
  cursor: pointer;
  -webkit-user-select: none;
  -moz-user-select: none;
  -ms-user-select: none;
  user-select: none;
  display:block;
  background-color: #33333375;
  border-bottom: 1px solid #000;
}
li:hover{
  color: #fff;
  background-color: #33333350;
}
.arrow {
  float: right;
  width: 1em;
}
</style>
