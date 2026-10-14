import test from 'node:test';
import assert from 'node:assert/strict';
import {projectCoordinate,municipalities,validCachedRows} from '../src/core/map79/geography.mjs';
const names=[{id:3500001,nome:'Município de teste'}];
const ring=[[-48,-21],[-47,-21],[-47,-20],[-48,-21]];
const feature=(coordinates)=>({type:'FeatureCollection',features:[{type:'Feature',properties:{codarea:'3500001'},geometry:{type:'Polygon',coordinates}}]});
test('polígonos usam a mesma projeção do mapa regional e preservam o código municipal',()=>{
 assert.deepEqual(projectCoordinate(-48,-21),[405,405]);
 const rows=municipalities('SP',names,feature([ring]));
 assert.equal(rows[0].id,'3500001');
 assert.equal(rows[0].name,names[0].nome);
 assert.equal(rows[0].uf,'SP');
 assert.deepEqual(rows[0].bounds,[405,390,15,15]);
 assert.equal(validCachedRows('SP',rows),true);
 assert.equal(validCachedRows('RJ',rows),false);
});
test('coordenadas inválidas, UF desconhecida e geometrias vazias são recusadas',()=>{
 assert.throws(()=>projectCoordinate(NaN,0));
 assert.throws(()=>municipalities('XX',names,feature([ring])));
 assert.throws(()=>municipalities('SP',names,feature([[[200,-21],...ring.slice(1)]])));
 assert.throws(()=>municipalities('SP',[],feature([ring])));
});
test('nomes vêm da fonte de localidades e código de outro estado não é misturado',()=>{
 const response=feature([ring]);
 response.features.push({...response.features[0],properties:{codarea:'3300001'}});
 const rows=municipalities('SP',names,response);
 assert.equal(rows.length,1);
 assert.equal(validCachedRows('SP',[{...rows[0],path:'<script>'}]),false);
});
