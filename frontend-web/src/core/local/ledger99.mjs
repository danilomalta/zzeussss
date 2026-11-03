import {validateSales97} from './closingReview97.mjs';
import {validate94,manager94} from './till94.mjs';
const safe=n=>Number.isSafeInteger(n)&&n>=0;
const sum=values=>{let n=0n;for(const v of values){if(!safe(v))throw Error('invalid99');n+=BigInt(v);}if(n>BigInt(Number.MAX_SAFE_INTEGER))throw Error('invalid99');return Number(n);};
const text=v=>typeof v==='string'&&v.length>0&&v.length<=400;
const at=v=>typeof v==='string'&&Number.isFinite(Date.parse(v));
export function ledger99(sales,data,complete){
 validateSales97(sales);if(sales.reduce((n,s)=>n+s.payments.length,0)>20000)throw Error('invalid99');validate94(data,data.scope,data.cash);if(typeof complete!=='boolean')throw Error('invalid99');
 const entries=[];let index=0;
 for(const s of sales){index++;if(s.status!=='committed')continue;for(const [n,p] of s.payments.entries())entries.push({key:JSON.stringify(['sale',s.id,p.id]),at:s.at,origin:'sale',kind:'sale',method:p.method,amount:p.amount,state:'committed',reference:s.id,label:index,part:n+1,reason:'',actor:''});}
 for(const [n,p] of data.requests.entries()){if(!text(p.id)||!at(p.at))throw Error('invalid99');entries.push({key:JSON.stringify(['request',p.id]),at:p.at,origin:'request',kind:p.kind,method:'',amount:p.amount,state:p.state,reference:p.reference,label:n+1,part:0,reason:p.reason,actor:p.actor});}
 const ids=new Set();for(const [n,j] of data.journal.entries()){if(!text(j.id)||ids.has(j.id)||!at(j.at)||!text(j.kind)||!text(j.actor)||!safe(j.amount)||typeof j.reference!=='string')throw Error('invalid99');ids.add(j.id);entries.push({key:JSON.stringify(['audit',j.id]),at:j.at,origin:'audit',kind:j.kind,method:'',amount:j.amount,state:'local99',reference:j.reference,label:n+1,part:0,reason:'',actor:j.actor});}
 const cash=sum(entries.filter(e=>e.origin==='sale'&&e.method==='cash').map(e=>e.amount)),total=sum(entries.filter(e=>e.origin==='sale').map(e=>e.amount));
 return {entries,cash:complete?cash:null,total:complete?total:null,complete,sales:sales.filter(s=>s.status==='committed').length,requests:data.requests.length};
}
export function filterLedger99(entries,{origin='all',method='all',state='all',query='',from=null,to=null,order='desc'}={}){
 if(query.length>400||from!==null&&!Number.isFinite(from)||to!==null&&!Number.isFinite(to)||from!==null&&to!==null&&from>=to)throw Error('range99');
 const q=query.normalize('NFD').replace(/[\u0300-\u036f]/g,'').toLocaleLowerCase().trim();
 return entries.filter(e=>(origin==='all'||e.origin===origin)&&(method==='all'||e.method===method)&&(state==='all'||e.state===state)&&(from===null||Date.parse(e.at)>=from)&&(to===null||Date.parse(e.at)<to)&&(!q||[e.reference,e.reason,String(e.label)].some(v=>v.normalize('NFD').replace(/[\u0300-\u036f]/g,'').toLocaleLowerCase().includes(q)))).sort((a,b)=>{const n=Date.parse(a.at)-Date.parse(b.at)||a.key.localeCompare(b.key);return order==='asc'?n:-n;});
}
export function range99(from,to){
 const parse=s=>{if(!/^\d{4}-\d{2}-\d{2}$/.test(s))throw Error('range99');const d=new Date(s+'T00:00:00');if(!Number.isFinite(d.getTime())||d.getFullYear()!==Number(s.slice(0,4))||d.getMonth()+1!==Number(s.slice(5,7))||d.getDate()!==Number(s.slice(8,10)))throw Error('range99');return d;};
 const f=from?parse(from).getTime():null,t=to?parse(to):null;if(t)t.setDate(t.getDate()+1);const end=t?t.getTime():null;if(f!==null&&end!==null&&f>=end)throw Error('range99');return {from:f,to:end};
}
export function bag99(b,cash){
 const stages=['sealed','handed','received','verified'];if(!b||!text(b.id)||b.cash!==cash||!safe(b.amount)||b.amount===0||!text(b.destination)||!['dual','self'].includes(b.policy)||!stages.includes(b.state)||!Array.isArray(b.chain)||b.chain.length!==stages.indexOf(b.state)+1)throw Error('invalidBag99');
 for(const [i,e] of b.chain.entries()){if(!e||e.stage!==stages[i]||!text(e.actor)||!at(e.at)||i&&Date.parse(e.at)<Date.parse(b.chain[i-1].at)||b.policy==='dual'&&i>=2&&e.actor===b.chain[0].actor)throw Error('invalidBag99');if(i===3&&(!safe(e.count)||typeof e.reason!=='string'||e.reason.length>400||e.count!==b.amount&&e.reason.trim().length<3||b.difference!==e.count-b.amount))throw Error('invalidBag99');}
 return b;
}
export function bagAction99(b,actor){
 bag99(b,b.cash);if(!actor||!text(actor.id)||!Array.isArray(actor.permissions)||!actor.permissions.some(p=>['sell','manage_cash'].includes(p)))return {next:'',reason:'permission'};
 const next={sealed:'handed',handed:'received',received:'verified'}[b.state]||'';if(!next)return {next:'',reason:''};if(next==='handed'&&actor.id!==b.chain[0].actor&&!manager94(actor))return {next:'',reason:'permission'};if(b.policy==='dual'&&['received','verified'].includes(next)&&actor.id===b.chain[0].actor)return {next:'',reason:'differentReviewer'};if(next==='verified'&&!manager94(actor))return {next:'',reason:'permission'};return {next,reason:''};
}
