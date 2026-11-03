import {bag99,bagAction99} from './ledger99.mjs';
import {validate94} from './till94.mjs';
const sum=values=>{const n=values.reduce((a,v)=>a+BigInt(v),0n);if(n>BigInt(Number.MAX_SAFE_INTEGER)||n<BigInt(Number.MIN_SAFE_INTEGER))throw Error('invalid109');return Number(n);};
const norm=s=>s.normalize('NFD').replace(/[\u0300-\u036f]/g,'').toLocaleLowerCase().trim();
export function bags109(data,actor,{query='',state='all',variance=false}={}){
 validate94(data,data.scope,data.cash);if(typeof query!=='string'||query.length>200||!['all','sealed','handed','received','verified'].includes(state)||typeof variance!=='boolean')throw Error('invalid109');
 const rows=data.bags.map(b=>{bag99(b,data.cash);return {...b,next:bagAction99(b,actor),last:b.chain.at(-1).at,difference:b.state==='verified'?b.difference:null};});
 const q=norm(query),filtered=rows.filter(b=>(state==='all'||b.state===state)&&(!variance||b.difference!==null&&b.difference!==0)&&(!q||norm(b.id+' '+b.destination).includes(q))).sort((a,b)=>Date.parse(b.last)-Date.parse(a.last)||a.id.localeCompare(b.id));
 return {rows:filtered,total:rows.length,open:rows.filter(b=>b.state!=='verified').length,verified:rows.filter(b=>b.state==='verified').length,variances:rows.filter(b=>b.difference!==null&&b.difference!==0).length,amount:sum(rows.map(b=>b.amount)),difference:sum(rows.filter(b=>b.difference!==null).map(b=>b.difference))};
}
export function ledgerSummary109(entries,complete){
 if(!Array.isArray(entries)||entries.length>25500||typeof complete!=='boolean')throw Error('invalid109');const keys=new Set();
 for(const r of entries){if(!r||typeof r.key!=='string'||keys.has(r.key)||!['sale','request','audit'].includes(r.origin)||!Number.isSafeInteger(r.amount)||r.amount<0||!Number.isFinite(Date.parse(r.at))||typeof r.state!=='string'||typeof r.kind!=='string'||!Number.isSafeInteger(r.label)||r.label<1||!Number.isSafeInteger(r.part)||r.part<0||typeof r.reason!=='string'||typeof r.actor!=='string'||typeof r.reference!=='string'||r.origin==='sale'&&!['cash','debit','credit','pix','creditAccount'].includes(r.method))throw Error('invalid109');keys.add(r.key);}
 const sales=entries.filter(r=>r.origin==='sale'),requests=entries.filter(r=>r.origin==='request'),audit=entries.filter(r=>r.origin==='audit');
 return {sales:sales.length,requests:requests.length,audit:audit.length,total:complete?sum(sales.map(r=>r.amount)):null,methods:['cash','debit','credit','pix','creditAccount'].map(method=>({method,parts:sales.filter(r=>r.method===method).length,amount:complete?sum(sales.filter(r=>r.method===method).map(r=>r.amount)):null})),requested:requests.filter(r=>r.state==='requested').length,approved:requests.filter(r=>r.state==='approved').length};
}
export const entryStamp109=r=>JSON.stringify([r.key,r.at,r.origin,r.kind,r.method,r.amount,r.state,r.reference,r.label,r.part,r.reason,r.actor]);
