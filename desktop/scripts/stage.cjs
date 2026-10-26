'use strict';
const fs=require('node:fs');const path=require('node:path');const {createHash}=require('node:crypto');const {spawnSync}=require('node:child_process');
const hash=value=>createHash('sha256').update(value).digest('hex');
const required=['main.cjs','preload.cjs','src/config.cjs','src/transport.cjs','src/assets.cjs','package.json','setup/index.html','setup/setup.js','setup/setup.css','setup/words.js','ui/index.html'];
function invalid(){throw Error('Interface compilada ausente ou inválida. Execute npm run build na pasta desktop; os fontes e os dados foram preservados.');}
function object(value){return value&&typeof value==='object'&&!Array.isArray(value);}
function readJSON(file,limit){const info=fs.lstatSync(file);if(!info.isFile()||info.isSymbolicLink()||info.size>limit)invalid();return JSON.parse(fs.readFileSync(file,'utf8'));}
function files(root,prefix=''){const out=[];for(const entry of fs.readdirSync(path.join(root,prefix),{withFileTypes:true})){const name=prefix?prefix+'/'+entry.name:entry.name;if(entry.isSymbolicLink())invalid();if(entry.isDirectory())out.push(...files(root,name));else if(entry.isFile())out.push(name);else invalid();}return out;}
function resolveStage(root){
 try{
  const builds=path.join(root,'.build');const info=fs.lstatSync(builds);if(!info.isDirectory()||info.isSymbolicLink())invalid();
  const current=readJSON(path.join(builds,'current.json'),16384);
  if(!object(current)||current.version!==1||!/^app-[0-9a-f-]+$/.test(current.stage)||!/^[0-9a-f]{64}$/.test(current.source_hash))invalid();
  const stage=path.join(builds,current.stage),stat=fs.lstatSync(stage);if(!stat.isDirectory()||stat.isSymbolicLink())invalid();
  const manifest=readJSON(path.join(stage,'build-manifest.json'),4*1024*1024);
  if(!object(manifest)||manifest.version!==1||manifest.source_hash!==current.source_hash||!object(manifest.files))invalid();
  const names=Object.keys(manifest.files);if(!names.length||names.length>10000||required.some(name=>!Object.hasOwn(manifest.files,name)))invalid();
  for(const name of names){if(!name||name.startsWith('/')||/[\\:\x00-\x1f]/.test(name)||name.split('/').some(p=>!p||p==='.'||p==='..'||p==='node_modules'||/^\.env(?:\.|$)/.test(p))||!/^[0-9a-f]{64}$/.test(manifest.files[name]))invalid();}
  const actual=files(stage).filter(name=>name!=='build-manifest.json').sort();if(JSON.stringify(actual)!==JSON.stringify(names.slice().sort()))invalid();
  // Check every compiled output, not the developer's mutable source tree.
  for(const name of names){const file=path.join(stage,name);if(fs.statSync(file).size>64*1024*1024||hash(fs.readFileSync(file))!==manifest.files[name])invalid();}
  return stage;
 }catch{invalid();}
}
function startDesktop(args=[],options={}){
 if(args.length>1||args.length===1&&args[0]!=='--build')throw Error('Uso: npm start [-- --build]. A compilação só ocorre quando solicitada.');
 const root=options.root||path.resolve(__dirname,'..');const spawn=options.spawn||spawnSync;
 if(args[0]==='--build'){const build=spawn(process.execPath,[path.join(root,'scripts/build.mjs')],{cwd:root,stdio:'inherit'});if(build.error)throw build.error;if(build.status!==0)return build.status||1;}
 const stage=resolveStage(root),environment=options.environment||process.env;
 let electron=options.electron||environment.TITAN_ELECTRON_BINARY;
 if(!electron){try{electron=require('electron');}catch{throw Error('Electron indisponível. Informe TITAN_ELECTRON_BINARY com uma instalação existente. Nenhuma dependência foi instalada.');}}
 if(typeof electron!=='string'||!path.isAbsolute(electron)||!fs.statSync(electron).isFile())throw Error('Caminho do Electron inválido.');
 const env={...environment};delete env.ELECTRON_RUN_AS_NODE;
 const child=spawn(electron,[stage],{cwd:root,stdio:'inherit',env});if(child.error)throw child.error;return child.status??1;
}
module.exports={resolveStage,startDesktop};
