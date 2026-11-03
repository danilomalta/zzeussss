'use strict';
const fs=require('node:fs/promises');const path=require('node:path');const {ownURL}=require('./transport.cjs');
const mime={'.html':'text/html;charset=utf-8','.js':'text/javascript;charset=utf-8','.mjs':'text/javascript;charset=utf-8','.css':'text/css;charset=utf-8','.json':'application/json;charset=utf-8','.svg':'image/svg+xml','.png':'image/png','.jpg':'image/jpeg','.webp':'image/webp','.ico':'image/x-icon','.woff':'font/woff','.woff2':'font/woff2','.ttf':'font/ttf','.wasm':'application/wasm'};
const csp="default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob: https:; font-src 'self' data:; connect-src 'self' https://servicodados.ibge.gov.br; worker-src 'self' blob:; media-src 'self' blob:; object-src 'none'; base-uri 'none'; frame-src 'none'; frame-ancestors 'none'; form-action 'self'";
function plain(status,text){return new Response(text,{status,headers:{'Content-Type':'text/plain;charset=utf-8','Cache-Control':'no-store'}});}
function createAssets(root,setupRoot){return async request=>{
 if(!ownURL(request.url))return plain(403,'Origem recusada.');if(!['GET','HEAD'].includes(request.method))return plain(405,'Método recusado.');
 let pathname;try{pathname=decodeURIComponent(new URL(request.url).pathname);}catch{return plain(400,'Caminho inválido.');}
 if(/[\\\x00-\x1f\x7f]/.test(pathname)||pathname.split('/').some(p=>p==='..'||p==='.')||pathname.includes('//'))return plain(400,'Caminho inválido.');
 const setup=pathname.startsWith('/desktop-setup/'),base=setup?setupRoot:root;
 if(!base)return plain(503,'Interface compilada indisponível.');
 let relative=setup?pathname.slice('/desktop-setup/'.length):pathname.slice(1);
 if(setup&&!['index.html','setup.js','setup.css','words.js'].includes(relative))return plain(404,'Recurso indisponível.');
 if(!setup){if(relative===''||relative==='local'||relative.startsWith('local/')||relative==='login'||relative==='registration'||relative==='pos'||relative==='checkout'||relative==='admin'||relative==='supply-chain'){
   if(!path.extname(relative))relative='index.html';
  }}
 try{const realRoot=await fs.realpath(base),target=await fs.realpath(path.resolve(base,relative));if(!target.startsWith(realRoot+path.sep)||!mime[path.extname(target)])return plain(403,'Recurso recusado.');const info=await fs.stat(target);if(!info.isFile()||info.size>32*1024*1024)return plain(404,'Recurso indisponível.');const headers={'Content-Type':mime[path.extname(target)],'Content-Security-Policy':csp,'Cache-Control':'no-store','X-Content-Type-Options':'nosniff','Referrer-Policy':'no-referrer'};return new Response(request.method==='HEAD'?null:await fs.readFile(target),{headers});}catch{return plain(404,'Recurso indisponível.');}
};}
module.exports={createAssets,csp};
