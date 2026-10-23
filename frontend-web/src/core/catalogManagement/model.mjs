const fail=()=>{throw Error('catalogInvalid');};
const bytes=v=>new TextEncoder().encode(v).length;
const exact=(v,keys)=>v&&typeof v==='object'&&!Array.isArray(v)&&Object.keys(v).length===keys.length&&keys.every(k=>Object.hasOwn(v,k));
const revision=(v,max=2147483647)=>Number.isSafeInteger(v)&&v>=0&&v<=max;
export const catalogID=v=>typeof v==='string'&&v===v.trim()&&bytes(v)>=1&&bytes(v)<=128&&!/[\x00\r\n]/.test(v);
export const catalogUnits=['unit','kg','g','liter','ml','meter'];
export const catalogEqual=(a,b)=>canonical(a)===canonical(b);
function canonical(v){return JSON.stringify(Array.isArray(v)?v.map(x=>JSON.parse(canonical(x))):v&&typeof v==='object'?Object.fromEntries(Object.keys(v).sort().map(k=>[k,JSON.parse(canonical(v[k]))])):v);}
const productKeys=['sku','name','unit','barcode','price_cents','cost_cents'];
export function catalogProduct(v){if(!exact(v,productKeys)||!catalogUnits.includes(v.unit)||!['sku','name','barcode'].every(k=>typeof v[k]==='string'&&v[k]===v[k].trim()&&bytes(v[k])<={sku:100,name:255,barcode:128}[k]&&(k==='barcode'||v[k].length>0))||!['price_cents','cost_cents'].every(k=>Number.isSafeInteger(v[k])&&v[k]>=0))fail();return v;}
export function catalogDetails(v,id){if(!catalogID(id)||!exact(v,['product_id','revision','product'])||v.product_id!==id||!revision(v.revision))fail();catalogProduct(v.product);return v;}
export function catalogState(v,id){if(!catalogID(id)||!exact(v,['product_id','revision','status','repeated'])||v.product_id!==id||!revision(v.revision)||!['active','inactive'].includes(v.status)||typeof v.repeated!=='boolean'||v.revision===0&&v.status!=='active')fail();return v;}
export function catalogCents(text){if(typeof text!=='string'||!/^\d{1,16}(?:[.,]\d{1,2})?$/.test(text))fail();const [whole,part='']=text.split(/[.,]/),value=BigInt(whole)*100n+BigInt(part.padEnd(2,'0'));if(value>BigInt(Number.MAX_SAFE_INTEGER))fail();return Number(value);}
export function catalogMoneyInput(value){if(!Number.isSafeInteger(value)||value<0)fail();const v=BigInt(value);return String(v/100n)+'.'+String(v%100n).padStart(2,'0');}
export function catalogInput(kind,before,body,id){
 if(!catalogID(id)||!['edit','state'].includes(kind))fail();
 const keys=kind==='edit'?['operation_id','expected_revision',...productKeys]:['operation_id','expected_revision','status','reason'];
 if(!exact(body,keys)||!catalogID(body.operation_id)||!revision(body.expected_revision,2147483646))fail();
 if(kind==='edit'){catalogDetails(before,id);catalogProduct(Object.fromEntries(productKeys.map(k=>[k,body[k]])));if(catalogEqual(before.product,Object.fromEntries(productKeys.map(k=>[k,body[k]]))))fail();}
 else{catalogState(before,id);if(!['active','inactive'].includes(body.status)||body.status===before.status||typeof body.reason!=='string'||body.reason!==body.reason.trim()||bytes(body.reason)<1||bytes(body.reason)>255||/[\x00\r\n]/.test(body.reason))fail();}
 if(before.revision!==body.expected_revision||bytes(JSON.stringify(body))>4096)fail();return body;
}
export function catalogCommand(v,scope){if(!exact(v,['version','scope','kind','product_id','before','body','uncertain'])||v.version!==1||v.scope!==scope||typeof scope!=='string'||!scope||typeof v.uncertain!=='boolean')fail();catalogInput(v.kind,v.before,v.body,v.product_id);return v;}
export function catalogResult(v,command,original=false){const {product_id:id,body,kind}=command;
 if(kind==='state')catalogState(v,id);else if(!exact(v,['product_id','revision','repeated'])||v.product_id!==id||!revision(v.revision)||typeof v.repeated!=='boolean')fail();
 if(v.revision!==body.expected_revision+1||kind==='state'&&v.status!==body.status||original&&v.repeated!==false)fail();return v;
}
export function catalogReceipt(v,command){if(!exact(v,['kind','input','result'])||v.kind!==command.kind||!catalogEqual(v.input,{...command.body,product_id:command.product_id}))fail();catalogResult(v.result,command,true);return v;}
function time(s){if(typeof s!=='string')return false;const m=/^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2}):(\d{2})(?:\.\d{1,9})?(?:Z|[+-](\d{2}):(\d{2}))$/.exec(s);if(!m)return false;const y=+m[1],month=+m[2],day=+m[3],leap=y%4===0&&(y%100!==0||y%400===0);return y>0&&month>0&&month<=12&&day>0&&day<=[31,leap?29:28,31,30,31,30,31,31,30,31,30,31][month-1]&&+m[4]<24&&+m[5]<60&&+m[6]<60&&(!m[7]||+m[7]<24&&+m[8]<60)&&Number.isFinite(Date.parse(s));}
export function catalogHistory(v,id,offset=0){
 if(!Number.isSafeInteger(offset)||offset<0||!exact(v,['current','offset','limit','total_count','has_more','items']))fail();catalogState(v.current,id);
 if(v.offset!==offset||v.limit!==50||v.total_count!==v.current.revision||v.has_more!==(offset+Math.min(50,Math.max(0,v.total_count-offset))<v.total_count)||!Array.isArray(v.items)||v.items.length!==Math.min(50,Math.max(0,v.total_count-offset)))fail();
 const seen=new Set();for(let i=0;i<v.items.length;i++){const row=v.items[i];if(!exact(row,['revision','store_id','operation_id','device_id','actor_id','before_status','after_status','reason','created_at'])||row.revision!==offset+i+1||!['store_id','operation_id','device_id','actor_id'].every(k=>catalogID(row[k]))||!['active','inactive'].includes(row.before_status)||row.after_status!==(row.before_status==='active'?'inactive':'active')||row.before_status!==(row.revision%2?'active':'inactive')||typeof row.reason!=='string'||row.reason!==row.reason.trim()||bytes(row.reason)<1||bytes(row.reason)>255||/[\x00\r\n]/.test(row.reason)||!time(row.created_at)||seen.has(JSON.stringify([row.device_id,row.operation_id])))fail();seen.add(JSON.stringify([row.device_id,row.operation_id]));}
 if(v.current.status!==(v.current.revision%2?'inactive':'active'))fail();return v;
}
