import type {Profile86} from './types';
export const activities87=['market','pharmacy','hotdogBusiness','restaurant','industry','other'] as const;
export const sectors87=['warehouse','checkout','bakery','butchery','pickingSector','counterSector','kitchenSector','manufacturing','dispatchSector','otherSector'];
export function recommended87(activity:string):Profile86 {
 const food=['hotdogBusiness','restaurant'].includes(activity),production=activity==='industry'||food;
 return {configured:true,activity,sectors:activity==='market'?['warehouse','checkout']:activity==='pharmacy'?['counterSector','pickingSector','dispatchSector']:food?['kitchenSector','counterSector','dispatchSector']:activity==='industry'?['warehouse','manufacturing','dispatchSector']:[],menu:food,kitchen:food,tables:activity==='restaurant',production};
}
export function validProfile87(p:unknown):p is Profile86 {
 if(!p||typeof p!=='object')return false;
 const v=p as Profile86;
 return v.configured===true&&activities87.includes(v.activity as typeof activities87[number])&&Array.isArray(v.sectors)&&v.sectors.length<=sectors87.length&&new Set(v.sectors).size===v.sectors.length&&v.sectors.every(s=>sectors87.includes(s))&&['menu','kitchen','tables','production'].every(k=>typeof v[k as keyof Profile86]==='boolean')&&(!v.kitchen||v.menu);
}
// Company/store preferences contain no employees, salary or commercial records.
// Permissions always come from the authenticated session, never this profile.
export function profileKey87(scope:string){const ids:unknown=JSON.parse(scope);if(!Array.isArray(ids)||ids.length!==4||ids.some(v=>typeof v!=='string'||!v.trim()||v.length>128))throw Error('context');return JSON.stringify(ids.slice(0,2));}
type Record87={profile:Profile86;revision:number};
function open87():Promise<IDBDatabase>{return new Promise((resolve,reject)=>{const r=indexedDB.open('titan-company-profile87',1);r.onupgradeneeded=()=>r.result.createObjectStore('profiles');r.onerror=()=>reject(r.error);r.onsuccess=()=>resolve(r.result);});}
export async function readProfile87(scope:string):Promise<Record87|null>{const key=profileKey87(scope),db=await open87();return new Promise((resolve,reject)=>{const tx=db.transaction('profiles'),r=tx.objectStore('profiles').get(key);let value:Record87|null=null;r.onsuccess=()=>{if(r.result){if(!validProfile87(r.result.profile)||!Number.isSafeInteger(r.result.revision)){tx.abort();return;}value=r.result;}};tx.oncomplete=()=>{db.close();resolve(value);};tx.onabort=tx.onerror=()=>{db.close();reject(Error('saveFailed'));};});}
export async function writeProfile87(scope:string,profile:Profile86,revision:number){if(!validProfile87(profile))throw Error('invalid');const key=profileKey87(scope),db=await open87();await new Promise<void>((resolve,reject)=>{const tx=db.transaction('profiles','readwrite'),store=tx.objectStore('profiles'),r=store.get(key);r.onsuccess=()=>{if((r.result?.revision||0)!==revision){tx.abort();return;}store.put({profile:structuredClone(profile),revision:revision+1},key);};tx.oncomplete=()=>{db.close();resolve();};tx.onabort=tx.onerror=()=>{db.close();reject(Error('profileConflict87'));};});window.dispatchEvent(new CustomEvent('titan-profile87',{detail:key}));}
export function proposal87(profile?:Profile86):Profile86|null{try{if(profile){if(!validProfile87(profile))throw Error('invalid');sessionStorage.setItem('titan-profile-proposal87',JSON.stringify(profile));return profile;}const p=JSON.parse(sessionStorage.getItem('titan-profile-proposal87')||'null');return validProfile87(p)?p:null;}catch{return null;}}
