import {validate80,type Settings80} from './model.mjs';
function database():Promise<IDBDatabase>{
 return new Promise((resolve,reject)=>{const request=indexedDB.open('titan-settings80',1);request.onupgradeneeded=()=>request.result.createObjectStore('preferences');request.onsuccess=()=>resolve(request.result);request.onerror=()=>reject(request.error);});
}
export async function readSettings80(key:string):Promise<Settings80|null>{
 const db=await database();
 return new Promise((resolve,reject)=>{const tx=db.transaction('preferences'),request=tx.objectStore('preferences').get(key);request.onsuccess=()=>{try{resolve(request.result?validate80(request.result):null);}catch{resolve(null);}};request.onerror=()=>reject(request.error);tx.oncomplete=()=>db.close();tx.onabort=()=>db.close();});
}
export async function writeSettings80(key:string,value:Settings80):Promise<void>{
 const data=validate80(value),db=await database();
 return new Promise((resolve,reject)=>{const tx=db.transaction('preferences','readwrite');tx.objectStore('preferences').put(data,key);tx.oncomplete=()=>{db.close();resolve();};tx.onerror=()=>{db.close();reject(tx.error);};tx.onabort=()=>{db.close();reject(tx.error);};});
}
export async function raster80(file:File,background=false):Promise<string>{
 if(!['image/png','image/jpeg','image/webp'].includes(file.type)||file.size>8000000)throw Error('image');
 const bitmap=await createImageBitmap(file);
 try{
  if(bitmap.width*bitmap.height>24000000||!bitmap.width||!bitmap.height)throw Error('size');
  const scale=Math.min(1,(background?1920:512)/Math.max(bitmap.width,bitmap.height));
  const canvas=document.createElement('canvas');canvas.width=Math.max(1,Math.round(bitmap.width*scale));canvas.height=Math.max(1,Math.round(bitmap.height*scale));
  const ctx=canvas.getContext('2d');if(!ctx)throw Error('canvas');
  ctx.drawImage(bitmap,0,0,canvas.width,canvas.height);
  const data=canvas.toDataURL(background?'image/jpeg':'image/webp',.82);
  if(data.length>(background?2800000:700000))throw Error('size');
  return data;
 }finally{bitmap.close();}
}
