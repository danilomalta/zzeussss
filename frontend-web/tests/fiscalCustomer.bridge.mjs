// Launched only by the disposable Go fixture; all requests use real HTTP/SQLite.
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {createFiscalCustomer,profileMatches,states} from '../src/core/fiscalCustomer/client.mjs';
const {url,token}=JSON.parse(readFileSync(0,'utf8'));
if(!/^http:\/\/127\.0\.0\.1:\d+$/.test(url||'')||!token)throw Error('Disposable Go fixture required');
let active=true,lost=false,calls=0,posts=0;
const api=createFiscalCustomer({token,isCurrent:()=>active,fetcher:async(path,options)=>{
 calls++;if(options.method==='POST')posts++;
 const result=await fetch(new URL(path,url),options);
 if(lost&&options.method==='POST'&&result.ok){lost=false;throw Error('Synthetic loss after real committed response');}
 return result;
}});
assert.equal((await api.capabilities()).transmission_available,false);
const original=await api.read('issuer-one');assert.equal(original.revision,1);
const input={...original.input,operation_id:'customer-bridge-revise',expected_revision:1,legal_name:'Disposable customer bridge'};
const saved=await api.save(input);assert.equal(profileMatches(saved,input),true);
assert.equal(profileMatches(await api.read('issuer-one',2),input),true);
await assert.rejects(api.save({...input,operation_id:'customer-bridge-stale'}),error=>error.status===409);
const uncertain={...input,operation_id:'customer-bridge-uncertain',expected_revision:2,legal_name:'Disposable uncertain revision'};
lost=true;await assert.rejects(api.save(uncertain),error=>error.code==='network');
const recovered=await api.read('issuer-one',3);assert.equal(profileMatches(recovered,uncertain),true);
assert.equal(posts,3); // success, rejected conflict, lost reply; no resend.
assert.equal((await api.read('issuer-one',1)).input.legal_name,original.input.legal_name);
for(const uf of states)assert.equal((await api.requirements(uf)).uf,uf);
active=false;const before=calls;
await assert.rejects(api.save({...uncertain,operation_id:'must-not-post',expected_revision:3}),error=>error.code==='session');
assert.equal(calls,before);
console.log(`PASS fiscal customer frontend + real Go HTTP: ${calls} requests, 3 POSTs including rejected conflict; original revisions and operation IDs preserved; lost response recovered by GET; 27 state references; no SEFAZ transmission or fabricated homologation.`);
