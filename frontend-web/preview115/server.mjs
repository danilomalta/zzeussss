// Explicit demo transport. Never imported by the production route.
export function demo115(){
 const products=[{id:'flour',name:'Farinha de trigo',unit:'kg',balance_milli:2500,minimum_milli:5000,target_milli:20000,revision:1},{id:'rice',name:'Arroz 5 kg',unit:'unit',balance_milli:1000,minimum_milli:0,target_milli:0,revision:0}],suppliers=[{id:'central',name:'Distribuidora Central',status:'active'},{id:'closed',name:'Fornecedor inativo',status:'inactive'}],suggestions=[{id:'first',product_id:'flour',name:'Farinha de trigo',unit:'kg',observed_milli:2500,recommended_milli:17500,policy_revision:1,status:'suggested',stale:false,reason:'',order_id:'',created_at:'2026-10-20T10:00:00Z'},{id:'stale',product_id:'rice',name:'Arroz 5 kg',unit:'unit',observed_milli:0,recommended_milli:10000,policy_revision:1,status:'suggested',stale:true,reason:'',order_id:'',created_at:'2026-10-19T10:00:00Z'}];
 const orders=[],operations=new Map(),supplierOps=new Map(),data=new Map(),calls=[];let n=0;
 const flags={error:false,lost:false,empty:false};
 const storage={getItem:k=>data.get(k)??null,setItem:(k,v)=>data.set(k,v),removeItem:k=>data.delete(k)};
 const locks={request:async(name,options,fn)=>fn({name})};
 const reply=(v,status=200)=>new Response(JSON.stringify(v),{status});
 const fetcher=async(url,options)=>{calls.push({url,method:options.method,body:options.body});await new Promise(resolve=>setTimeout(resolve,20));if(flags.error)return reply({},503);const u=new URL(url,'http://preview.invalid'),path=u.pathname.replace('/local/v1',''),offset=Number(u.searchParams.get('offset')||0),input=options.body?JSON.parse(options.body):null;
 const approvals=()=>suggestions.filter(v=>v.status==='approved'&&!v.order_id&&!v.stale).map(v=>({suggestion_id:v.id,product_id:v.product_id,name:v.name,unit:v.unit,quantity_milli:v.recommended_milli}));
 if(options.method==='GET'){
  const lists={'/purchase-suppliers':suppliers,'/purchase-orders':orders.map(o=>({...o,items:[]})),'/purchase-approvals':approvals(),'/replenishment/products':products,'/replenishment/suggestions':suggestions};
  if(lists[path])return reply({items:flags.empty?[]:lists[path].slice(offset,offset+50)});
  if(path.startsWith('/purchase-orders/')){const order=orders.find(o=>o.id===decodeURIComponent(path.split('/').at(-1)));return order?reply(order):reply({},404);}
  if(path.startsWith('/replenishment/operations/')){const v=operations.get(path.split('/').slice(-2).join('/'));return v?reply(v):reply({},404);}
  return reply({},404);
 }
 let result;
 if(path==='/purchase-suppliers'){if(supplierOps.has(input.operation_id))return reply({id:input.id,repeated:true});suppliers.unshift({id:input.id,name:input.name,status:input.status});supplierOps.set(input.operation_id,input);result={id:input.id,repeated:false};}
 else if(path==='/purchase-orders'){const previous=orders.find(o=>o.operation_id===input.operation_id);if(previous)return reply({id:previous.id,repeated:true});const supplier=suppliers.find(v=>v.id===input.supplier_id&&v.status==='active'),s=suggestions.find(v=>v.id===input.suggestion_id&&v.status==='approved'&&!v.order_id&&!v.stale);if(!supplier||!s)return reply({},409);orders.unshift({id:input.order_id,operation_id:input.operation_id,supplier_id:supplier.id,supplier_name:supplier.name,suggestion_id:s.id,status:'local_not_sent',created_at:new Date().toISOString(),approved_by:'demo-manager',approved_at:s.created_at,items:[{product_id:s.product_id,sku:'DEMO-'+s.product_id,name:s.name,unit:s.unit,quantity_milli:s.recommended_milli}]});s.order_id=input.order_id;result={id:input.order_id,repeated:false};}
 else {
  const kind={'/replenishment/policies':'policy','/replenishment/suggestions':'suggest','/replenishment/reviews':'review'}[path];if(!kind)return reply({},404);const key=kind+'/'+input.operation_id;if(operations.has(key))return reply({...operations.get(key).result,repeated:true});const p=products.find(v=>v.id===input.product_id);
  if(kind==='policy'){if(!p)return reply({},404);p.minimum_milli=input.minimum_milli;p.target_milli=input.target_milli;p.revision++;result={operation_id:input.operation_id,revision:p.revision,repeated:false};}
  if(kind==='suggest'){if(!p?.revision)return reply({code:'missing_policy'},409);const amount=p.balance_milli<p.minimum_milli?p.target_milli-p.balance_milli:0,id='demo-suggestion-'+(++n);suggestions.unshift({id,product_id:p.id,name:p.name,unit:p.unit,observed_milli:p.balance_milli,recommended_milli:amount,policy_revision:p.revision,status:amount?'suggested':'not_needed',stale:false,reason:'',order_id:'',created_at:new Date().toISOString()});result={operation_id:input.operation_id,suggestion_id:id,observed_milli:p.balance_milli,recommended_milli:amount,policy_revision:p.revision,needed:amount>0,repeated:false};}
  if(kind==='review'){const s=suggestions.find(v=>v.id===input.suggestion_id);if(!s||s.status!=='suggested'||s.stale&&input.decision==='approved')return reply({code:'stale_suggestion'},409);s.status=input.decision;s.reason=input.reason;result={operation_id:input.operation_id,suggestion_id:s.id,decision:s.status,repeated:false};}
  operations.set(key,{kind,input,result});
 }
 if(flags.lost){flags.lost=false;throw Error('demo lost reply after commit');}return reply(result,201);
 };
 return {fetcher,storage,locks,flags,calls,orders,suppliers,suggestions};
}
