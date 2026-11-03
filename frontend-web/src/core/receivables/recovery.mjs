// Only scoped operation identifiers are durable. Never persist commands, amounts,
// customer text, tokens, passwords or session objects. A restored pointer is GET-only.
const validID=v=>typeof v==='string'&&/^[A-Za-z0-9][A-Za-z0-9_-]{0,127}$/.test(v);
function pointer(value){
 if(!value||!validID(value.op)||!validID(value.id)||!['create','receive_cash','cancel','reverse_cash'].includes(value.action))throw Error('storage');
 return {op:value.op,id:value.id,action:value.action};
}
function same(a,b){return a&&a.op===b.op&&a.id===b.id&&a.action===b.action;}
export function createRecoveryJournal(factory=globalThis.indexedDB){
 // Node integrations inject storage or have no window. Browsers fail closed.
 if(!factory&&typeof window==='undefined')return null;
 async function database(){return new Promise((resolve,reject)=>{
  if(!factory){reject(Error('storage'));return;}
  const q=factory.open('titan-receivables-recovery',1);
  q.onupgradeneeded=()=>q.result.createObjectStore('pointers');
  q.onerror=q.onblocked=()=>reject(Error('storage'));
  q.onsuccess=()=>resolve(q.result);
 });}
 async function transaction(scope,mode,work){
  if(typeof scope!=='string'||scope.length>2048)throw Error('storage');
  const db=await database();
  return new Promise((resolve,reject)=>{
   const tx=db.transaction('pointers',mode),store=tx.objectStore('pointers');let value;
   const q=store.get(scope);
   q.onsuccess=()=>{try{value=work(q.result?pointer(q.result):null,store);}catch{tx.abort();}};
   tx.oncomplete=()=>{db.close();resolve(value);};
   tx.onerror=tx.onabort=()=>{db.close();reject(Error('storage'));};
  });
 }
 return {
  read:scope=>transaction(scope,'readonly',old=>old),
  reserve:(scope,command)=>{const value=pointer(command);return transaction(scope,'readwrite',(old,store)=>{if(old)return false;store.add(value,scope);return true;});},
  remove:(scope,command)=>transaction(scope,'readwrite',(old,store)=>{if(old&&!same(old,command))throw Error('storage');if(old)store.delete(scope);})
 };
}
