import ReplenishmentPanel from './ReplenishmentPanel';
import {useEffect,useState,useRef} from 'react';
import type {FormEvent} from 'react';
import {useLocalSession,localErrorMessage} from '../../../core/local/useLocalSession';
import {useLocalAccess} from '../../../core/local/LocalAccess';
import {createPurchaseClient,pendingPurchaseKey,readPurchasePending,persistPurchasePending,resolvePurchasePending} from '../../../core/local/purchaseClient.mjs';
import type {Supplier,Approval,PurchaseOrder,PendingPurchase} from '../../../core/local/purchaseClient.mjs';
const client=createPurchaseClient();
const quantity=(n:number)=>`${Math.floor(n/1000)},${String(n%1000).padStart(3,'0')}`;
export default function LocalOrders(){
 const token=useLocalSession(s=>s.token),session=useLocalSession(s=>s.session);const {capabilities}=useLocalAccess();
 const [suppliers,setSuppliers]=useState<Supplier[]>([]),[approvals,setApprovals]=useState<Approval[]>([]),[orders,setOrders]=useState<PurchaseOrder[]>([]),[detail,setDetail]=useState<PurchaseOrder|null>(null);
 const [supplier,setSupplier]=useState(''),[approval,setApproval]=useState(''),[name,setName]=useState(''),[pending,setPending]=useState<PendingPurchase|null>(null);
 const [busy,setBusy]=useState(false),[error,setError]=useState(''),[notice,setNotice]=useState(''),[offset,setOffset]=useState(0),[supplierOffset,setSupplierOffset]=useState(0),[approvalOffset,setApprovalOffset]=useState(0);
 const latch=useRef(false);
 const [storageError,setStorageError]=useState('');
 const key=session?pendingPurchaseKey(session):'';
 const manage=!!capabilities?.permissions.includes('manage_replenishment');
 const canWrite=manage&&capabilities?.license.state==='active';
 async function load(){if(!token)return;const [s,o]=await Promise.all([client.suppliers(token,supplierOffset),client.orders(token,offset)]);setSuppliers(s);setOrders(o);if(manage)setApprovals(await client.approvals(token,approvalOffset));}
 useEffect(()=>{let live=true;if(!token)return;setBusy(true);Promise.all([client.suppliers(token,supplierOffset),client.orders(token,offset),manage?client.approvals(token,approvalOffset):Promise.resolve([])]).then(([s,o,a])=>{if(live){setSuppliers(s);setOrders(o);setApprovals(a);setError('');}}).catch(e=>{if(live)setError(localErrorMessage(e));}).finally(()=>{if(live)setBusy(false);});return()=>{live=false;};},[token,offset,supplierOffset,approvalOffset,manage]);
 useEffect(()=>{try{setPending(readPurchasePending(localStorage,key));setStorageError('');}catch{setStorageError('O navegador não permitiu ler a operação pendente, ou seu conteúdo está inválido. Gravações bloqueadas.');}},[key]);
 async function locked(action:()=>Promise<void>){if(!key||!token||latch.current)return;latch.current=true;setBusy(true);setError('');try{if(!navigator.locks)throw new Error('Este navegador não oferece bloqueio entre abas. Use um navegador atualizado para gravar pedidos.');await navigator.locks.request(key,{ifAvailable:true},async lock=>{if(!lock)throw new Error("Outra aba está gravando pedidos. Aguarde e consulte novamente.");await action();});}catch(e){setError(e instanceof Error?e.message:localErrorMessage(e));}finally{try{setPending(readPurchasePending(localStorage,key));}catch{setStorageError('Não foi possível conferir a operação pendente.');}latch.current=false;setBusy(false);}}
 async function resolve(send:boolean){if(!token)return;const result=await resolvePurchasePending(client,token,localStorage,key,send);if(result?.order)setDetail(result.order);setNotice(result?.state==='order_missing'?'Nenhum pedido com esse ID foi encontrado. A repetição explícita preserva a operação original.':result?.state==='supplier_uncertain'?'Cadastro sem resposta confirmada. Repita explicitamente a mesma operação.': 'Operação confirmada no servidor local.');await load();}
 async function createSupplier(e:FormEvent){e.preventDefault();if(!canWrite||!token)return;await locked(async()=>{const input={operation_id:crypto.randomUUID(),id:crypto.randomUUID(),name:name.trim(),status:'active' as const};persistPurchasePending(localStorage,key,{kind:'supplier',input});await resolve(true);setName('');});}
 async function createOrder(e:FormEvent){e.preventDefault();if(!canWrite||!token)return;await locked(async()=>{persistPurchasePending(localStorage,key,{kind:'order',input:{operation_id:crypto.randomUUID(),order_id:crypto.randomUUID(),supplier_id:supplier,suggestion_id:approval}});await resolve(true);setApproval('');});}
 async function read(id:string){if(!token)return;setBusy(true);try{setDetail(await client.order(token,id));setError('');}catch(e){setError(localErrorMessage(e));}finally{setBusy(false);}}
 const blocked=busy||!!pending||!!storageError||!canWrite;
 return <main className="brand-app-content" style={{display:"grid",gap:20}}><h1>Pedidos e fornecedores</h1><p>Pedidos locais a partir de reposições aprovadas. O fornecedor ainda não recebe estes pedidos.</p>
  {error&&<p className="brand-state" role="alert">{error}</p>}{storageError&&<p role="alert">{storageError}</p>}{notice&&<p role="status">{notice}</p>}
  {pending&&<section className="brand-panel"><h2>Operação pendente</h2><p>{pending.kind==='order'?'Pedido':'Fornecedor'}: {pending.kind==='order'?pending.input.order_id:pending.input.id}. Os identificadores originais estão preservados.</p><button className="brand-secondary" disabled={busy} onClick={()=>void locked(()=>resolve(false))}>Consultar resultado</button> <button className="brand-primary" disabled={busy||!canWrite} onClick={()=>void locked(()=>resolve(true))}>Repetir mesma operação</button></section>}
  <ReplenishmentPanel onChange={load}/>
  {manage&&<section className="brand-panel"><h2>Cadastrar fornecedor local</h2><form className="catalog-form" onSubmit={createSupplier}><label className="brand-field">Nome<input required maxLength={255} value={name} onChange={e=>setName(e.target.value)} disabled={blocked}/></label><button className="brand-primary" disabled={blocked||!name.trim()}>Cadastrar fornecedor</button></form><p>Referência comercial nesta loja; não cria login para outra empresa.</p></section>}
  {manage&&<section className="brand-panel"><h2>Criar pedido de reposição</h2><form className="catalog-form" onSubmit={createOrder}><label className="brand-field">Fornecedor ativo<select required value={supplier} onChange={e=>setSupplier(e.target.value)} disabled={blocked}><option value="">Selecione</option>{suppliers.filter(s=>s.status==='active').map(s=><option key={s.id} value={s.id}>{s.name}</option>)}</select></label><label className="brand-field">Reposição aprovada<select required value={approval} onChange={e=>setApproval(e.target.value)} disabled={blocked}><option value="">Selecione</option>{approvals.map(a=><option key={a.suggestion_id} value={a.suggestion_id}>{a.name} — {quantity(a.quantity_milli)} {a.unit}</option>)}</select></label><button className="brand-primary" disabled={blocked||!supplier||!approval}>Criar pedido local</button></form>
   {!approvals.length&&<p>Nenhuma aprovação disponível nesta página. Configure, calcule e aprove uma reposição na seção acima.</p>}
   <button className="brand-secondary" disabled={busy||supplierOffset===0} onClick={()=>setSupplierOffset(v=>Math.max(0,v-50))}>Fornecedores anteriores</button> <button className="brand-secondary" disabled={busy||suppliers.length<50} onClick={()=>setSupplierOffset(v=>v+50)}>Mais fornecedores</button><br/>
   <button className="brand-secondary" disabled={busy||approvalOffset===0} onClick={()=>setApprovalOffset(v=>Math.max(0,v-50))}>Aprovações anteriores</button> <button className="brand-secondary" disabled={busy||approvals.length<50} onClick={()=>setApprovalOffset(v=>v+50)}>Mais aprovações</button>
   {!canWrite&&<p>Gravações indisponíveis: confira a vigência do contrato de pedidos.</p>}
  </section>}
  <section className="brand-panel"><h2>Histórico de pedidos</h2><button className="brand-secondary" disabled={busy} onClick={()=>{setBusy(true);load().catch(e=>setError(localErrorMessage(e))).finally(()=>setBusy(false));}}>Atualizar</button>
   {!orders.length?<p>Nenhum pedido nesta página.</p>:<ul>{orders.map(o=><li key={o.id}><button className="brand-secondary" disabled={busy} onClick={()=>void read(o.id)}>{o.supplier_name} — {new Date(o.created_at).toLocaleString('pt-BR')}</button><p>Pedido local — não enviado · {o.id}</p></li>)}</ul>}
   <button className="brand-secondary" disabled={busy||offset===0} onClick={()=>setOffset(v=>Math.max(0,v-50))}>Anterior</button> <button className="brand-secondary" disabled={busy||orders.length<50} onClick={()=>setOffset(v=>v+50)}>Próxima</button>
  </section>
  {detail&&<section className="brand-panel"><h2>Pedido {detail.id}</h2><p>{detail.supplier_name} · Local, não enviado</p><p>Aprovação: {detail.suggestion_id}</p>{detail.items.map(i=><p key={i.product_id}>{i.sku} · {i.name}: {quantity(i.quantity_milli)} {i.unit}</p>)}<p>Preço, frete, impostos e total ainda não negociados. Nenhuma entrada de estoque ou pagamento foi registrada.</p></section>}
 </main>;
}
