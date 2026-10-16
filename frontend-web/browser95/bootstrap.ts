// Isolated demonstration. This mock is never imported by the installed application.
if(import.meta.env.VITE_CHECKOUT91_MOCKS==='true'){
 const product='00000000-0000-4000-8000-000000000001',location='00000000-0000-4000-8000-000000000002';let cash:string|null='DEMO-SHIFT',cashTotal=10000;const sales=new Map<string,unknown>();
 (window as unknown as {previewSales93:()=>unknown[]}).previewSales93=()=>[...sales.values()].map((raw:unknown)=>{const v=raw as {sale_id:string;status:string;payments:{payment_id:string;method:string;amount_cents:number}[]};return {id:v.sale_id,at:new Date().toISOString(),status:v.status,payments:v.payments.map(p=>({id:p.payment_id,method:p.method,amount:p.amount_cents,providerConfirmed:false}))};});
 window.fetch=async(input,options)=>{const path=String(input),body=options?.body?JSON.parse(String(options.body)):null,json=(v:unknown,status=200)=>new Response(JSON.stringify(v),{status,headers:{'Content-Type':'application/json'}});
 if(path.includes('/products?'))return json({items:[{id:product,sku:'DEMO-1',name:'Produto de demonstração',unit:'unit',price_cents:3000,barcode:'123'}]});
 if(path.endsWith('/locations'))return json({items:[{ID:location,name:'Prateleira de demonstração',kind:'shelf'}]});
 if(path.endsWith('/staff'))return json({items:[{identity_id:'DEMO-USER',name:'Operador demonstrativo',role:'owner',status:'active'}]});
 if(path.endsWith('/cash/current'))return json({session:cash?{session_id:cash,opened_at:new Date().toISOString()}:null});
 if(path.endsWith('/cash/open')){cash=body.session_id;cashTotal=body.opening_cents;return json({session_id:cash,repeated:false});}
 if(path.endsWith('/cash/close')){const id=cash;cash=null;return json({session_id:id,repeated:false,expected_cents:cashTotal,difference_cents:body.declared_cents-cashTotal});}
 if(path.endsWith('/sales')&&body){const old=sales.get(body.sale_id);if(old)return json({sale_id:body.sale_id,total_cents:body.payments[0].amount_cents,repeated:true});const total=body.payments[0].amount_cents,receipt={...body,status:'committed',total_cents:total,fiscal_authorized:false,items:body.items.map((i:Record<string,number|string>,n:number)=>({...i,item_id:'item-'+n,unit_price_cents:3000,total_cents:Math.round(Number(i.quantity_milli)*3000/1000)})),payments:[{payment_id:'pay-'+body.sale_id,method:'cash',amount_cents:total,status:'confirmed'}]};sales.set(body.sale_id,receipt);cashTotal+=total;return json({sale_id:body.sale_id,total_cents:total,repeated:false});}
 if(path.includes('/sales?'))return json({items:[]});
 if(path.includes('/sales/'))return json(sales.get(decodeURIComponent(path.split('/').at(-1)||''))||{},sales.has(decodeURIComponent(path.split('/').at(-1)||''))?200:404);
 return json({},503);
 };
 if(!navigator.locks)Object.defineProperty(navigator,'locks',{value:{request:async(_key:string,_options:unknown,run:(lock:object)=>Promise<unknown>)=>run({})}});
}
void import('./main');
