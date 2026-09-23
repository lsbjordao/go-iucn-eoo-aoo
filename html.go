package eooaoo

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const mapLibreVersion = "6.10.0"

// WriteMapHTML creates a single HTML report with the GeoJSON data embedded in
// the document. MapLibre GL JS and the demonstration basemap are loaded from
// the network when the file is opened; occurrence/EOO/AOO data stay embedded.
func WriteMapHTML(path, geojsonDir string, r Result) error {
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		return fmt.Errorf("HTML output already exists or is inaccessible: %s", path)
	}

	readLayer := func(name string) ([]byte, error) {
		b, err := os.ReadFile(filepath.Join(geojsonDir, name+".geojson"))
		if err != nil {
			return nil, fmt.Errorf("read %s GeoJSON: %w", name, err)
		}
		if !json.Valid(b) {
			return nil, fmt.Errorf("%s GeoJSON is invalid", name)
		}
		return b, nil
	}
	occ, err := readLayer("occurrences")
	if err != nil {
		return err
	}
	aoo, err := readLayer("aoo")
	if err != nil {
		return err
	}
	eoo, err := readLayer("eoo")
	if err != nil {
		return err
	}

	metadata := map[string]any{
		"taxon":                r.Taxon,
		"software_version":     r.SoftwareVersion,
		"gdal_version":         r.GDALVersion,
		"proj_version":         r.PROJVersion,
		"projection_strategy":  r.Projection.Strategy,
		"projection_reference": r.Projection.Reference,
		"input_records":        r.InputRecords,
		"unique_coordinates":   r.UniqueCoordinates,
		"eoo_raw_km2":          r.EOO.RawAreaKM2,
		"eoo_assessment_km2":   r.EOO.AssessmentAreaKM2,
		"aoo_km2":              r.AOO.AreaKM2,
		"aoo_occupied_cells":   r.AOO.OccupiedCells,
		"aoo_cell_size_m":      r.AOO.CellSizeM,
		"grid_mode":            r.AOO.GridMode,
		"input_sha256":         r.InputSHA256,
	}
	metaJSON, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	occJS, _ := json.Marshal(string(occ))
	aooJS, _ := json.Marshal(string(aoo))
	eooJS, _ := json.Marshal(string(eoo))

	html := strings.NewReplacer(
		"__MAPLIBRE_VERSION__", mapLibreVersion,
		"__META__", string(metaJSON),
		"__OCC__", string(occJS),
		"__AOO__", string(aooJS),
		"__EOO__", string(eooJS),
	).Replace(mapHTMLTemplate)

	return os.WriteFile(path, []byte(html), 0644)
}

