'use strict';const {spawnSync}=require('node:child_process');const fs=require('node:fs');const path=require('node:path');
const root=path.resolve(__dirname,'..');const build=spawnSync(process.execPath,[path.join(__dirname,'build.mjs')],{stdio:'inherit'});if(build.status!==0)process.exit(build.status||1);
const current=JSON.parse(fs.readFileSync(path.join(root,'.build/current.json'),'utf8'));if(!/^app-[0-9a-f-]+$/.test(current.stage))throw Error('Invalid build stage');
const stage=path.join(root,'.build',current.stage);let electron=process.env.TITAN_ELECTRON_BINARY;
if(!electron){try{electron=require('electron');}catch{console.error('Electron não está instalado nesta pasta. Informe TITAN_ELECTRON_BINARY com o caminho absoluto de uma instalação existente. Nenhuma dependência foi instalada.');process.exit(1);}}
if(!path.isAbsolute(electron)||!fs.statSync(electron).isFile())throw Error('Invalid Electron binary');
const env={...process.env};delete env.ELECTRON_RUN_AS_NODE;const result=spawnSync(electron,[stage],{stdio:'inherit',env});if(result.error)throw result.error;process.exitCode=result.status??1;
