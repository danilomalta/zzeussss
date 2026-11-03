import {cashCount,cashDenominations} from './cashCount.mjs';
export function normalizeCount107(value){if(typeof value!=='string'||!/^\d{0,9}$/.test(value))throw Error('invalidCount107');return value?String(BigInt(value)):'';}
export function countSummary107(counts){
 if(!Array.isArray(counts)||counts.length!==cashDenominations.length||!counts.every(v=>typeof v==='string'))throw Error('invalidCount107');
 const total=cashCount(counts),rows=cashDenominations.map((value,index)=>({index,value,quantity:Number(counts[index]||'0'),subtotal:value*Number(counts[index]||'0'),kind:value>=200?'notes':'coins'}));
 return {total,notes:rows.filter(r=>r.kind==='notes').reduce((s,r)=>s+r.subtotal,0),coins:rows.filter(r=>r.kind==='coins').reduce((s,r)=>s+r.subtotal,0),pieces:rows.reduce((s,r)=>s+r.quantity,0),rows,signature:JSON.stringify(counts)};
}
export function updateCount107(counts,index,value){countSummary107(counts);if(!Number.isInteger(index)||index<0||index>=counts.length)throw Error('invalidCount107');const next=[...counts];next[index]=normalizeCount107(value);countSummary107(next);return next;}
export function stepCount107(counts,index,delta){if(![1,-1].includes(delta))throw Error('invalidCount107');const summary=countSummary107(counts),row=summary.rows[index];if(!row)throw Error('invalidCount107');const q=row.quantity+delta;if(q<0||q>999999999)throw Error('invalidCount107');return updateCount107(counts,index,String(q));}
export function confirmCount107(counts,signature,reviewed){const s=countSummary107(counts);if(signature!==s.signature||reviewed!==true)throw Error('reviewCount107');return s.total;}