const mapHTMLTemplate = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>EOO/AOO map</title>
<link rel="stylesheet" href="https://unpkg.com/maplibre-gl@__MAPLIBRE_VERSION__/dist/maplibre-gl.css">
<style>
html,body{height:100%;margin:0;font-family:system-ui,-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif;background:#f7f7f7;color:#171717}
#map{position:absolute;inset:0}
.panel{position:absolute;z-index:2;top:14px;left:14px;width:min(360px,calc(100vw - 28px));max-height:calc(100vh - 28px);overflow:auto;background:rgba(255,255,255,.96);border:1px solid rgba(0,0,0,.12);border-radius:12px;box-shadow:0 6px 24px rgba(0,0,0,.14);padding:14px;box-sizing:border-box}
h1{font-size:18px;margin:0 0 4px}.sub{font-size:12px;color:#555;margin-bottom:12px}.layers{display:grid;grid-template-columns:repeat(3,1fr);gap:6px;margin:10px 0 12px}.layers label{font-size:13px;display:flex;align-items:center;gap:5px}.metrics{display:grid;grid-template-columns:1fr 1fr;gap:8px}.metric{border:1px solid #ddd;border-radius:8px;padding:8px}.metric b{display:block;font-size:15px}.metric span{font-size:11px;color:#666}.details{font-size:12px;line-height:1.45;margin-top:10px}.details code{font-size:11px;word-break:break-all}.note{font-size:11px;color:#666;margin-top:10px}.maplibregl-popup-content{font:12px/1.4 system-ui,sans-serif}
@media (max-width:640px){.panel{top:8px;left:8px;width:calc(100vw - 16px);max-height:46vh}}
</style>
</head>
<body>
<div id="map"></div>
<section class="panel">
  <h1 id="title">EOO / AOO</h1>
  <div class="sub" id="projection"></div>
  <div class="layers">
    <label><input type="checkbox" data-layers="eoo-fill,eoo-line" checked> EOO</label>
    <label><input type="checkbox" data-layers="aoo-fill,aoo-line" checked> AOO</label>
    <label><input type="checkbox" data-layers="occurrences" checked> Occurrences</label>
  </div>
  <div class="metrics">
    <div class="metric"><b id="eooMetric">—</b><span>Assessment EOO</span></div>
    <div class="metric"><b id="aooMetric">—</b><span>AOO</span></div>
  </div>
  <div class="details" id="details"></div>
  <div class="note">Spatial assessment data are embedded in this HTML file. MapLibre GL JS and the basemap require an internet connection.</div>
</section>
<script type="module">
import * as maplibregl from 'https://unpkg.com/maplibre-gl@__MAPLIBRE_VERSION__/dist/maplibre-gl.mjs';

const metadata = __META__;
const occurrences = JSON.parse(__OCC__);
const aoo = JSON.parse(__AOO__);
const eoo = JSON.parse(__EOO__);
const km2 = v => (v === null || v === undefined) ? '—' : Number(v).toLocaleString('en-US',{maximumFractionDigits:3}) + ' km²';

document.getElementById('title').textContent = metadata.taxon || 'EOO / AOO';
document.getElementById('projection').textContent = metadata.projection_strategy + ' · ' + metadata.projection_reference;
document.getElementById('eooMetric').textContent = km2(metadata.eoo_assessment_km2);
document.getElementById('aooMetric').textContent = km2(metadata.aoo_km2);
document.getElementById('details').innerHTML =
  '<b>Raw EOO:</b> ' + km2(metadata.eoo_raw_km2) + '<br>' +
  '<b>AOO cells:</b> ' + metadata.aoo_occupied_cells + ' × ' + metadata.aoo_cell_size_m + ' m<br>' +
  '<b>Occurrences:</b> ' + metadata.input_records + ' records; ' + metadata.unique_coordinates + ' unique coordinates<br>' +
  '<b>Grid:</b> ' + metadata.grid_mode + '<br>' +
  '<b>GDAL / PROJ:</b> ' + metadata.gdal_version + ' / ' + metadata.proj_version + '<br>' +
  '<b>SHA-256:</b> <code>' + metadata.input_sha256 + '</code>';

const map = new maplibregl.Map({
  container: 'map',
  style: 'https://demotiles.maplibre.org/style.json',
  center: [0,0],
  zoom: 1,
  maplibreLogo: true
});
map.addControl(new maplibregl.NavigationControl(), 'bottom-right');
map.addControl(new maplibregl.ScaleControl({unit:'metric'}), 'bottom-left');

function extendBounds(bounds, value) {
  if (!Array.isArray(value)) return;
  if (value.length >= 2 && typeof value[0] === 'number' && typeof value[1] === 'number') {
    bounds.extend([value[0], value[1]]);
    return;
  }
  for (const child of value) extendBounds(bounds, child);
}
function includeCollection(bounds, fc) {
  for (const f of (fc.features || [])) if (f.geometry) extendBounds(bounds, f.geometry.coordinates);
}

map.on('load', () => {
  map.addSource('eoo', {type:'geojson', data:eoo});
  map.addSource('aoo', {type:'geojson', data:aoo});
  map.addSource('occurrences', {type:'geojson', data:occurrences});

  map.addLayer({id:'eoo-fill',type:'fill',source:'eoo',paint:{'fill-color':'#e63946','fill-opacity':0.13}});
  map.addLayer({id:'eoo-line',type:'line',source:'eoo',paint:{'line-color':'#b91c2b','line-width':3}});
  map.addLayer({id:'aoo-fill',type:'fill',source:'aoo',paint:{'fill-color':'#2563eb','fill-opacity':0.18}});
  map.addLayer({id:'aoo-line',type:'line',source:'aoo',paint:{'line-color':'#1d4ed8','line-width':1.2}});
  map.addLayer({id:'occurrences',type:'circle',source:'occurrences',paint:{'circle-radius':5,'circle-color':'#111827','circle-stroke-color':'#fff','circle-stroke-width':1.5}});

  const bounds = new maplibregl.LngLatBounds();
  includeCollection(bounds, eoo);
  includeCollection(bounds, aoo);
  includeCollection(bounds, occurrences);
  if (!bounds.isEmpty()) {
    const padding = window.innerWidth <= 640 ? 40 : {top:90,bottom:55,left:390,right:55};
    map.fitBounds(bounds, {padding,maxZoom:11,duration:0});
  }

  document.querySelectorAll('[data-layers]').forEach(control => {
    control.addEventListener('change', () => {
      const visibility = control.checked ? 'visible' : 'none';
      for (const id of control.dataset.layers.split(',')) map.setLayoutProperty(id, 'visibility', visibility);
    });
  });

  map.on('click','occurrences', e => {
    const f = e.features && e.features[0];
    if (!f) return;
    const p = f.properties || {};
    const node = document.createElement('div');
    const heading = document.createElement('b');
    heading.textContent = 'Occurrence';
    node.appendChild(heading);
    const lines = [];
    if (p.taxon) lines.push(String(p.taxon));
    lines.push('records at this coordinate: ' + (p.records ?? '—'));
    lines.push('input indices: ' + (p.input_indices ?? '—'));
    for (const line of lines) {
      node.appendChild(document.createElement('br'));
      node.appendChild(document.createTextNode(line));
    }
    new maplibregl.Popup().setLngLat(e.lngLat).setDOMContent(node).addTo(map);
  });
  map.on('mouseenter','occurrences',()=>map.getCanvas().style.cursor='pointer');
  map.on('mouseleave','occurrences',()=>map.getCanvas().style.cursor='');
});
</script>
</body>
</html>
`
