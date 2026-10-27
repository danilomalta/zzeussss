// Opt-in actual HTTP bridge. Requires two synthetic services supplied by Go tests.
import assert from 'node:assert/strict';
import {createCompanyTrade} from '../src/core/companyTrade66/source.mjs';
if(!process.env.TITAN_TRADE66_FIXTURES)throw Error('Run only with disposable Go fixtures');
const fixtures=JSON.parse(process.env.TITAN_TRADE66_FIXTURES);
async function client(f){const fetcher=(path,o)=>fetch(f.url+path,o),r=await fetcher('/local/v1/capabilities',{headers:{Authorization:'Bearer '+f.token}});assert.equal(r.status,200);const capabilities=await r.json(),map=new Map();return createCompanyTrade({token:f.token,session:f.session,capabilities,isCurrent:()=>true,storage:{getItem:k=>map.get(k)??null,setItem:(k,v)=>map.set(k,v),removeItem:k=>map.delete(k)},locks:{request:async(k,o,fn)=>fn({name:k})},fetcher});}
const b=await client(fixtures.buyer),s=await client(fixtures.supplier);
async function write(api,kind,target,body){const c=await api.prepare(kind,target,{operation_id:crypto.randomUUID(),...body}),r=await api.submit(c);assert.equal(r.state,'confirmed');assert.equal(api.pending(),null);return r.receipt;}
try{
 const bi=await b.identity(),si=await s.identity(),r=await b.recovery();assert.equal(r.required,true);
 await write(b,'review','',{generation:r.generation,verification_reference:'Synthetic owner review after restore'});
 assert.equal((await b.recovery()).required,false);
 await write(b,'trust','',{id:'vendor',remote:si.party,public_key:si.public_key,fingerprint_sha256:si.fingerprint_sha256,role:'supplier',supplier_id:'supplier',verification_reference:'Synthetic independent fingerprint check'});
 await write(s,'trust','',{id:'customer',remote:bi.party,public_key:bi.public_key,fingerprint_sha256:bi.fingerprint_sha256,role:'customer',supplier_id:'',verification_reference:'Synthetic independent fingerprint check'});
 assert.equal((await b.peers()).items[0].id,'vendor');
 const offer=await write(b,'export','order',{peer_id:'vendor',reason:'Approved synthetic order'});
 const imported=await write(s,'import','',{peer_id:'customer',envelope:offer.envelope});assert.equal(imported.message.id,offer.message.id);
 const received=await s.messages(0,'received','order');assert.equal(received.total,1);assert.equal((await s.message(offer.message.id)).peer_id,'customer');
 const decision=await write(s,'decide',offer.message.id,{decision:'confirmed',accepted_milli:3000,promised_at:new Date(Date.now()+3600000).toISOString(),reason:'Confirmed partial synthetic delivery'});
 const applied=await write(b,'apply','order',{peer_id:'vendor',envelope:decision.envelope});assert.equal(applied.cycle.decision.external_verified,true);assert.equal(applied.cycle.decision.accepted_milli,3000);
 assert.equal((await b.messages(0,'received','decision')).total,1);
 await write(b,'revoke','vendor',{expected_revision:1,reason:'Synthetic end of relationship'});assert.equal((await b.peers()).items[0].status,'revoked');
 console.log('PASS: 13 API routes; two real Go/Fiber HTTP services and temporary SQLite stores; trust, owner restore review, export, import, decision, application, pagination, receipt recovery and revocation. No commercial data.');
}finally{b.dispose();s.dispose();}
