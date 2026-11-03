// React may release/reacquire an effect synchronously in development StrictMode.
// Delay final disposal by one microtask; a live lease prevents disposing the same resource.
const leases=new WeakMap();
export function retainDisposable(resource){
 if(!resource)return ()=>{};
 let lease=leases.get(resource);if(!lease){lease={count:0,disposed:false};leases.set(resource,lease);}
 if(lease.disposed)throw Error('disposedResource');
 lease.count++;let released=false;
 return ()=>{if(released)return;released=true;lease.count--;queueMicrotask(()=>{if(lease.count===0&&!lease.disposed){lease.disposed=true;resource.dispose();}});};
}
