import {demoPDF95} from './demoPDF';
import type {Gateway95} from '../src/core/local/service95';import {domains95,type Input95,type Reply95} from '../src/core/local/desk95.mjs';import {demoQR95} from './demoQR';
// Deliberately outside application source: no real PSP, tax or customer traffic.
const records=new Map<string,Reply95>();
export const mockGateway95:Gateway95={mode:'demo',features:domains95,async command(_token,input,signal){if(signal.aborted)throw Error('abort');if(!input.operation)throw Error();let previous=records.get(input.operation);const scenario=input.scenario||'approved',action=input.action,domain=input.domain,amount=previous?.amount??input.amount??0;let state='pending';
 if(domain==='payments')state=action==='cancel'?'cancelled':scenario==='declined'?'declined':scenario==='expired'?'expired':scenario==='pending'?'pending':action==='create'?'pending':'approved';
 if(domain==='credit')state=action==='cancel'?'cancelled':action==='limit'?'ready':scenario==='declined'||amount>80000?'declined':'approved';
 if(domain==='fiscal')state=action==='cancel'?'cancelled':action==='contingency'?'contingency':scenario==='rejected'?'rejected':scenario==='contingency'?'contingency':'issued';
 if(domain==='loyalty')state=action==='lookup'?'ready':scenario==='declined'?'declined':'approved';
 if(domain==='discount')state=action==='cancel'?'cancelled':action==='request'?'pending':action==='authorize'&&input.username&&input.password?'approved':scenario==='declined'?'declined':previous?.state||'pending';
 if(domain==='refund')state=action==='request'?'pending':scenario==='declined'?'declined':scenario==='partial_refund'?'partial_refund':action==='refund'||previous?.state==='partial_refund'?'refunded':'pending';
 if(domain==='devices')state=scenario==='error'?'error':action==='list'?'ready':'approved';
 if(domain==='pairing')state=['attach','submitPaper'].includes(action)?'paired':action==='disconnect'?'disconnected':scenario==='expired'?'expired':action==='create'?'pending':scenario==='pending'?'pending':'paired';
 if(domain==='report')state='ready';
 const r:Reply95={operation:input.operation,domain,state,reference:'DEMO-'+input.operation.slice(0,8),amount};
 if(domain==='payments'&&(input.method==='pix'||previous?.qr)){r.qr=demoQR95;r.copy='TITAN DEMONSTRACAO - NAO PAGAR';r.expires=previous?.expires||new Date(Date.now()+120000).toISOString();}
 if(domain==='credit')r.credit={customer:input.customer||'DEMO',limit:100000,used:20000};
 if(domain==='fiscal')r.document={number:'DEMO-0001',accessKey:'DEMONSTRACAO SEM VALIDADE FISCAL',kind:'demo',...(action==='reprint'?{pdf:demoPDF95}:{})};
 if(domain==='loyalty')r.member={id:'DEMO-MEMBER',name:input.name||'Cliente demonstrativo',points:action==='redeem'?Math.max(0,100-(input.points||0)):action==='earn'?100+(input.points||0):100};
 if(domain==='refund')r.refundedAmount=state==='refunded'?amount:state==='partial_refund'?Math.floor(amount/2):0;
 if(domain==='devices')r.devices=[{id:'demo-printer',name:'Impressora demonstrativa',kind:'printer',state:scenario==='error'?'error':'ready'},{id:'demo-terminal',name:'Terminal demonstrativo',kind:'terminal',state:'ready'}];
 if(domain==='pairing'){r.expires=previous?.expires||new Date(Date.now()+(scenario==='expired'?-1000:180000)).toISOString();r.pair={code:'123456',device:state==='paired'?'Celular demonstrativo':'—',cash:input.cash||previous?.pair?.cash||'DEMO-SHIFT',lastSeen:new Date().toISOString(),reviewed:action==='submitPaper'?(previous?.pair?.reviewed||0)+1:previous?.pair?.reviewed||0};}
 if(domain==='report')r.report={opening:10000,cashSales:3000,withdrawals:0,deposits:0,refunds:0,expected:13000,counted:13000,difference:0,debit:21900,credit:4600,pix:0,pending:0,reviewer:'Conferente demonstrativo'};
 records.set(input.operation,r);if(scenario==='timeout'&&action!=='status')throw Error('simulated lost acknowledgement');return structuredClone(r);
}};
