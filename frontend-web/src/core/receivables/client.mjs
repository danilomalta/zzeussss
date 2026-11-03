import {createRecoveryJournal} from './recovery.mjs';
const pendingCommands=new Map(),locks=new Set();
const fields=['tenant_id','store_id','identity_id','device_id'];
const id=v=>typeof v==='string'&&/^[A-Za-z0-9][A-Za-z0-9_-]{0,127}$/.test(v);
const cents=v=>Number.isSafeInteger(v)&&v>=0;
const text=(v,n)=>typeof v==='string'&&v.length>0&&v.trim()===v&&v.length<=n&&!/[\u0000\r\n]/.test(v);
const date=v=>typeof v==='string'&&/^\d{4}-\d{2}-\d{2}$/.test(v)&&Number.isFinite(Date.parse(v))&&new Date(v).toISOString().slice(0,10)===v;
export function validateCustomerOrder(v){
 if(!v||!text(v.customer_tenant_id,128)||!text(v.customer_store_id,128)||!id(v.message_id)||!id(v.decision_id)||!id(v.order_id)||!text(v.customer_reference,128)||!text(v.product_id,128)||!text(v.unit,128)||!cents(v.accepted_milli)||v.accepted_milli<1||typeof v.offer_sha256!=='string'||!/^[a-f0-9]{64}$/.test(v.offer_sha256))throw new ReceivableError('invalid');return v;
}
export function validateCreateInput(v){
 if(!v||!id(v.id)||!id(v.operation_id)||!text(v.customer_reference,128)||!text(v.document_reference,128)||!text(v.description,500)||!date(v.issued_on)||!cents(v.total_cents)||!v.total_cents||!Array.isArray(v.installments)||!v.installments.length||v.installments.length>120)throw new ReceivableError('invalid');
 if(v.customer_order!=null&&(!id(v.customer_order.message_id)||!text(v.customer_order.terms_reference,500)||v.customer_order.confirmed!==true))throw new ReceivableError('invalid');
 if(v.credit_sale!=null){const c=v.credit_sale;if(v.customer_order!=null||!id(c.sale_id)||!id(c.session_id)||!Array.isArray(c.items)||!c.items.length||c.items.length>500)throw new ReceivableError('invalid');for(const p of c.items)if(!id(p.product_id)||!id(p.location_id)||!cents(p.quantity_milli)||!p.quantity_milli)throw new ReceivableError('invalid');}
 const seen=new Set();let sum=0n;for(const p of v.installments){if(!p||!id(p.id)||seen.has(p.id)||!date(p.due_on)||p.due_on<v.issued_on||!cents(p.amount_cents)||!p.amount_cents)throw new ReceivableError('invalid');seen.add(p.id);sum+=BigInt(p.amount_cents);}
 if(sum!==BigInt(v.total_cents))throw new ReceivableError('invalid');return v;
}
export class ReceivableError extends Error{constructor(code,status=0){super(code);this.status=status;}}
export function parseCents(value){
 if(typeof value!=='string'||!/^\d{1,14}([.,]\d{1,2})?$/.test(value))throw new ReceivableError('invalid');
 const [whole,fraction='']=value.replace(',','.').split('.'),n=BigInt(whole)*100n+BigInt(fraction.padEnd(2,'0'));
 if(n<1n||n>BigInt(Number.MAX_SAFE_INTEGER))throw new ReceivableError('invalid');return Number(n);
}
export function validateDocument(v){
 if(!v||!id(v.id)||!['open','settled','cancelled'].includes(v.state)||!cents(v.total_cents)||v.total_cents<1||!cents(v.received_cents)||!cents(v.outstanding_cents)||!Array.isArray(v.installments)||!v.installments.length||!Array.isArray(v.receipts)||!Array.isArray(v.history))throw new ReceivableError('invalid');
 let total=0n,received=0n;const seen=new Set();
 for(const part of v.installments){if(!part||!id(part.id)||seen.has(part.id)||!cents(part.amount_cents)||part.amount_cents<1||!cents(part.received_cents)||part.received_cents>part.amount_cents)throw new ReceivableError('invalid');seen.add(part.id);total+=BigInt(part.amount_cents);received+=BigInt(part.received_cents);}
 if(total!==BigInt(v.total_cents)||received!==BigInt(v.received_cents)||v.state==='cancelled'&&(received!==0n||v.outstanding_cents!==0)||v.state!=='cancelled'&&BigInt(v.outstanding_cents)!==total-received||v.state==='settled'&&v.outstanding_cents!==0)throw new ReceivableError('invalid');
 let receipts=0n;for(const receipt of v.receipts){if(!receipt||!id(receipt.operation_id)||!id(receipt.movement_id)||!cents(receipt.amount_cents)||!receipt.amount_cents)throw new ReceivableError('invalid');receipts+=BigInt(receipt.amount_cents);}
 if(!cents(v.reversed_cents??0)||receipts-BigInt(v.reversed_cents??0)!==received||!v.history.length||!text(v.customer_reference,128)||!text(v.document_reference,128)||!text(v.description,500)||!date(v.issued_on))throw new ReceivableError('invalid');
 if(v.customer_order!=null&&(validateCustomerOrder(v.customer_order.source).customer_reference!==v.customer_reference||!text(v.customer_order.terms_reference,500)))throw new ReceivableError('invalid');
 return v;
}
export function createReceivablesClient({fetcher=globalThis.fetch,getCurrent,pending=pendingCommands,journal=createRecoveryJournal()}){
 return {bind(context){
  if(!context.token||!context.session||!fields.every(k=>typeof context.session[k]==='string'&&context.session[k]))throw new ReceivableError('context');
  const scope=JSON.stringify(fields.map(k=>context.session[k])),controllers=new Set();
  const current=()=>{const now=getCurrent();if(now.token!==context.token||now.session!==context.session||JSON.stringify(fields.map(k=>now.session?.[k]))!==scope)throw new ReceivableError('context');};
  async function request(path,body){
   current();const controller=new AbortController();controllers.add(controller);const timer=setTimeout(()=>controller.abort(),15000);
   try{const response=await fetcher('/local/v1/finance/receivables'+path,{method:body?'POST':'GET',credentials:'omit',cache:'no-store',signal:controller.signal,headers:{Authorization:`Bearer ${context.token}`,...(body?{'Content-Type':'application/json'}:{})},...(body?{body:JSON.stringify(body)}:{})});current();
    if(!response.ok)throw new ReceivableError(response.status===403?'denied':response.status===409?'conflict':'unavailable',response.status);
    const result=await response.json();current();return result;
   }finally{clearTimeout(timer);controllers.delete(controller);}
  }
  function result(value,command){if(value?.resolution!==undefined){if(value.resolution!=='not_applied'||value.operation_id!==command.op||value.document_id!==command.id||value.action!==command.action||typeof value.repeated!=='boolean')throw new ReceivableError('invalid');return null;}if(!value||value.operation_id!==command.op||value.action!==command.action||typeof value.repeated!=='boolean'||validateDocument(value.document).id!==command.id)throw new ReceivableError('invalid');return value;}
  async function restore(){current();if(!journal)return;const saved=await journal.read(scope);current();if(saved&&!pending.has(scope))pending.set(scope,{...saved,uncertain:true});}
  async function forget(command){if(journal)await journal.remove(scope,command);if(pending.get(scope)===command)pending.delete(scope);}
  async function locked(work){current();if(locks.has(scope))throw new ReceivableError('busy');locks.add(scope);try{await restore();return await work();}finally{locks.delete(scope);}}
  async function send(command){if(!command.body||command.originToken!==context.token||command.originSession!==context.session)throw new ReceivableError('recoveryOnly');try{const value=result(await request(command.path,command.body),command);current();await forget(command);return value;}catch(e){if(e instanceof ReceivableError&&[400,404,409,422].includes(e.status)&&!command.uncertain)await forget(command);else command.uncertain=true;throw e;}}
  async function submit(action,document,path,input){if(!id(document)||!id(input?.operation_id))throw new ReceivableError('invalid');return locked(async()=>{if(pending.has(scope))throw new ReceivableError('pending');const command={op:input.operation_id,id:document,action,path,body:JSON.parse(JSON.stringify(input)),uncertain:false,originToken:context.token,originSession:context.session};
   if(journal){const saved=await journal.reserve(scope,command);current();if(!saved){await restore();throw new ReceivableError('pending');}}
   pending.set(scope,command);return send(command);});}
  return {
   async ready(){await restore();return this.pending();},
   pending(){const p=pending.get(scope);return p?{operation_id:p.op,document_id:p.id,action:p.action}:null;},
   dispose(){for(const c of controllers)c.abort();},
   async list(offset=0){if(!Number.isSafeInteger(offset)||offset<0)throw new ReceivableError('invalid');const p=await request('?limit=20&offset='+offset);if(!p||!Array.isArray(p.items)||p.limit!==20||p.offset!==offset||!cents(p.total)||p.items.length>20)throw new ReceivableError('invalid');p.items.forEach(validateDocument);return p;},
   async customerOrder(message){if(!id(message))throw new ReceivableError('invalid');const v=validateCustomerOrder(await request('/customer-orders/'+encodeURIComponent(message)));if(v.message_id!==message)throw new ReceivableError('invalid');return v;},
   async read(document){if(!id(document))throw new ReceivableError('invalid');const v=validateDocument(await request('/'+encodeURIComponent(document)));if(v.id!==document)throw new ReceivableError('invalid');return v;},
   create(input){validateCreateInput(input);return submit('create',input.id,'',input);},
   receive(document,input){if(!id(input?.session_id)||!id(input?.installment_id)||!cents(input.amount_cents)||!input.amount_cents||!cents(input.expected_received_cents))throw new ReceivableError('invalid');return submit('receive_cash',document,'/'+encodeURIComponent(document)+'/receive-cash',input);},
   reverse(document,input){if(!id(input?.original_operation_id)||!id(input.original_device_id)||!id(input.session_id)||!cents(input.amount_cents)||!input.amount_cents||!cents(input.expected_reversed_cents)||!text(input.reason,500))throw new ReceivableError('invalid');return submit('reverse_cash',document,'/'+encodeURIComponent(document)+'/reverse-cash',input);},
   cancel(document,input){if(!text(input?.reason,500))throw new ReceivableError('invalid');return submit('cancel',document,'/'+encodeURIComponent(document)+'/cancel',input);},
   async reconcile(){return locked(async()=>{const command=pending.get(scope);if(!command)return null;
    try{const value=result(await request('/operations/'+encodeURIComponent(command.op)),command);current();await forget(command);return value;}catch(e){if(!(e instanceof ReceivableError)||e.status!==404)throw e;}
    current();const value=result(await request('/operations/'+encodeURIComponent(command.op)+'/reconcile',{document_id:command.id,action:command.action,confirmed:true}),command);current();await forget(command);return value;
   });},
   async recover(retry=false){return locked(async()=>{const command=pending.get(scope);if(!command)return null;
    try{const value=result(await request('/operations/'+encodeURIComponent(command.op)),command);current();await forget(command);return value;}catch(e){if(!(e instanceof ReceivableError)||e.status!==404)throw e;}
    current();if(!retry)return null;return send(command);
   });}
  };
 }};
}
