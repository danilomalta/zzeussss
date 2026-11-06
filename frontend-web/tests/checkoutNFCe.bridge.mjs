import {createNFCeCheckoutAdapter} from '../src/core/local/checkoutNFCe.mjs';
import {validateFinal96} from '../src/core/local/checkout96.mjs';
import {createCheckoutFiscalClient} from '../src/core/local/checkoutFiscal.mjs';

// Stdin only: no installed data, credentials in argv, fixtures on disk or logs.
let input='';for await(const chunk of process.stdin)input+=chunk;
const b=JSON.parse(input),current={token:b.token,session:b.session};input='';
const paths=[];
const fetcher=(path,options)=>{if(!path.startsWith('/local/v1/'))throw Error('nonlocal request');paths.push([options.method,path]);return fetch(new URL(path,b.url),options);};
const setup={nature:'VENDA',presence:'1',policies:{0:b.policy},unitPrices:{0:1599},issuerAddress:{Street:'Rua Sintetica',Number:'1',District:'Centro',City:'Sao Paulo',PostalCode:'01001000'}};
try{
 const adapter=createNFCeCheckoutAdapter({getCurrent:()=>current,getSetup:()=>setup,fetcher});
 const items=[{product_id:b.product,location_id:b.location,quantity_milli:1000}],id='checkout-browser';
 const quote=await adapter.quote(b.token,{checkout:id,cash:'turn-one',items,gross:1599,discount:0,points:0,reason:'',customer:{cpf:'',club:'skip'},fiscal:'nfce'},new AbortController().signal);
 const plan={version:1,id,cash:'turn-one',source:'disposable-cart',mode:'live',state:'quoted',total:1599,parts:[{id:'browser-cash',method:'cash',amount:1599,state:'approved',reference:''}],received:2000,quote,fiscal:'nfce',items};
 const result=validateFinal96(await adapter.finalize(b.token,{plan,items},new AbortController().signal),plan,items,'nfce');
 if(result.state!=='committed'||!result.receipt.fiscal_authorized)throw Error('checkout not authorized by disposable authority');
 // Client restart loses every in-memory invoice/quote, but status and print
 // continue to use the original sale and server-owned fiscal evidence.
 const restarted=createNFCeCheckoutAdapter({getCurrent:()=>current,getSetup:()=>{throw Error('client request lost');},fetcher});
 const before=paths.filter(([method])=>method==='POST').length;
 const recovered=validateFinal96(await restarted.status(b.token,id,new AbortController().signal),plan,items,'nfce');
 if(recovered.receipt.sale_id!==result.receipt.sale_id||paths.filter(([method])=>method==='POST').length!==before)throw Error('status changed identity or resent a command');
 const client=createCheckoutFiscalClient({getCurrent:()=>current,fetcher}).bind({token:b.token,session:b.session});
 const html=await client.document(id,{width:80,timezone:'America/Sao_Paulo'});
 if(!html.includes('SEM VALOR FISCAL')||!html.includes('data:image/png;base64,'))throw Error('missing homologation print evidence');
 console.log('PASS JavaScript -> real HTTP/SQLite -> disposable signed NFC-e; restart/read/80mm print; no second POST. Not Focus/SEFAZ acceptance.');
}catch(e){console.error('FAIL NFC-e frontend bridge:',e instanceof Error?e.message:'unknown');process.exitCode=1;}
