export const stateCodes={RO:'11',AC:'12',AM:'13',RR:'14',PA:'15',AP:'16',TO:'17',MA:'21',PI:'22',CE:'23',RN:'24',PB:'25',PE:'26',AL:'27',SE:'28',BA:'29',MG:'31',ES:'32',RJ:'33',SP:'35',PR:'41',SC:'42',RS:'43',MS:'50',MT:'51',GO:'52',DF:'53'};
export function projectCoordinate(longitude,latitude){
 if(!Number.isFinite(longitude)||!Number.isFinite(latitude)||longitude < -180||longitude>180||latitude < -90||latitude>90)throw Error('coordinate');
 return [(longitude+75)*15,(6-latitude)*15];
}
export function municipalities(uf,names,geo){
 const prefix=stateCodes[uf];
 if(!prefix||!Array.isArray(names)||names.length>1500||geo?.type!=='FeatureCollection'||!Array.isArray(geo.features)||geo.features.length>1500)throw Error('geography');
 const lookup=new Map(names.filter(row=>String(row?.id).startsWith(prefix)&&typeof row?.nome==='string').map(row=>[String(row.id),row.nome.slice(0,200)]));
 const result=[],seen=new Set();
 let points=0;
 for(const feature of geo.features){
  const id=String(feature?.properties?.codarea);
  if(!lookup.has(id))continue;
  if(!/^\d{7}$/.test(id)||seen.has(id))throw Error('municipality');
  seen.add(id);
  const geometry=feature.geometry;
  const polygons=geometry?.type==='Polygon'?[geometry.coordinates]:geometry?.type==='MultiPolygon'?geometry.coordinates:null;
  if(!Array.isArray(polygons))throw Error('geometry');
  let x0=Infinity,y0=Infinity,x1=-Infinity,y1=-Infinity;
  const parts=[];
  for(const polygon of polygons){
   if(!Array.isArray(polygon))throw Error('polygon');
   for(const ring of polygon){
    if(!Array.isArray(ring)||ring.length<4)throw Error('ring');
    const path=[];
    for(const point of ring){
     if(!Array.isArray(point)||point.length<2||++points>1500000)throw Error('point');
     const [x,y]=projectCoordinate(point[0],point[1]);
     x0=Math.min(x0,x);x1=Math.max(x1,x);y0=Math.min(y0,y);y1=Math.max(y1,y);
     path.push(x.toFixed(4)+','+y.toFixed(4));
    }
    parts.push('M'+path.join('L')+'Z');
   }
  }
  if(!(x1>x0&&y1>y0))throw Error('bounds');
  result.push({id,uf,name:lookup.get(id),path:parts.join(''),bounds:[x0,y0,x1-x0,y1-y0]});
 }
 if(!result.length)throw Error('empty');
 return result.sort((a,b)=>a.name.localeCompare(b.name,'pt-BR'));
}
export function validCachedRows(uf,rows){
 return !!stateCodes[uf]&&Array.isArray(rows)&&rows.length>0&&rows.length<=1500&&rows.every(row=>row.uf===uf&&typeof row.id==='string'&&row.id.startsWith(stateCodes[uf])&&/^\d{7}$/.test(row.id)&&typeof row.name==='string'&&row.name.length<=200&&typeof row.path==='string'&&row.path.length<=2000000&&/^M[0-9.,LZM\-]+$/.test(row.path)&&Array.isArray(row.bounds)&&row.bounds.length===4&&row.bounds.every(Number.isFinite)&&row.bounds[2]>0&&row.bounds[3]>0);
}
