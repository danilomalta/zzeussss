import {reviewClosing97} from './closingReview97.mjs';
// A historical report must never borrow a live service response for another till.
export function serviceFor98(selected,current,service){return selected===current?service:undefined;}
export function reconciliationRows98(sales,papers,complete,mode,w,money){
 const review=reviewClosing97(sales,papers,complete,mode);
 const rows=[[w.total97,review.total===null?'—':money(review.total)],[w.pending97,review.pending===null?'—':money(review.pending)],[w.unverified98,review.unverified===null?'—':money(review.unverified)]];
 for(const m of review.methods){const label=w[m.method];rows.push([label+' · '+w.expected97,m.expected===null?'—':money(m.expected)],[label+' · '+w.electronic97,money(m.confirmed)],[label+' · '+w.paper97,money(m.documented)],[label+' · '+w.checked97,money(m.verified)],[label+' · '+w.pending97,m.pending===null?'—':money(m.pending)]);}
 rows.push([w.toReview97,String(review.issues.length)]);for(const issue of review.issues)rows.push([w.warning97,w[issue.kind]||w.invalidEvidence97]);
 return {review,rows};
}
