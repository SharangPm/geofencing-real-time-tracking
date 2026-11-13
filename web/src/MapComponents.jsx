import React,{useEffect}from'react';
import L from'leaflet';
import{MapContainer,TileLayer,Polygon,Marker,Popup,useMap,useMapEvents}from'react-leaflet';
import'leaflet/dist/leaflet.css';
import'leaflet-draw';
import'leaflet-draw/dist/leaflet.draw.css';

const colors={restricted_zone:'#e35462',delivery_zone:'#3974d5',toll_zone:'#ae78dd',customer_area:'#2ca985'};
const vehicleIcon=L.divIcon({className:'vehicle-pin',html:'<div>●</div>',iconSize:[26,26],iconAnchor:[13,13]});
function MapClick({onPick}){useMapEvents({click:e=>onPick?.(e.latlng)});return null}
export function MapView({geofences=[],vehicles=[],height='70vh',onPick,onVehicle,locationSelectionActive=false}){
 const first=vehicles.find(v=>v.latitude!=null&&v.longitude!=null),center=first?[first.latitude,first.longitude]:[37.7739,-122.4312];
 return <MapContainer center={center} zoom={12} style={{height,width:'100%'}} scrollWheelZoom><TileLayer attribution='&copy; OpenStreetMap' url="https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png"/>{geofences.map(g=><Polygon key={g.id} positions={g.coordinates} pathOptions={{color:colors[g.category]||'#3974d5',fillOpacity:.13,weight:2,bubblingMouseEvents:!locationSelectionActive}} eventHandlers={locationSelectionActive?{click:e=>onPick?.(e.latlng)}:undefined}>{!locationSelectionActive&&<Popup><b>{g.name}</b><br/>{g.category?.replaceAll('_',' ')}</Popup>}</Polygon>)}{vehicles.filter(v=>v.latitude!=null&&v.longitude!=null).map(v=><Marker key={v.id} position={[v.latitude,v.longitude]} icon={vehicleIcon} eventHandlers={{click:()=>onVehicle?.(v)}}><Popup><b>{v.vehicle_number}</b><br/>{v.driver_name}<br/>{v.latitude.toFixed(5)}, {v.longitude.toFixed(5)}</Popup></Marker>)}{onPick&&<MapClick onPick={onPick}/>}</MapContainer>
}
function DrawControl({onChange,clearKey}){
 const map=useMap();
 useEffect(()=>{
  const group=new L.FeatureGroup();map.addLayer(group);
  // leaflet-draw 1.0.4 assigns an undeclared `type` in readableArea when
  // showArea is enabled, which throws in the browser's strict module mode.
  const control=new L.Control.Draw({draw:{polygon:{allowIntersection:false,shapeOptions:{color:'#3974d5'}},polyline:false,rectangle:false,circle:false,circlemarker:false,marker:false},edit:{featureGroup:group,remove:true}});map.addControl(control);
  const emit=()=>{const layer=group.getLayers()[0];if(!layer){onChange([]);return}let coords=layer.getLatLngs()[0].map(p=>[p.lat,p.lng]);if(coords.length&& (coords[0][0]!==coords.at(-1)[0]||coords[0][1]!==coords.at(-1)[1]))coords.push([...coords[0]]);onChange(coords)};
  const created=e=>{group.clearLayers();group.addLayer(e.layer);emit()};
  map.on(L.Draw.Event.CREATED,created);map.on(L.Draw.Event.EDITED,emit);map.on(L.Draw.Event.DELETED,emit);
  return()=>{map.off(L.Draw.Event.CREATED,created);map.off(L.Draw.Event.EDITED,emit);map.off(L.Draw.Event.DELETED,emit);map.removeControl(control);map.removeLayer(group)};
 },[map,onChange,clearKey]);
 return null;
}
export function GeofenceMap({geofences,onDraw,clearKey}){
 return <MapContainer center={[37.7739,-122.4312]} zoom={12} style={{height:420,width:'100%'}}><TileLayer attribution='&copy; OpenStreetMap' url="https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png"/><DrawControl onChange={onDraw} clearKey={clearKey}/>{geofences.map(g=><Polygon key={g.id} positions={g.coordinates} pathOptions={{color:colors[g.category]||'#3974d5',fillOpacity:.12}}><Popup>{g.name}</Popup></Polygon>)}</MapContainer>
}
