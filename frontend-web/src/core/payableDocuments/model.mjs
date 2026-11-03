import {key113, date113, document113} from '../payables113/model.mjs';

const invalid=()=>{throw Error('documentInvalid');};
const text=(s,n)=>typeof s==='string'&&s===s.trim()&&s.length>0&&[...s].length<=n&&!/[\u0000-\u001f\u007f]/.test(s);
const exact=(v,keys)=>v&&typeof v==='object'&&!Array.isArray(v)&&Object.keys(v).length===keys.length&&keys.every(k=>Object.hasOwn(v,k));
const stamp=s=>typeof s==='string'&&/^\d{4}-\d{2}-\d{2}T/.test(s)&&Number.isFinite(Date.parse(s));
export function centsInput(value){
 if(typeof value!=='string'||!/^\d{1,14}([.,]\d{1,2})?$/.test(value))invalid();
 const [whole,fraction='']=value.replace(',','.').split('.'),n=BigInt(whole)*100n+BigInt(fraction.padEnd(2,'0'));
 if(n<1n||n>BigInt(Number.MAX_SAFE_INTEGER))invalid();return Number(n);
}
export function createInput(v){
 if(!exact(v,['operation_id','payable_id','supplier_id','document_reference','description','issued_on','currency','total_cents','installments'])||!key113(v.operation_id)||!key113(v.payable_id)||!text(v.supplier_id,128)||!key113(v.document_reference)||!text(v.description,500)||v.currency!=='BRL'||!date113(v.issued_on)||v.issued_on<'1900-01-01'||!Number.isSafeInteger(v.total_cents)||v.total_cents<=0||!Array.isArray(v.installments)||!v.installments.length||v.installments.length>120)invalid();
 let sum=0n,previous=v.issued_on;const seen=new Set();
 for(const p of v.installments){if(!exact(p,['id','due_on','amount_cents'])||!key113(p.id)||seen.has(p.id)||!date113(p.due_on)||p.due_on<previous||!Number.isSafeInteger(p.amount_cents)||p.amount_cents<=0)invalid();seen.add(p.id);previous=p.due_on;sum+=BigInt(p.amount_cents);}
 if(sum!==BigInt(v.total_cents))invalid();return v;
}
export function payableDocument(v){
 document113(v);
 if(!text(v.supplier_id,128)||!text(v.description,500)||!text(v.creator_id,128)||!date113(v.issued_on)||v.issued_on<'1900-01-01'||v.currency!=='BRL'||v.payment_execution_available!==false||v.settled_cents!==null||!stamp(v.created_at)||!stamp(v.updated_at)||!Array.isArray(v.history)||!v.history.length||v.history.length>3)invalid();
 createInput({operation_id:v.last_operation_id,payable_id:v.id,supplier_id:v.supplier_id,document_reference:v.document_reference,description:v.description,issued_on:v.issued_on,currency:v.currency,total_cents:v.total_cents,installments:v.installments});
 let state='',previous=0;const seen=new Set();
 for(const [i,e] of v.history.entries()){
  if(!e||!key113(e.operation_id)||seen.has(e.operation_id)||!text(e.actor_id,128)||!text(e.reason,500)||!stamp(e.created_at)||Date.parse(e.created_at)<previous||e.from_state!==state)invalid();
  const valid=e.action==='create'?i===0&&e.actor_id===v.creator_id&&e.to_state==='pending'&&e.reason===v.description&&e.created_at===v.created_at:
   ['approve','reject'].includes(e.action)?state==='pending'&&e.actor_id!==v.creator_id&&e.to_state===(e.action==='approve'?'approved':'rejected'):
   e.action==='cancel'&&e.to_state==='cancelled'&&(state==='pending'&&e.actor_id===v.creator_id||state==='approved'&&e.actor_id!==v.creator_id);
  if(!valid)invalid();state=e.to_state;previous=Date.parse(e.created_at);seen.add(e.operation_id);
 }
 const last=v.history.at(-1);if(state!==v.state||last.operation_id!==v.last_operation_id||last.created_at!==v.updated_at)invalid();return v;
}
export function documentPage(v,offset){
 if(!v||!Array.isArray(v.items)||v.limit!==20||v.offset!==offset||!Number.isSafeInteger(v.total_count)||v.total_count<0||v.total_count>1000||v.items.length!==Math.min(20,Math.max(0,v.total_count-offset))||v.has_more!==(offset+v.items.length<v.total_count))invalid();
 const seen=new Set();for(const p of v.items){payableDocument(p);if(seen.has(p.id))invalid();seen.add(p.id);}return v;
}
export function availableActions(document,actor){
 payableDocument(document);if(!text(actor,128))return [];
 if(document.state==='pending')return actor===document.creator_id?['cancel']:['approve','reject'];
 return document.state==='approved'&&actor!==document.creator_id&&document.cash?.applied_cents===0?['cancel']:[];
}
export function documentCommand(c){
 if(!exact(c,['version','kind','document','body','expected','uncertain'])||c.version!==1||!key113(c.document)||typeof c.uncertain!=='boolean')invalid();
 if(c.kind==='create'){createInput(c.body);if(c.expected!==null||c.body.payable_id!==c.document)invalid();}
 else {if(!['approve','reject','cancel'].includes(c.kind)||!exact(c.body,['operation_id','reason'])||!key113(c.body.operation_id)||!text(c.body.reason,500)||!exact(c.expected,['last_operation_id','state','creator_id'])||!key113(c.expected.last_operation_id)||!['pending','approved'].includes(c.expected.state)||!text(c.expected.creator_id,128))invalid();}
 return c;
}
export function documentReceipt(v,c,actor){
 documentCommand(c);if(!v||v.operation_id!==c.body.operation_id||typeof v.repeated!=='boolean')invalid();const p=payableDocument(v.payable),e=p.history.at(-1);
 if(p.id!==c.document||e.operation_id!==v.operation_id||e.action!==c.kind||e.actor_id!==actor)invalid();
 if(c.kind==='create'){
  const b=c.body;if(['supplier_id','document_reference','description','issued_on','currency','total_cents'].some(k=>p[k]!==b[k])||p.installments.length!==b.installments.length||p.installments.some((part,i)=>['id','due_on','amount_cents'].some(k=>part[k]!==b.installments[i][k])))invalid();
 }else if(e.reason!==c.body.reason||p.creator_id!==c.expected.creator_id||e.from_state!==c.expected.state||p.history.at(-2)?.operation_id!==c.expected.last_operation_id)invalid();
 return v;
}
