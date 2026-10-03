import test from 'node:test';
import assert from 'node:assert/strict';
import {allowedAreas,areaForRoute} from '../src/core/local/accessModel.mjs';
import {createLocalClient} from '../src/core/local/localClient.mjs';
const scope={tenant_id:'tenant',store_id:'store',device_id:'device',identity_id:'person'};
const json=body=>new Response(JSON.stringify(body));
const caps=(modules,permissions,role='owner')=>({...scope,role,permissions,license:{state:'active',modules,remaining_days:2,expires_unix:Date.now()/1000|0}});
test('production selection does not grant POS or RH even to the owner',()=>{
 const areas=allowedAreas(caps(['core','inventory','production'],['sell','view_catalog','manage_stock','manage_production','manage_staff']));
 assert.ok(areas.includes('production'));assert.ok(areas.includes('stock'));assert.ok(!areas.includes('pos'));assert.ok(!areas.includes('staff'));
});
test('cashier cannot see stock mutation or HR management while own point stays distinct',()=>{
 const areas=allowedAreas(caps(['core','inventory','pos','staff'],['view_catalog','sell'],'cashier'));
 assert.ok(areas.includes('pos'));assert.ok(areas.includes('catalog'));assert.ok(areas.includes('point'));assert.ok(!areas.includes('staff'));assert.ok(!areas.includes('stock'));assert.ok(!areas.includes('plans'));
});
test('employee and accountant never gain modules from the company size',()=>{
 const employee=allowedAreas(caps(['core','staff'],[],'employee'));assert.deepEqual(employee,['home','point']);
 const accountant=allowedAreas(caps(['core','accounting'],['view_accounting'],'accountant'));assert.ok(accountant.includes('accounting'));assert.ok(!accountant.includes('pos'));assert.ok(!accountant.includes('staff'));
 assert.equal(areaForRoute('/local/stock'),'stock');assert.equal(areaForRoute('/local/staff'),'staff');
});
test('invalid or missing trust fails closed for module navigation',()=>{
 const forged={...caps(['core','inventory','pos'],['sell']),license:{state:'invalid',modules:['pos'],remaining_days:0}};
 assert.ok(!allowedAreas(forged).includes('pos'));assert.deepEqual(allowedAreas(null),['home']);
});
test('client rejects foreign capabilities and unexpected permissions',async()=>{
 const valid=caps(['core','staff'],['manage_staff']);
 assert.deepEqual(await createLocalClient(async()=>json(valid)).capabilities('token',scope),valid);
 await assert.rejects(createLocalClient(async()=>json({...valid,identity_id:'foreign'})).capabilities('token',scope));
 await assert.rejects(createLocalClient(async()=>json({...valid,permissions:['platform_admin']})).capabilities('token',scope));
});
test('employee creation never allows owner assignment or fractional/unscoped input',async()=>{
 const calls=[];const client=createLocalClient(async(url,o)=>{calls.push(JSON.parse(o.body));return json({identity_id:'new',repeated:false});});
 const input={operation_id:'op',identity_id:'new',name:'Pessoa',role:'employee',password:'strong-password-123'};
 await client.createStaff('token',input);assert.deepEqual(calls[0],input);
 await assert.rejects(client.createStaff('token',{...input,role:'owner'}));assert.equal(calls.length,1);
 await assert.rejects(client.createStaff('token',{...input,password:'short'}));assert.equal(calls.length,1);
});
