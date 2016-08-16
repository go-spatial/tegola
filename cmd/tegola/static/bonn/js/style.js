function createMapboxStreetsV6Style() {
  var fill = new ol.style.Fill({color: ''});
  var stroke = new ol.style.Stroke({color: '', width: 1});
  var polygon = new ol.style.Style({fill: fill});
  var strokedPolygon = new ol.style.Style({fill: fill, stroke: stroke});
  var line = new ol.style.Style({stroke: stroke});
  var text = new ol.style.Style({text: new ol.style.Text({
    text: '', fill: fill, stroke: stroke
  })});
  var iconCache = {};
  function getIcon(iconName) {
    var icon = iconCache[iconName];
    if (!icon) {
      icon = new ol.style.Style({image: new ol.style.Icon({
        src: 'https://cdn.rawgit.com/mapbox/maki/master/icons/' + iconName + '-15.svg',
        imgSize: [15, 15]
      })});
      iconCache[iconName] = icon;
    }
    return icon;
  }
  var styles = [];
  return function(feature, resolution) {
    var length = 0;
    var layer = feature.get('layer');
    var cls = feature.get('class');
    var type = feature.get('type');
    var scalerank = feature.get('scalerank');
    var labelrank = feature.get('labelrank');
    var adminLevel = feature.get('admin_level');
    var maritime = feature.get('maritime');
    var disputed = feature.get('disputed');
    var maki = feature.get('maki');
    var geom = feature.getGeometry().getType();

  //  tegola debug styles
  if (layer == 'debug' && type == 'debug_outline'){
    //  outline our tile
    stroke.setColor('#f00');
    stroke.setWidth(1);
    styles[length++] = line;
  } else if (layer == 'debug' && type == 'debug_text'){
    //  write z, x, y values
    text.getText().setText(feature.get('name_en'));
    text.getText().setFont('11px "Open Sans", "Arial Unicode MS"');
    fill.setColor('#333');
    stroke.setColor('rgba(255,255,255,0.8)');
    stroke.setWidth(1);
    styles[length++] = text;
  }

    //  default mapbox v6 open streets styles
    if (layer == 'landuse' && cls == 'park') {
      fill.setColor('#d8e8c8');
      styles[length++] = polygon;
    } else if (layer == 'landuse' && cls == 'cemetery') {
      fill.setColor('#e0e4dd');
      styles[length++] = polygon;
    } else if (layer == 'landuse' && cls == 'hospital') {
      fill.setColor('#fde');
      styles[length++] = polygon;
    } else if (layer == 'landuse' && cls == 'school') {
      fill.setColor('#f0e8f8');
      styles[length++] = polygon;
    } else if (layer == 'landuse' && cls == 'wood') {
      fill.setColor('rgb(233,238,223)');
      styles[length++] = polygon;
    } else if (layer == 'river' ) {
      stroke.setColor('#0000ff');
      stroke.setWidth(1);
      styles[length++] = line;
    } else if (layer == 'waterway' && cls == 'river') {
      stroke.setColor('#a0c8f0');
      stroke.setWidth(1);
      styles[length++] = line;
    } else if (layer == 'waterway' && (cls == 'stream' ||
        cls == 'canal')) {
      stroke.setColor('#a0c8f0');
      stroke.setWidth(1);
      styles[length++] = line;
    } else if (layer == 'water') {
      fill.setColor('#a0c8f0');
      styles[length++] = polygon;
    } else if (layer == 'aeroway' && geom == 'Polygon') {
      fill.setColor('rgb(242,239,235)');
      styles[length++] = polygon;
    } else if (layer == 'aeroway' && geom == 'LineString' &&
        resolution <= 76.43702828517625) {
      stroke.setColor('#f0ede9');
      stroke.setWidth(1);
      styles[length++] = line;
    } else if (layer == 'building') {
      fill.setColor('#f2eae2');
      stroke.setColor('#dfdbd7');
      stroke.setWidth(1);
      styles[length++] = strokedPolygon;
    } else if (layer == 'farms') {
      fill.setColor('#00ff00');
      stroke.setColor('#000000');
      stroke.setWidth(0.3);
      styles[length++] = strokedPolygon;
    } else if (layer == 'aerodromes_polygon') {
      fill.setColor('#00ffff');
      stroke.setColor('#000000');
      stroke.setWidth(0.3);
      styles[length++] = strokedPolygon;
    } else if (layer == 'forest') {
      fill.setColor('#266A2E');
      stroke.setColor('#000000');
      stroke.setWidth(0.3);
      styles[length++] = strokedPolygon;
    } else if (layer == 'grassland') {
      fill.setColor('#b0b389');
      stroke.setColor('#000000');
      stroke.setWidth(0.3);
      styles[length++] = strokedPolygon;
    } else if (layer == 'lakes') {
      fill.setColor('#6878c9');
      stroke.setColor('#000000');
      stroke.setWidth(0.3);
      styles[length++] = strokedPolygon;
    } else if (layer == 'medical_polygon') {
      fill.setColor('#CCCCCC');
      stroke.setColor('#000000');
      stroke.setWidth(0.3);
      styles[length++] = strokedPolygon;
    } else if (layer == 'military') {
      fill.setColor('#ff0000');
      stroke.setColor('#000000');
      stroke.setWidth(0.3);
      styles[length++] = strokedPolygon;
    } else if (layer == 'schools_polygon') {
      fill.setColor('#ffff00');
      stroke.setColor('#000000');
      stroke.setWidth(0.3);
      styles[length++] = strokedPolygon;
    } else if (layer == 'road') {
      stroke.setColor('#00ff00');
      stroke.setWidth(0.5);
      styles[length++] = line;
    } else if (layer == 'main_roads') {
      stroke.setColor('#000000');
      stroke.setWidth(1);
      styles[length++] = line;
    } else if (layer == 'country_label' && scalerank === 1) {
      text.getText().setText(feature.get('name_en'));
      text.getText().setFont('bold 11px "Open Sans", "Arial Unicode MS"');
      fill.setColor('#334');
      stroke.setColor('rgba(255,255,255,0.8)');
      stroke.setWidth(2);
      styles[length++] = text;
    } else if (layer == 'country_label' && scalerank === 2 &&
        resolution <= 19567.87924100512) {
      text.getText().setText(feature.get('name_en'));
      text.getText().setFont('bold 10px "Open Sans", "Arial Unicode MS"');
      fill.setColor('#334');
      stroke.setColor('rgba(255,255,255,0.8)');
      stroke.setWidth(2);
      styles[length++] = text;
    } else if (layer == 'country_label' && scalerank === 3 &&
        resolution <= 9783.93962050256) {
      text.getText().setText(feature.get('name_en'));
      text.getText().setFont('bold 9px "Open Sans", "Arial Unicode MS"');
      fill.setColor('#334');
      stroke.setColor('rgba(255,255,255,0.8)');
      stroke.setWidth(2);
      styles[length++] = text;
    } else if (layer == 'country_label' && scalerank === 4 &&
        resolution <= 4891.96981025128) {
      text.getText().setText(feature.get('name_en'));
      text.getText().setFont('bold 8px "Open Sans", "Arial Unicode MS"');
      fill.setColor('#334');
      stroke.setColor('rgba(255,255,255,0.8)');
      stroke.setWidth(2);
      styles[length++] = text;
    } else if (layer == 'marine_label' && labelrank === 1 &&
        geom == 'Point') {
      text.getText().setText(feature.get('name_en'));
      text.getText().setFont(
          'italic 11px "Open Sans", "Arial Unicode MS"');
      fill.setColor('#74aee9');
      stroke.setColor('rgba(255,255,255,0.8)');
      stroke.setWidth(1);
      styles[length++] = text;
    } else if (layer == 'marine_label' && labelrank === 2 &&
        geom == 'Point') {
      text.getText().setText(feature.get('name_en'));
      text.getText().setFont(
          'italic 11px "Open Sans", "Arial Unicode MS"');
      fill.setColor('#74aee9');
      stroke.setColor('rgba(255,255,255,0.8)');
      stroke.setWidth(1);
      styles[length++] = text;
    } else if (layer == 'marine_label' && labelrank === 3 &&
        geom == 'Point') {
      text.getText().setText(feature.get('name_en'));
      text.getText().setFont(
          'italic 10px "Open Sans", "Arial Unicode MS"');
      fill.setColor('#74aee9');
      stroke.setColor('rgba(255,255,255,0.8)');
      stroke.setWidth(1);
      styles[length++] = text;
    } else if (layer == 'marine_label' && labelrank === 4 &&
        geom == 'Point') {
      text.getText().setText(feature.get('name_en'));
      text.getText().setFont(
          'italic 9px "Open Sans", "Arial Unicode MS"');
      fill.setColor('#74aee9');
      stroke.setColor('rgba(255,255,255,0.8)');
      stroke.setWidth(1);
      styles[length++] = text;
    } else if (layer == 'place_label' && type == 'city' &&
        resolution <= 1222.99245256282) {
      text.getText().setText(feature.get('name_en'));
      text.getText().setFont('11px "Open Sans", "Arial Unicode MS"');
      fill.setColor('#333');
      stroke.setColor('rgba(255,255,255,0.8)');
      stroke.setWidth(1);
      styles[length++] = text;
    } else if (layer == 'place_label' && type == 'town' &&
        resolution <= 305.748113140705) {
      text.getText().setText(feature.get('name_en'));
      text.getText().setFont('9px "Open Sans", "Arial Unicode MS"');
      fill.setColor('#333');
      stroke.setColor('rgba(255,255,255,0.8)');
      stroke.setWidth(1);
      styles[length++] = text;
    } else if (layer == 'place_label' && type == 'village' &&
        resolution <= 38.21851414258813) {
      text.getText().setText(feature.get('name_en'));
      text.getText().setFont('8px "Open Sans", "Arial Unicode MS"');
      fill.setColor('#333');
      stroke.setColor('rgba(255,255,255,0.8)');
      stroke.setWidth(1);
      styles[length++] = text;
    } else if (layer == 'place_label' &&
        resolution <= 19.109257071294063 && (type == 'hamlet' ||
        type == 'suburb' || type == 'neighbourhood')) {
      text.getText().setText(feature.get('name_en'));
      text.getText().setFont('bold 9px "Arial Narrow"');
      fill.setColor('#633');
      stroke.setColor('rgba(255,255,255,0.8)');
      stroke.setWidth(1);
      styles[length++] = text;
    } else if (layer == 'poi_label' && resolution <= 19.109257071294063 &&
        scalerank == 1 && maki !== 'marker') {
      styles[length++] = getIcon(maki);
    } else if (layer == 'poi_label' && resolution <= 9.554628535647032 &&
        scalerank == 2 && maki !== 'marker') {
      styles[length++] = getIcon(maki);
    } else if (layer == 'poi_label' && resolution <= 4.777314267823516 &&
        scalerank == 3 && maki !== 'marker') {
      styles[length++] = getIcon(maki);
    } else if (layer == 'poi_label' && resolution <= 2.388657133911758 &&
        scalerank == 4 && maki !== 'marker') {
      styles[length++] = getIcon(maki);
    } else if (layer == 'poi_label' && resolution <= 1.194328566955879 &&
        scalerank >= 5 && maki !== 'marker') {
      styles[length++] = getIcon(maki);
    }
    styles.length = length;
    return styles;
  };
}