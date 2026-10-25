'use strict';
const {validate}=require('./config.cjs');
const APP='titan://app';
const requestHeaders=['authorization','content-type','accept','idempotency-key','x-request-id'];
const responseHeaders=['content-type','content-disposition','x-content-type-options','cache-control','retry-after'];
function ownURL(value){try{const u=new URL(value);return u.protocol==='titan:'&&u.hostname==='app'&&!u.port&&!u.username&&!u.password;}catch{return false;}}
function route(value,config){
 if(!ownURL(value))throw Error('origin_refused');const u=new URL(value);let base,prefix,kind;
 if(u.pathname==='/local/v1'||u.pathname.startsWith('/local/v1/')){base=config?.operational_url;prefix='/local/v1';kind='operational';}
 else if(u.pathname==='/api/v1'||u.pathname.startsWith('/api/v1/')){base=config?.online_url;prefix='/api/v1';kind='online';}
 else return null;
 if(!base)throw Error('service_not_configured');validate(config);return {target:base+u.pathname.slice(prefix.length)+u.search,kind};
}
function error(status,code){return new Response(JSON.stringify({code}),{status,headers:{'Content-Type':'application/json;charset=utf-8','Cache-Control':'no-store','X-Content-Type-Options':'nosniff'}});}
async function limitedBody(request,max){if(!request.body)return undefined;const reader=request.body.getReader(),parts=[];let n=0;try{for(;;){const {value,done}=await reader.read();if(done)break;n+=value.byteLength;if(n>max){await reader.cancel();throw Error('body_limit');}parts.push(value);}}finally{reader.releaseLock();}const body=new Uint8Array(n);let at=0;for(const v of parts){body.set(v,at);at+=v.byteLength;}return body;}
function createTransport(config,fetcher){if(config)validate(config);return async request=>{
 let destination;try{destination=route(request.url,config);}catch(e){return error(e.message==='service_not_configured'?503:403,e.message);}
 if(!destination)return null;
 if(request.initiatorOrigin&&request.initiatorOrigin!==APP)return error(403,'origin_refused');
 if(!['GET','HEAD','POST','PUT','PATCH','DELETE'].includes(request.method))return error(405,'method_refused');
 const headers=new Headers();for(const name of requestHeaders){const value=request.headers.get(name);if(value!==null)headers.set(name,value);}headers.set('X-Client-Platform','desktop');
 let dispatched=false;const abort=new AbortController(),cancel=()=>abort.abort();request.signal?.addEventListener('abort',cancel,{once:true});if(request.signal?.aborted)cancel();const timer=setTimeout(cancel,20000);
 try{
  const body=await limitedBody(request,1024*1024);if(abort.signal.aborted)throw Error('aborted');
  dispatched=true;const response=await fetcher(destination.target,{method:request.method,headers,body,credentials:destination.kind==='online'?'include':'omit',redirect:'manual',cache:'no-store',signal:abort.signal});
  if(response.status>=300&&response.status<400){await response.body?.cancel();return error(502,'redirect_refused');}
  const out=new Headers();for(const name of responseHeaders){const v=response.headers.get(name);if(v!==null)out.set(name,v);}out.set('Cache-Control','no-store');out.set('X-Content-Type-Options','nosniff');out.set('Content-Security-Policy',"default-src 'none'; frame-ancestors 'none'");
  // Do not expose service cookies, server location or HTML as application code.
  if(/(?:text\/html|javascript)/i.test(out.get('content-type')||'')){await response.body?.cancel();return error(502,'service_response_refused');}
  if(request.method==='HEAD'||[204,205,304].includes(response.status))return new Response(null,{status:response.status,headers:out});
  // Bound buffering so timeouts cover the whole response, not only its headers.
  const data=await limitedBody(response,32*1024*1024);return new Response(data,{status:response.status,headers:out});
 }catch(e){return error(e.message==='body_limit'&&!dispatched?413:502,e.message==='body_limit'?(dispatched?'response_limit':'body_limit'):'service_unavailable');}
 finally{clearTimeout(timer);request.signal?.removeEventListener('abort',cancel);}
};}
module.exports={APP,ownURL,route,createTransport,limitedBody};
