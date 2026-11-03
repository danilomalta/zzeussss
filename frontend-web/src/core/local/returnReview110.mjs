import {validateReceipt100,receiptFingerprint100,estimateReturn100,parseReturnQuantity100,filterRequests100} from './returns100.mjs';
export function returnReview110(input){
 const {receipt,requests,mode,item,quantity,unit,outcome,reason,scope,cash,actor}=input;
 validateReceipt100(receipt,receipt.sale_id);if(receipt.status!=='committed')throw Error('receiptCancelled100');
 if(!['item','cancel'].includes(mode)||typeof reason!=='string'||reason.trim().length<3||reason.length>400||![scope,cash,actor].every(v=>typeof v==='string'&&v.trim()))throw Error('invalid100');
 filterRequests100(requests);const related=requests.filter(r=>r.reference===receipt.sale_id&&['return','cancel'].includes(r.kind)),ids=new Set();for(const r of related){if(ids.has(r.id)||r.amount<=0)throw Error('invalid100');ids.add(r.id);}
 let amount=receipt.total_cents,qty=null;
 if(mode==='cancel'){if(related.some(r=>!['denied','withdrawn'].includes(r.state)))throw Error('cancelHelp100');}
 else {if(!['refund','exchange','credit'].includes(outcome)||typeof unit!=='string')throw Error('invalid100');qty=parseReturnQuantity100(quantity,unit);amount=estimateReturn100(receipt,item,qty,requests).amount;}
 if(!Number.isSafeInteger(amount)||amount<=0)throw Error('noValue100');
 const signature=JSON.stringify([receiptFingerprint100(receipt),related,mode,item,quantity,unit,outcome,reason,scope,cash,actor]);
 return {amount,quantity:qty,mode,outcome:mode==='cancel'?'':outcome,reason,signature};
}
export function confirmReturn110(input,signature,ack){const v=returnReview110(input);if(ack!==true||v.signature!==signature)throw Error('changed110');return v;}
export function returnQueue110(requests){
 const rows=filterRequests100(requests),ids=new Set();for(const r of rows){if(ids.has(r.id)||r.amount<=0)throw Error('invalid100');ids.add(r.id);}
 const pending=rows.filter(r=>r.state==='requested');let amount=pending.reduce((a,r)=>a+BigInt(r.amount),0n);if(amount>BigInt(Number.MAX_SAFE_INTEGER))throw Error('invalid100');
 return {requested:pending.length,approved:rows.filter(r=>r.state==='approved').length,denied:rows.filter(r=>r.state==='denied').length,withdrawn:rows.filter(r=>r.state==='withdrawn').length,pendingAmount:Number(amount),refund:rows.filter(r=>r.kind==='return'&&r.outcome==='refund').length,exchange:rows.filter(r=>r.kind==='return'&&r.outcome==='exchange').length,credit:rows.filter(r=>r.kind==='return'&&r.outcome==='credit').length,cancel:rows.filter(r=>r.kind==='cancel').length};
}
