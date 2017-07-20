'use strict';

var app = new Vue({
	el: '#app',
	map: null,		//	map reference
	data: {
		//	stored reference from the capabilities endpoint response
		capabilities: {
			version: null,
			maps: []
		},
		//	built out based on the /capabilities response and style.json response
		maps:[
		/*	data model 
			{
				name: '',
				layers: [
					{
						name: '',
						visibility: '',	// 'none' / 'visibile'
						color: ''		//	hex value, i.e. #fff
					}
				]
			}
		*/
		]
	},
	created: function(){
		var me = this;
		//	read server capabilities
		get("/capabilities", function(res){
			me.$data.capabilities = JSON.parse(res.response);

			//	load our first map
			if (me.$data.capabilities.maps.length != 0){
				me.loadMap(me.$data.capabilities.maps[0].name);
			}
		});
	},
	methods: {
		//	uses the capabilities data and the active map to organize the map and layer data
		//	structure for rendering
		setData:function(){
			var maps = this.$data.capabilities.maps;
			var mapsList = [];

			//	iterate our maps
			for (var i=0, len=maps.length; i<len; i++) {
				var mapItem = {
					name: maps[i].name,
					layers: []
				};

				var layers = maps[i].layers
				//	iterate our map layers
				for (var j=0, l=layers.length; j<l; j++){
					mapItem.layers.push({
						name: layers[j].name,
						visibility: this.map.getLayoutProperty(layers[j].name, 'visibility') === 'visible' ? 'visible' : 'hidden',
						color: this.map.getPaintProperty(layers[j].name, 'line-color')
					})				
				}
				mapsList.push(mapItem);
			}

			//	update our app data
			this.$data.maps = mapsList;
		},
		loadMap: function(mapName){
			if (!mapName){
				return;
			}
			var me = this;
			var maps = this.$data.capabilities.maps;
			var mapRec;
			for (var i=0, len=maps.length; i<len; i++) {
				if (maps[i].name == mapName){
					mapRec = maps[i];
					break;
				}
			}
			//	initial load
			if (!me.map){
				me.map = new mapboxgl.Map({
					container: 'map',
					style: '/maps/'+mapRec.name+'/style.json',
					hash: true 
				});
				me.map.addControl(new mapboxgl.NavigationControl());
			} else {
				me.map.setStyle('/maps/'+mapRec.name+'/style.json');
			}

			me.map.on('load', me.setData);
			//me.map.on('zoomend', me.setData)
		},
		toggleLayerVisibility: function(layerName){
			if(!layerName){
				return;
			}

			var visibility = this.map.getLayoutProperty(layerName, 'visibility');
			if (visibility === 'visible') {
				this.map.setLayoutProperty(layerName, 'visibility', 'none');
			} else {
				this.map.setLayoutProperty(layerName, 'visibility', 'visible');
			}

			this.setData();
		}
	}
});

//	ajax helper
function get(url, callback) {
	var xhr;

	if(typeof XMLHttpRequest !== 'undefined') {
		xhr = new XMLHttpRequest();
	} 
	else {
		var versions = [
			"MSXML2.XmlHttp.5.0", 
			"MSXML2.XmlHttp.4.0",
			"MSXML2.XmlHttp.3.0", 
			"MSXML2.XmlHttp.2.0",
			"Microsoft.XmlHttp"
		]

		for(var i = 0, len = versions.length; i < len; i++) {
			try {
				xhr = new ActiveXObject(versions[i]);
				break;
			}
			catch(e){}
		}
	}

	xhr.onreadystatechange = ensureReadiness;

	function ensureReadiness() {
		if(xhr.readyState < 4) {
			return;
		}

		if(xhr.status !== 200) {
			return;
		}

		// all is well  
		if(xhr.readyState === 4) {
			callback(xhr);
		}
	}

	xhr.open('GET', url, true);
	xhr.send('');
}