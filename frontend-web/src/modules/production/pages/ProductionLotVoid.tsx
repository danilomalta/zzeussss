import {prepareLotVoid} from '../../../core/local/productionLotActions.mjs';
import {useState} from 'react';
import {createProductionLots} from '../../../core/local/productionLots.mjs';
import type {ProductionLot} from '../../../core/local/productionLots.mjs';
import {exactQuantity as q} from '../../../core/local/operationsRead.mjs';
import {useReadTask,ReadState,ReadSection} from './readPanel';
import {useProductionMutations} from './ProductionPending';
const client=createProductionLots();
export default function ProductionLotVoid(){
 const loaded=useReadTask<ProductionLot>(),mutations=useProductionMutations();const [lotID,setLotID]=useState(''),[reason,setReason]=useState(''),[confirm,setConfirm]=useState(false);
 async function change(){if(!loaded.data||!confirm||loaded.data.status!=='recorded')return;const lot=loaded.data;const ok=await mutations.submit(()=>({kind:'lot_void',input:prepareLotVoid(lot,reason,crypto.randomUUID())}));if(ok){loaded.clear();setReason('');setConfirm(false);}}
 return <ReadSection title="Anular declaração de lote"><p>Anular preserva a declaração e registra um novo evento. A quantidade volta ao total produzido ainda sem lote. Esta ação não repõe produto nem ingredientes e não estorna estoque.</p>
 <form className="operation-form" onSubmit={e=>{e.preventDefault();setReason('');setConfirm(false);void loaded.run(t=>client.lot(t,lotID.trim()));}}><label className="brand-field">ID do lote<input required maxLength={128} value={lotID} onChange={e=>{setLotID(e.target.value);loaded.clear();setReason('');setConfirm(false);}}/></label><button className="brand-secondary" disabled={loaded.busy||mutations.busy}>Consultar declaração atual</button></form><ReadState task={loaded}/>
 {loaded.data&&<><p className="operation-id">Lote: {loaded.data.id}<br/>Resultado: {loaded.data.result_id}<br/>Produto: {loaded.data.product_id}<br/>Declarada por: {loaded.data.created_by}</p><p>{q(loaded.data.quantity_milli)} {loaded.data.unit} · {loaded.data.status==='recorded'?'Declarada':'Anulada'} · revisão {loaded.data.revision}</p><p>Motivo original: {loaded.data.reason}</p>
 {loaded.data.status==='recorded'?<><label className="brand-field">Motivo da anulação<textarea maxLength={255} disabled={mutations.blocked} value={reason} onChange={e=>{setReason(e.target.value);setConfirm(false);}}/></label><label><input type="checkbox" disabled={mutations.blocked} checked={confirm} onChange={e=>setConfirm(e.target.checked)}/> Confirmo a anulação desta classificação, preservando seu histórico e o estoque.</label><button className="brand-primary" disabled={mutations.blocked||!reason.trim()||!confirm} onClick={()=>void change()}>Anular declaração com auditoria</button></>:<p>Esta declaração já foi anulada. Não é possível reabrir ou anular novamente; uma nova classificação exige outra declaração autorizada, com outro código.</p>}
 <p>A anulação usa a revisão consultada. Conflito não muda revisão ou motivo automaticamente. Consulte novamente após resolver uma operação pendente.</p></>}
 </ReadSection>;
}
