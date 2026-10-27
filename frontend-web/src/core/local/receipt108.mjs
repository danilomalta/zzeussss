import {validateClosing92} from './closing92.mjs';
import {reviewClosing97} from './closingReview97.mjs';
const sum=values=>{const n=values.reduce((a,b)=>a+BigInt(b),0n);if(n>BigInt(Number.MAX_SAFE_INTEGER))throw Error('invalidSource108');return Number(n);};
export function receiptStamp108(p){return JSON.stringify([p.id,p.method,p.amount,p.reference,p.provider,p.terminal,p.at,p.void,p.sale||'',p.payment||'']);}
export function receiptLedger108(d,sales,complete){
 validateClosing92(d,d.scope,d.cash);const review=reviewClosing97(sales,d.papers,complete,d.mode);
 const rows=d.papers.map(p=>{const issue=review.issues.find(i=>i.id===p.id),sale=review.rows.find(r=>r.id===p.sale);return {...p,status:p.void?'void':p.payment&&!issue?'linked':'pending',issue:issue?.kind||'',saleNumber:sale?.index||null,signature:receiptStamp108(p)};});
 const active=rows.filter(p=>!p.void);
 return {rows,total:sum(active.map(p=>p.amount)),active:active.length,void:rows.length-active.length,linked:active.filter(p=>p.status==='linked').length,pending:active.filter(p=>p.status==='pending').length};
}
const norm=s=>s.normalize('NFD').replace(/[\u0300-\u036f]/g,'').toLocaleLowerCase();
function day108(s){if(!/^\d{4}-\d{2}-\d{2}$/.test(s))throw Error('invalidPeriod108');const d=new Date(s+'T00:00:00Z');if(!Number.isFinite(+d)||d.toISOString().slice(0,10)!==s)throw Error('invalidPeriod108');return s;}
function localDay(at){const d=new Date(at);return [d.getFullYear(),String(d.getMonth()+1).padStart(2,'0'),String(d.getDate()).padStart(2,'0')].join('-');}
export function filterReceipts108(rows,{query='',method='all',status='all',from='',to=''}={}){
 if(!Array.isArray(rows)||rows.length>1000||typeof query!=='string'||query.length>200||!['all','debit','credit','pix'].includes(method)||!['all','pending','linked','void'].includes(status)||typeof from!=='string'||typeof to!=='string')throw Error('invalidSource108');
 if(from)day108(from);if(to)day108(to);if(from&&to&&from>to)throw Error('invalidPeriod108');const q=norm(query.trim());
 return rows.filter(p=>(method==='all'||p.method===method)&&(status==='all'||p.status===status)&&(!from||localDay(p.at)>=from)&&(!to||localDay(p.at)<=to)&&(!q||norm([p.reference,p.provider,p.terminal].join(' ')).includes(q))).sort((a,b)=>Date.parse(b.at)-Date.parse(a.at)||a.id.localeCompare(b.id));
}
export function sameReceipt108(d,id,signature){const p=d.papers.find(p=>p.id===id);if(!p||receiptStamp108(p)!==signature)throw Error('changedReceipt108');return p;}
