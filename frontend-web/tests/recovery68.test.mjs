import test from 'node:test';
import assert from 'node:assert/strict';
import {diagnostic68,localPath68,moveFocus68,protectRoutes68,recovery68,release68} from '../src/core/recovery68/model.mjs';
import {words68} from '../src/core/recovery68/words.mjs';

test('diagnostic uses an explicit allowlist even if the error contains private data',()=>{
 const info={...recovery68('/local/customer-returns?token=secret'),token:'bearer-private',stack:'private-path',operation_id:'private-id',error:Error('password-private')};
 const result=JSON.parse(diagnostic68(info,new Date('2026-10-26T12:00:00Z')));
 assert.deepEqual(result,{release:release68,code:'UI68-RENDER',area:'local',at:'2026-10-26T12:00:00.000Z'});
 assert.doesNotMatch(JSON.stringify(result),/secret|private|password|token|customer/);
});
test('malformed diagnostics are refused',()=>{
 for(const info of [null,{code:'UI68-404',area:'foreign'},{code:'Authorization:secret',area:'local'}])assert.throws(()=>diagnostic68(info));
 assert.throws(()=>diagnostic68(recovery68('/'),new Date('invalid')));
});
test('local recovery is restricted to the actual path prefix',()=>{
 for(const path of ['/local','/local/home','/local/pos','/local/'])assert.equal(localPath68(path),true);
 for(const path of ['/locality','/local-evil','https://other/local/pos',null,undefined,5])assert.equal(localPath68(path),false);
 assert.equal(recovery68('/local/pos').home,'/local/home');
 assert.equal(recovery68('/checkout').home,'/');
 assert.equal(recovery68('/local/unknown',true).code,'UI68-404');
});
test('all nested routes retain guards, loaders and actions unchanged',()=>{
 const guard={},loader=()=>{},action=()=>{},existing={},fallback={};
 const routes=[{id:'local',path:'/local',element:guard,loader,action,handle:{permission:'sell'},children:[{index:true,element:{}},{path:'pos',element:guard,errorElement:existing},{path:'orders',children:[{path:':id',element:{}}]}]}];
 const originalChildren=routes[0].children,result=protectRoutes68(routes,fallback);
 assert.equal(result[0].element,guard);assert.equal(result[0].loader,loader);assert.equal(result[0].action,action);assert.equal(result[0].handle,routes[0].handle);
 assert.equal(result[0].children[0].errorElement,fallback);assert.equal(result[0].children[1].errorElement,existing);assert.equal(result[0].children[2].children[0].errorElement,fallback);
 assert.equal(result[0].id,'local');assert.equal(result[0].children[0].index,true);
 assert.equal(routes[0].errorElement,undefined);assert.equal(routes[0].children,originalChildren);
});
test('a lazy placeholder does not receive navigation focus',()=>{
 assert.equal(moveFocus68({querySelector:()=>null},null,null,{}),false);
 assert.equal(moveFocus68(null,null,null,{}),false);
});
function heading(tab=null){return {tab,focused:0,getAttribute(){return this.tab;},setAttribute(_,v){this.tab=v;},removeAttribute(){this.tab=null;},focus(options){assert.equal(options.preventScroll,true);this.focused++;assert.equal(this.tab,'-1');}};}
test('navigation focuses the actual heading and restores its original tab order',()=>{
 for(const original of [null,'0','-1']){const h=heading(original),active={},container={querySelector:s=>{assert.equal(s,'h1');return h;}};
  assert.equal(moveFocus68(container,active,active,{}),true);assert.equal(h.focused,1);assert.equal(h.tab,original);
 }
});
test('focus already moved by the user is preserved',()=>{
 const h=heading();assert.equal(moveFocus68({querySelector:()=>h},{},{},{}),false);assert.equal(h.focused,0);
});
test('body focus after an old view unmounts permits the new heading',()=>{
 const h=heading(),body={};assert.equal(moveFocus68({querySelector:()=>h},{},body,body),true);
});
test('failed focus still restores tabindex',()=>{
 const h=heading('0');h.focus=()=>{throw Error('detached');};assert.equal(moveFocus68({querySelector:()=>h},null,null,{}),false);assert.equal(h.tab,'0');
});
test('five dictionaries have matching nonempty keys, regional aliases and immutable text',()=>{
 const keys=Object.keys(words68('pt-BR')).sort();
 for(const locale of ['pt-BR','en-US','es-ES','fr-FR','zh-CN']){const w=words68(locale);assert.deepEqual(Object.keys(w).sort(),keys);for(const v of Object.values(w))assert.ok(typeof v==='string'&&v.trim());assert.ok(Object.isFrozen(w));}
 assert.equal(words68('en-GB'),words68('en'));assert.equal(words68('unknown'),words68('pt'));
});
