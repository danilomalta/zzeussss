import {validateClosing92} from './closing92.mjs';
import {reviewClosing97,gate97} from './closingReview97.mjs';
import {countSummary107} from './count107.mjs';
export function finalStamp108(d,sales,complete){return JSON.stringify([d.scope,d.cash,d.revision,d.counts,d.countConfirmed,d.papers,d.mode,d.approved,d.reason,sales,complete]);}
export function finalReview108(d,sales,complete,manager){
 validateClosing92(d,d.scope,d.cash);if(typeof manager!=='boolean')throw Error('invalidSource108');const counted=countSummary107(d.counts),review=reviewClosing97(sales,d.papers,complete,d.mode),gate=gate97(d,sales,complete,manager);
 const exceptions=!complete||review.issues.length>0||review.rows.some(r=>r.parts.some(p=>p.method!=='cash'&&(d.mode==='integrated'?!p.confirmed:!p.confirmed&&!p.proof)));
 return {counted,review,gate,exceptions,exceptionApproved:gate===null&&exceptions,signature:finalStamp108(d,sales,complete)};
}
export function confirmFinal108(d,sales,complete,manager,signature,ack){const result=finalReview108(d,sales,complete,manager);if(ack!==true||result.signature!==signature)throw Error('changedFinal108');if(result.gate)throw Error(result.gate);return result.counted.total;}
