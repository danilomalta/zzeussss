import {useState} from 'react';
import {createProductionExecution,executionChoices} from '../../../core/local/productionExecution.mjs';
import type {ExecutionTrace} from '../../../core/local/productionExecution.mjs';
import {exactQuantity as q} from '../../../core/local/operationsRead.mjs';
import {useReadTask,ReadState,ReadSection} from './readPanel';
import {useProductionMutations} from './ProductionPending';
const client=createProductionExecution();
export default function ProductionMaterials(){
 const loaded=useReadTask<ExecutionTrace>(),mutations=useProductionMutations();
 const [orderID,setOrderID]=useState(''),[reason,setReason]=useState(''),[confirm,setConfirm]=useState(false);
 const choices=loaded.data?executionChoices(loaded.data):[];
 async function action(kind:'reserve'|'release'|'consume'){
  if(!loaded.data||!choices.includes(kind)||(kind==='consume'&&!confirm))return;const trace=loaded.data;
  const ok=await mutations.submit(()=>kind==='reserve'?{kind:'reserve',input:{operation_id:crypto.randomUUID(),reservation_id:crypto.randomUUID(),order_id:trace.order.id,reason:reason.trim()}}:{kind:'materials',input:{operation_id:crypto.randomUUID(),reservation_id:trace.materials.current!.id,action:kind,reason:reason.trim()}});
  if(ok){loaded.clear();setReason('');setConfirm(false);}
 }
 return <ReadSection title="Reservar, liberar ou consumir ingredientes"><p>A reserva reduz o saldo livre, sem baixar o saldo físico. Liberar devolve a disponibilidade. Consumir baixa os ingredientes do local e não conclui a ordem.</p>
 <form className="operation-form" onSubmit={e=>{e.preventDefault();setReason('');setConfirm(false);void loaded.run(t=>client.trace(t,orderID.trim()));}}><label className="brand-field">ID da ordem<input required maxLength={128} value={orderID} onChange={e=>{setOrderID(e.target.value);loaded.clear();setReason('');setConfirm(false);}}/></label><button className="brand-secondary" disabled={loaded.busy||mutations.busy}>Consultar ingredientes e reserva</button></form><ReadState task={loaded}/>
 {loaded.data&&<><p>{loaded.data.order.recipe.name} · estado {loaded.data.order.status}</p><p className="operation-id">Ordem: {loaded.data.order.id}<br/>Versão: {loaded.data.order.version_id}<br/>Local: {loaded.data.order.location_id}<br/>Reserva atual: {loaded.data.materials.current?.id||'Nenhuma'} · {loaded.data.materials.current?.status||'sem reserva'}<br/>Reservas liberadas: {loaded.data.materials.released_count}</p>
 <div className="operation-table-wrap"><table className="operation-table"><caption>Ingredientes preservados pelo plano</caption><thead><tr><th>Produto</th><th>Planejado</th><th>Reservado</th><th>Consumido</th></tr></thead><tbody>{loaded.data.ingredients.map(i=><tr key={i.product_id}><td className="operation-id">{i.product_id}</td><td>{q(i.planned_milli)} {i.unit}</td><td>{q(i.reserved_milli)} {i.unit}</td><td>{q(i.consumed_milli)} {i.unit}</td></tr>)}</tbody></table></div>
 {choices.length?<><label className="brand-field">Motivo da operação<textarea maxLength={255} disabled={mutations.blocked} value={reason} onChange={e=>setReason(e.target.value)}/></label>{choices.includes('consume')&&<label><input type="checkbox" disabled={mutations.blocked} checked={confirm} onChange={e=>setConfirm(e.target.checked)}/> Confirmo a baixa física de todos os ingredientes desta reserva no local indicado.</label>}<div className="operation-actions">{choices.map(choice=><button key={choice} className="brand-primary" disabled={mutations.blocked||!reason.trim()||(choice==='consume'&&!confirm)} onClick={()=>void action(choice as 'reserve'|'release'|'consume')}>{({reserve:'Reservar ingredientes',release:'Liberar reserva',consume:'Consumir ingredientes'})[choice as 'reserve'|'release'|'consume']}</button>)}</div></>:<p>Sem ação de materiais disponível neste estado. Consumo já registrado permanece preservado. Consulte novamente após resolver uma operação pendente.</p>}
 <p>O servidor confere autorização, saldo livre, unidades e estado na transação. Mudança concorrente pode recusar a operação; nenhum saldo é presumido pela interface.</p></>}
 </ReadSection>;
}
