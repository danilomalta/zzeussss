import test from 'node:test';
import assert from 'node:assert/strict';
import {watchProductionQueue} from '../src/core/productionRead/queueWatch.mjs';
function fixture(){
 const target=new EventTarget(),visibility=new EventTarget();visibility.visibilityState='visible';
 let value=null,fail=false,reads=0,errors=0;const values=[];
 const source={scope:'tenant:store:device:user',pending(){reads++;if(fail)throw Error('private-data');return structuredClone(value);}};
 const stop=watchProductionQueue({source,target,visibility,onChange:v=>values.push(v),onError:()=>errors++});
 const event=(type,key)=>{const e=new Event(type);Object.defineProperty(e,'key',{value:key});Object.defineProperty(e,'newValue',{value:'untrusted event body'});target.dispatchEvent(e);};
 return {target,visibility,values,source,stop,event,set:v=>{value=v;},fail:v=>{fail=v;},reads:()=>reads,errors:()=>errors,key:'titan-production-commands:v1:'+source.scope};
}
test('initial mount reads its own queue without a write, request or retry',()=>{
 const f=fixture();assert.equal(f.reads(),1);assert.deepEqual(f.values,[null]);assert.equal(f.errors(),0);f.stop();
});
test('another tab updates pending state and clearing it without event payload trust',()=>{
 const f=fixture(),command={version:1,scope:f.source.scope,kind:'result',uncertain:true,body:{operation_id:'stable-id'}};
 f.set(command);f.event('storage',f.key);assert.deepEqual(f.values.at(-1),command);assert.equal(f.values.length,2);
 f.set(null);f.event('storage',f.key);assert.equal(f.values.at(-1),null);assert.equal(f.values.length,3);f.stop();
});
test('events for other identities and unrelated storage are ignored',()=>{
 const f=fixture();for(const key of ['titan-production-commands:v1:foreign','other',undefined])f.event('storage',key);assert.equal(f.reads(),1);assert.deepEqual(f.values,[null]);f.stop();
});
test('storage clear rechecks the actual queue and does not erase its own data',()=>{
 const f=fixture();f.set({uncertain:true});f.event('storage',null);assert.equal(f.values.at(-1).uncertain,true);f.stop();
});
test('focus and restored page recheck but unchanged values do not repeat callbacks',()=>{
 const f=fixture();f.event('focus');f.event('pageshow');assert.equal(f.reads(),3);assert.equal(f.values.length,1);f.set({uncertain:true});f.event('pageshow');assert.equal(f.values.length,2);f.event('focus');assert.equal(f.values.length,2);f.stop();
});
test('only returning to a visible document causes a visibility read',()=>{
 const f=fixture();f.visibility.visibilityState='hidden';f.visibility.dispatchEvent(new Event('visibilitychange'));assert.equal(f.reads(),1);f.visibility.visibilityState='visible';f.visibility.dispatchEvent(new Event('visibilitychange'));assert.equal(f.reads(),2);f.stop();
});
test('corrupt storage or stale authorization reports an error without clearing pending',()=>{
 const f=fixture();f.set({uncertain:true});f.event('storage',f.key);f.fail(true);f.event('storage',f.key);assert.equal(f.errors(),1);assert.equal(f.values.at(-1).uncertain,true);f.event('focus');assert.equal(f.errors(),1);f.fail(false);f.event('focus');assert.equal(f.values.at(-1).uncertain,true);assert.equal(f.values.length,3);f.stop();
});
test('unmount removes every listener, including focus, storage and visibility',()=>{
 const f=fixture();f.stop();f.event('storage',f.key);f.event('focus');f.event('pageshow');f.visibility.dispatchEvent(new Event('visibilitychange'));assert.equal(f.reads(),1);assert.equal(f.values.length,1);f.stop();
});
test('a replacement subscription never publishes events under the previous scope',()=>{
 const f=fixture();f.stop();const values=[];const stop=watchProductionQueue({source:{scope:'second',pending:()=>null},target:f.target,visibility:f.visibility,onChange:v=>values.push(v),onError:()=>assert.fail('unexpected error')});f.event('storage',f.key);assert.deepEqual(values,[null]);stop();
});
test('invalid subscription arguments fail before registering a listener',()=>{
 assert.throws(()=>watchProductionQueue({source:{scope:'',pending:()=>null},target:new EventTarget(),onChange(){},onError(){}}),/productionInvalid/);
});
