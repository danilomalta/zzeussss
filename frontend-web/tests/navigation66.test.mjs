import test from 'node:test';
import assert from 'node:assert/strict';
import {logoDestination} from '../src/core/local/navigationDestination.mjs';
test('logo autenticado local retorna ao menu em todas as áreas sem mudar de sessão',()=>{
 for(const pathname of ['/local/pos','/local/cash','/local/hr','/local/settings','/local/overview','/local/map','/local/home'])assert.equal(logoDestination({pathname,localActive:true,onlineActive:false}),'/local/home');
});
test('contextos online e local não são trocados quando há duas sessões',()=>{
 assert.equal(logoDestination({pathname:'/local/pos',localActive:true,onlineActive:true}),'/local/home');
 assert.equal(logoDestination({pathname:'/pos',localActive:true,onlineActive:true}),'/');
 assert.equal(logoDestination({pathname:'/local/catalog',localActive:false,onlineActive:true}),'/local/login');
});
test('sem sessão ativa o logo continua levando ao acesso público, não a área protegida',()=>{
 assert.equal(logoDestination({pathname:'/local/login',localActive:false,onlineActive:false}),'/local/login');
 assert.equal(logoDestination({pathname:'/register',localActive:true,onlineActive:false}),'/local/home');
 assert.throws(()=>logoDestination({pathname:null,localActive:true,onlineActive:false}));
});
