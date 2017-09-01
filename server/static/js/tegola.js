'use strict';

var app = new Vue({
	el: '#app',
	//	map reference
	map: null,
	//	tooltip for displaying properties during inspect mode
	inspector: null,
	data: {
		//	stored reference from the capabilities endpoint response
		capabilities: {
			version: null,
			maps: []
		},
		hideMapsList: false,
		inspectorIsActive: false,
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
		],
		hoverLayers: []
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
			me.map.on('zoomend', me.setData);

			me.inspector = new mapboxgl.Popup();
		},
		//	show / hide the maps list
		toggleMapsVisibility: function(){
			var hidden = this.$data.hideMapsList;
			if (hidden){
				this.$data.hideMapsList = false;
			} else {
				this.$data.hideMapsList = true;
			}
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
		},
		toggleFeatureInspector: function() {
			var me = this;

			if (!me.$data.inspectorIsActive){
				me.map.on('mousemove', me.inspectFeatures);
				me.$data.inspectorIsActive = true;
			} else {
				me.map.off('mousemove', me.inspectFeatures);
				me.$data.inspectorIsActive = false;
				if (me.inspector.isOpen()){
					me.inspector.remove();
				}
			}
		},
		inspectFeatures: function(e){
			var me = this;
			var html = '';
			var bbox = {
				width: 10,
				height: 10
			}

			//	query within a few pixels of the mouse to give us some tolerance to work with
			var features = me.map.queryRenderedFeatures([
				[e.point.x - bbox.width / 2, e.point.y - bbox.height / 2],
				[e.point.x + bbox.width / 2, e.point.y + bbox.height / 2]
			]);

			for (var i=0, l=features.length; i<l; i++){
				html += '<h4>'+features[i].layer.id+'</h4>';

				html += '<ul>';
				html += '<li>feature id <span class="float-r">'+features[i].id+'</span></li>'
				for (var key in features[i].properties){
					html += '<li>'+key+'<span class="float-r">'+features[i].properties[key]+'</span></li>';
				}
				html += '</ul>';
			}

			if (html != '') {
				me.inspector.setLngLat(e.lngLat)
					.setHTML(html)
					.addTo(me.map);					
			} else {
				if (me.inspector.isOpen()){
					me.inspector.remove();
				}
			}
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