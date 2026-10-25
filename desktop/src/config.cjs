'use strict';
const {createHash}=require('node:crypto');
const fs=require('node:fs/promises');const path=require('node:path');
const fields=['version','mode','operational_url','online_url'];
function invalid(){throw Error('connection_invalid');}
function endpoint(value,prefix,local=false){
 if(typeof value!=='string'||!value||value!==value.trim()||value.length>2048||/[\x00-\x20\x7f\\]/.test(value))invalid();let u;try{u=new URL(value);}catch{invalid();}
 if(u.username||u.password||u.search||u.hash||u.pathname!==prefix&&u.pathname!==prefix+'/')invalid();
 const loopback=['127.0.0.1','[::1]'].includes(u.hostname);
 if(u.port==='0'||local&&!/^https?:\/\/(127\.0\.0\.1|\[::1\])(?::[1-9]\d{0,4})?\/local\/v1\/?$/.test(value))invalid();
 if(local?(!loopback||!['http:','https:'].includes(u.protocol)):u.protocol!=='https:')invalid();
 // Canonical literal loopback in local mode; no DNS or wildcard fallback.
 if(!local&&u.hostname==='localhost')invalid();
 return u.origin+prefix;
}
function validate(input){
 if(!input||Array.isArray(input)||Object.keys(input).length!==fields.length||!fields.every(k=>Object.hasOwn(input,k))||input.version!==1||!['local','remote','hybrid'].includes(input.mode))invalid();
 const operational_url=endpoint(input.operational_url,'/local/v1',input.mode!=='remote');
 const online_url=input.online_url===null?null:endpoint(input.online_url,'/api/v1');
 if(input.mode==='local'&&online_url!==null||input.mode==='hybrid'&&online_url===null)invalid();
 return Object.freeze({version:1,mode:input.mode,operational_url,online_url});
}
function profile(config){return 'persist:titan-'+createHash('sha256').update(JSON.stringify(validate(config))).digest('hex').slice(0,32);}
function parseConfig(raw){
 // Flat connection schema: preserve duplicate-key detection before JSON.parse.
 const tokens=raw.match(/"(?:[^"\\\x00-\x1f]|\\(?:["\\/bfnrt]|u[0-9a-fA-F]{4}))*"|[{}:,]|null|1|[^\s]/g)||[];
 let n=0;const take=t=>{if(tokens[n++]!==t)invalid();},object=Object.create(null),seen=new Set();take('{');
 while(tokens[n]!=='}'){const key=JSON.parse(tokens[n++]);if(typeof key!=='string'||seen.has(key)||!fields.includes(key))invalid();seen.add(key);take(':');const token=tokens[n++];if(token!=='null'&&token!=='1'&&!token?.startsWith('"'))invalid();object[key]=JSON.parse(token);if(tokens[n]!==',')break;take(',');if(tokens[n]==='}')invalid();}
 take('}');if(n!==tokens.length)invalid();return validate(object);
}
async function readConfig(file){
 try{const s=await fs.lstat(file);if(!s.isFile()||s.isSymbolicLink()||s.size>16384)invalid();const raw=await fs.readFile(file,'utf8');return parseConfig(raw);}catch(e){if(e.code==='ENOENT')return null;throw Error('connection_invalid');}
}
async function writeConfig(file,input){
 const config=validate(input),dir=path.dirname(file);await fs.mkdir(dir,{recursive:true,mode:0o700});
 try{const s=await fs.lstat(file);if(!s.isFile()||s.isSymbolicLink()||s.size>16384)invalid();const backup=file+'.previous.'+require('node:crypto').randomUUID()+'.json';await fs.copyFile(file,backup,require('node:fs').constants.COPYFILE_EXCL);await fs.chmod(backup,0o600);}catch(e){if(e.code!=='ENOENT')throw e;}
 const tmp=file+'.'+require('node:crypto').randomUUID()+'.tmp';let created=false;
 try{const h=await fs.open(tmp,'wx',0o600);created=true;try{await h.writeFile(JSON.stringify(config,null,2)+'\n');await h.sync();}finally{await h.close();}await fs.rename(tmp,file);created=false;}finally{if(created)await fs.unlink(tmp).catch(()=>{});}return config;
}
module.exports={validate,endpoint,profile,parseConfig,readConfig,writeConfig};
