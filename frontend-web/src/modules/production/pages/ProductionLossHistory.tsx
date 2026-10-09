import {useState} from 'react';
import {createProductionLosses} from '../../../core/local/productionLosses.mjs';
import type {LossAudit} from '../../../core/local/productionLosses.mjs';
import {exactQuantity as q} from '../../../core/local/operationsRead.mjs';
import {useReadTask,ReadState,ReadSection} from './readPanel';
const client=createProductionLosses();
export default function ProductionLossHistory(){
 const loaded=useReadTask<LossAudit>();const [lossID,setLossID]=useState('');
 return <ReadSection title="Histórico auditado da perda"><p>Declaração e anulação são eventos distintos e preservados. Consultar o histórico não altera a classificação ou o estoque.</p>
 <form className="operation-form" onSubmit={e=>{e.preventDefault();void loaded.run(t=>client.audit(t,lossID.trim()));}}><label className="brand-field">ID da perda<input required maxLength={128} value={lossID} onChange={e=>{setLossID(e.target.value);loaded.clear();}}/></label><button className="brand-secondary" disabled={loaded.busy}>Consultar registro e histórico</button></form><ReadState task={loaded}/>
 {loaded.data&&<><p className="operation-id">Perda: {loaded.data.loss.id}<br/>Resultado: {loaded.data.loss.result_id}<br/>Produto: {loaded.data.loss.product_id}</p><p>Quantidade original: {q(loaded.data.loss.quantity_milli)} {loaded.data.loss.unit}. Estado atual: {loaded.data.loss.status==='recorded'?'Declarada':'Anulada'} · revisão {loaded.data.loss.revision}.</p>
 <ol>{loaded.data.events.map(e=><li key={e.revision}><strong>{e.kind==='recorded'?'Declaração original':'Anulação auditada'}</strong> · revisão {e.revision}<p>{e.reason}</p><p className="operation-id">Operador: {e.actor_id}<br/>Aparelho: {e.device_id}<br/>Operação: {e.operation_id}<br/>Registrado em: {e.created_at}</p></li>)}</ol><p>A anulação conserva a quantidade e o motivo originais. Ela altera somente a classificação disponível do resultado.</p></>}
 </ReadSection>;
}
