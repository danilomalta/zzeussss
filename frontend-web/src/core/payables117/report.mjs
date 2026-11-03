import {date113,key113} from '../payables113/model.mjs';
import {window114} from '../payables114/agenda.mjs';
import {csv97} from '../local/closingReview97.mjs';
export const buckets117=['future','today','days1to7','days8to30','days31to60','days61to90','days91plus'];
export const totals117=['approved','pending','rejected','cancelled','applied','balance','overdue'];
const fail=()=>{throw Error('invalid117');},max=BigInt(Number.MAX_SAFE_INTEGER);
const cents=v=>Number.isSafeInteger(v)&&v>=0;
const day=v=>Date.parse(v+'T12:00:00Z')/86400000;
export function age117(due,reference){if(!date113(due)||!date113(reference))return fail();const d=day(reference)-day(due);return d<0?'future':d===0?'today':d<=7?'days1to7':d<=30?'days8to30':d<=60?'days31to60':d<=90?'days61to90':'days91plus';}
function add(obj,k,n){const value=BigInt(obj[k]||0)+BigInt(n);if(value>max)fail();obj[k]=Number(value);}
const blank=()=>Object.fromEntries(totals117.map(k=>[k,0]));
export function summarize117(rows,window){window114(window);if(!Array.isArray(rows)||rows.length>10000)fail();const seen=new Set(),documents=new Map(),suppliers=new Map(),totals=blank(),buckets=Object.fromEntries(buckets117.map(k=>[k,0]));
 for(const r of rows){if(!r||![r.payable_id,r.installment_id,r.supplier_id].every(key113)||typeof r.supplier_name!=='string'||!r.supplier_name.trim()||r.supplier_name.length>255||typeof r.document_reference!=='string'||!r.document_reference.trim()||!['approved','pending','rejected','cancelled'].includes(r.state)||!date113(r.due_on)||r.due_on<window.from||r.due_on>window.to||![r.amount_cents,r.cash_applied_cents,r.outstanding_cents].every(cents)||!r.amount_cents||r.cash_applied_cents>r.amount_cents||r.outstanding_cents!==(r.state==='approved'?r.amount_cents-r.cash_applied_cents:0))fail();
 const id=JSON.stringify([r.payable_id,r.installment_id]);if(seen.has(id))fail();seen.add(id);const identity=JSON.stringify([r.supplier_id,r.document_reference,r.state]);if(documents.has(r.payable_id)&&documents.get(r.payable_id)!==identity)fail();documents.set(r.payable_id,identity);const age=age117(r.due_on,window.as_of),expected=age==='future'?'future':age==='today'?'today':'overdue';if(r.timing!==expected)fail();
 let s=suppliers.get(r.supplier_id);if(!s){s={id:r.supplier_id,name:r.supplier_name,parts:0,documentIDs:new Set(),...blank(),nextDue:null,oldestDue:null};suppliers.set(r.supplier_id,s);}s.parts++;s.documentIDs.add(r.payable_id);
 for(const obj of [s,totals]){add(obj,r.state,r.amount_cents);add(obj,'applied',r.cash_applied_cents);add(obj,'balance',r.outstanding_cents);if(expected==='overdue')add(obj,'overdue',r.outstanding_cents);}
 add(buckets,age,r.outstanding_cents);
 if(r.outstanding_cents){s.oldestDue=s.oldestDue===null||r.due_on<s.oldestDue?r.due_on:s.oldestDue;if(r.due_on>=window.as_of)s.nextDue=s.nextDue===null||r.due_on<s.nextDue?r.due_on:s.nextDue;}
 }
 return {totals,buckets,suppliers:[...suppliers.values()].map(({documentIDs,...s})=>({...s,documents:documentIDs.size})),parts:rows.length,documents:documents.size,window:{...window},currency:'BRL'};
}
export function sort117(rows,field='balance',direction='desc',locale='pt-BR'){if(!['name','balance','overdue','nextDue'].includes(field)||!['asc','desc'].includes(direction))fail();const collator=new Intl.Collator(locale,{numeric:true,sensitivity:'base'}),sign=direction==='asc'?1:-1;return [...rows].sort((a,b)=>{let c;if(field==='name')c=collator.compare(a.name,b.name);else if(field==='nextDue'){if(a.nextDue===null||b.nextDue===null)return a.nextDue===b.nextDue?collator.compare(a.name,b.name):a.nextDue===null?1:-1;c=a.nextDue.localeCompare(b.nextDue);}else c=a[field]===b[field]?0:a[field]<b[field]?-1:1;return sign*c||collator.compare(a.name,b.name)||a.id.localeCompare(b.id);});}
export function matrices117(report,rows,w,meta){const base=[[w.title],[w.coverage],[w.asOfWarning],[w.from,report.window.from,w.to,report.window.to],[w.asOf,report.window.as_of,w.readAt,meta.readAt],[w.exportScope,meta.scope],[w.currency,'BRL'],[w.unit,w.cents]];
 const summary=[...base,[],[w.metric,w.value],...totals117.map(k=>[w[k],report.totals[k]]),[w.suppliers,report.suppliers.length],[w.documents,report.documents],[w.parts,report.parts],[],[w.age,w.balance],...buckets117.map(k=>[w[k],report.buckets[k]])];
 const suppliers=[[w.supplier,w.documents,w.parts,w.approved,w.pending,w.rejected,w.cancelled,w.applied,w.balance,w.overdue,w.nextDue,w.oldestDue],...sort117(report.suppliers).map(s=>[s.name,s.documents,s.parts,s.approved,s.pending,s.rejected,s.cancelled,s.applied,s.balance,s.overdue,s.nextDue||'',s.oldestDue||''])];
 const items=[[w.supplier,w.document,w.state,w.due,w.amount,w.applied,w.balance,w.age],...rows.map(r=>[r.supplier_name,r.document_reference,w[r.state],r.due_on,r.amount_cents,r.cash_applied_cents,r.outstanding_cents,w[age117(r.due_on,report.window.as_of)]])];return {summary,suppliers,items};
}
export function csv117(matrices,w){return csv97([...matrices.summary,[],[w.suppliers],...matrices.suppliers,[],[w.parts],...matrices.items]);}
export function sheet117(report,w){if(report.suppliers.length>500)throw Error('sheetLimit117');return [[w.supplier,w.documents,w.parts,w.balanceCents,w.overdueCents,w.appliedCents,w.from,w.to,w.asOf],...sort117(report.suppliers).map(s=>[s.name,s.documents,s.parts,s.balance,s.overdue,s.applied,report.window.from,report.window.to,report.window.as_of])];}
