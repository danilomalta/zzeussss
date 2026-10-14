import {municipalities,stateCodes,validCachedRows,type Municipality79} from './geography.mjs';
interface PublicCache{uf:string;updated:number;rows:Municipality79[]}
function database():Promise<IDBDatabase>{
 return new Promise((resolve,reject)=>{
  const request=indexedDB.open('titan-public-geography79',1);
  request.onupgradeneeded=()=>request.result.createObjectStore('municipalities',{keyPath:'uf'});
  request.onsuccess=()=>resolve(request.result);request.onerror=()=>reject(request.error);
 });
}
async function cached(uf:string):Promise<PublicCache|null>{
 try{
  const db=await database();
  return await new Promise((resolve,reject)=>{const tx=db.transaction('municipalities'),r=tx.objectStore('municipalities').get(uf);r.onsuccess=()=>resolve(r.result&&validCachedRows(uf,r.result.rows)?r.result:null);r.onerror=()=>reject(r.error);tx.oncomplete=()=>db.close();tx.onabort=()=>db.close();});
 }catch{return null;}
}
async function save(value:PublicCache){
 try{const db=await database();await new Promise<void>((resolve,reject)=>{const tx=db.transaction('municipalities','readwrite');tx.objectStore('municipalities').put(value);tx.oncomplete=()=>{db.close();resolve();};tx.onerror=()=>{db.close();reject(tx.error);};});}catch{/* Public geography cache is optional. */}
}
async function json(url:string,signal:AbortSignal){
 const response=await fetch(url,{signal,credentials:'omit',referrerPolicy:'no-referrer'});
 if(!response.ok)throw Error('geography');
 const length=Number(response.headers.get('content-length'));
 if(length>20000000)throw Error('size');
 const text=await response.text();
 if(text.length>20000000)throw Error('size');
 return JSON.parse(text);
}
export async function loadMunicipalities(uf:string,signal:AbortSignal):Promise<{rows:Municipality79[];cached:boolean}>{
 if(!stateCodes[uf])throw Error('state');
 const previous=await cached(uf);
 if(signal.aborted)throw new DOMException('Aborted','AbortError');
 if(previous&&Date.now()-previous.updated<30*86400000)return {rows:previous.rows,cached:true};
 try{
  const namesURL='https://servicodados.ibge.gov.br/api/v1/localidades/estados/'+stateCodes[uf]+'/municipios';
  const shapeURL='https://servicodados.ibge.gov.br/api/v3/malhas/estados/'+stateCodes[uf]+'?formato=application%2Fvnd.geo%2Bjson&qualidade=minima&intrarregiao=municipio';
  const [names,geo]=await Promise.all([json(namesURL,signal),json(shapeURL,signal)]);
  const rows=municipalities(uf,names,geo);
  await save({uf,updated:Date.now(),rows});
  return {rows,cached:false};
 }catch(error){if(signal.aborted)throw error;if(previous)return {rows:previous.rows,cached:true};throw error;}
}
