// ToggleTileBoundariesControl is responsible for toggling tile boundary
// debug outlines
export default class ToggleTileBoundariesControl {
  // required to meet the iControl interface
  onAdd(map) {
      this._map = map;
      this._container = document.createElement('div');
      this._container.className = 'mapboxgl-ctrl mapboxgl-ctrl-group';
      this._container.style.color = '#f00';

      // toggle the tile boundaries on / off on click
      this._container.onclick = function(){
        map.showTileBoundaries = !map.showTileBoundaries;
      };

      // build button with grid icon
      let btn = document.createElement('button');
      btn.title = "Toggle tile boundaries";
      btn.className = 'mapboxgl-ctrl-icon mapboxgl-ctrl-toggle-tile-boundaries'
      this._container.appendChild(btn);

      return this._container;
  }

  // required to meet the iControl interface
  onRemove() {
      this._container.parentNode.removeChild(this._container);
      this._map = undefined;
  }
}