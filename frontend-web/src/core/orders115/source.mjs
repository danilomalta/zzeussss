import {createRestockPreview} from '../restockPreview/source.mjs';
import {createPurchaseClient,pendingPurchaseKey,readPurchasePending,persistPurchasePending,resolvePurchasePending,validPurchaseInput,validSupplierInput} from '../local/purchaseClient.mjs';
import {createReplenishmentClient,restockKey,readRestockPending,persistRestockPending,resolveRestockPending,validRestockInput} from '../local/replenishmentClient.mjs';
import {contextOf,contextKey} from '../context74/model.mjs';
import {LocalAPIError} from '../local/localClient.mjs';
import {access115,page115} from './model.mjs';
// Same durable queues and lock names as the previous interface. No POST on opening.
export function createOrders115({token,session,capabilities,storage,locks,fetcher=globalThis.fetch,isCurrent=()=>true}){
 const scope=contextKey(contextOf(session,capabilities));
 const current=()=>{let valid=false;try{valid=isCurrent()&&contextKey(contextOf(session,capabilities))===scope;}catch{}if(!valid)throw new LocalAPIError(401,'orders115');};
 const scopedFetch=async(...args)=>{current();if(args[1]?.method==='POST')allow(String(args[0]).endsWith('/replenishment/suggestions')?'suggest':'policy');const result=await fetcher(...args);current();return result;};
 const scopedClient=client=>Object.fromEntries(Object.entries(client).map(([name,fn])=>[name,async(...args)=>{current();try{return await fn(...args);}finally{current();}}]));
 current();if(!token||!access115(capabilities).read)throw Error('permission');
 const preview=createRestockPreview({token,session,capabilities,isCurrent,fetcher});
 const p=scopedClient(createPurchaseClient(scopedFetch)),r=scopedClient(createReplenishmentClient(scopedFetch)),pk=pendingPurchaseKey(session),rk=restockKey(session);
 const access=()=>{current();return access115(capabilities);};
 const pending=()=>{current();return {purchase:readPurchasePending(storage,pk),restock:readRestockPending(storage,rk)};};
 const lock=async fn=>{if(!locks?.request)throw Error('locks');return locks.request(pk,{ifAvailable:true},async a=>{if(!a)throw Error('locked');return locks.request(rk,{ifAvailable:true},async b=>{if(!b)throw Error('locked');return fn();});});};
 const allow=kind=>{const a=access();if(!(kind==='suggest'?a.calculate:a.write))throw Error('permission');};
 return {
  previewRestock:preview.preview,assertPreviewCurrent:preview.assertCurrent,
  pending,canLock:!!locks?.request,
  async list(kind,offset=0){const a=access();if(!a.read||kind==='approvals'&&!a.manage)throw Error('permission');const fn={suppliers:()=>p.suppliers(token,offset),orders:()=>p.orders(token,offset),approvals:()=>p.approvals(token,offset),products:()=>r.products(token,offset),suggestions:()=>r.suggestions(token,offset)}[kind];if(!fn)throw Error('kind');return page115(await fn());},
  async order(id){access();return p.order(token,id);},
  async submit(kind,input){allow(kind);if(!(kind==='order'?validPurchaseInput(input):kind==='supplier'?validSupplierInput(input):validRestockInput(kind,input)))throw Error('invalid');return lock(async()=>{allow(kind);const q=pending();if(q.purchase||q.restock)throw Error('pending');if(kind==='order'||kind==='supplier'){persistPurchasePending(storage,pk,{kind,input});return resolvePurchasePending(p,token,storage,pk,true);}persistRestockPending(storage,rk,kind,input);return resolveRestockPending(r,token,storage,rk,true);});},
  async recover(queue,send=false,discard=false){return lock(async()=>{const q=pending(),v=queue==='purchase'?q.purchase:queue==='restock'?q.restock:null;if(!v)return null;if(send||discard)allow(v.kind);else {const a=access();if(queue==='restock'&&!(v.kind==='suggest'?capabilities.permissions.includes('manage_stock'):a.manage)||queue==='purchase'&&!a.manage)throw Error('permission');}if(queue==='purchase'){if(discard)throw Error('uncertain');return resolvePurchasePending(p,token,storage,pk,send);}return resolveRestockPending(r,token,storage,rk,send,discard);});}
 };
}
