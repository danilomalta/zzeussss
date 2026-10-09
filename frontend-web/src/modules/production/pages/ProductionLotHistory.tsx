import {useState} from 'react';
import {createProductionLots} from '../../../core/local/productionLots.mjs';
import type {LotAudit} from '../../../core/local/productionLots.mjs';
import {exactQuantity as q} from '../../../core/local/operationsRead.mjs';
import {useReadTask,ReadState,ReadSection} from './readPanel';
const client=createProductionLots();
export default function ProductionLotHistory(){
 const loaded=useReadTask<LotAudit>();const [lotID,setLotID]=useState('');
 return <ReadSection title="Histórico auditado do lote"><p>Confira a declaração original e uma eventual anulação, com operador, aparelho, motivo e operação preservados.</p>
 <form className="operation-form" onSubmit={e=>{e.preventDefault();void loaded.run(t=>client.audit(t,lotID.trim()));}}><label className="brand-field">ID do lote<input required maxLength={128} value={lotID} onChange={e=>{setLotID(e.target.value);loaded.clear();}}/></label><button className="brand-secondary" disabled={loaded.busy}>Consultar lote e histórico</button></form><ReadState task={loaded}/>
 {loaded.data&&<><p className="operation-id">Lote: {loaded.data.lot.id}<br/>Resultado: {loaded.data.lot.result_id}<br/>Produto: {loaded.data.lot.product_id}</p><p>Código original: {loaded.data.lot.code} · quantidade: {q(loaded.data.lot.quantity_milli)} {loaded.data.lot.unit}<br/>Fabricação: {loaded.data.lot.manufactured_on} · validade: {loaded.data.lot.expires_on||'Não informada'}<br/>Estado atual: {loaded.data.lot.status==='recorded'?'Declarado':'Anulado'} · revisão {loaded.data.lot.revision}</p><ol>{loaded.data.events.map(e=><li key={e.revision}><strong>{e.kind==='recorded'?'Declaração original':'Anulação auditada'}</strong> · revisão {e.revision}<p>{e.reason}</p><p className="operation-id">Operador: {e.actor_id}<br/>Aparelho: {e.device_id}<br/>Operação: {e.operation_id}<br/>Registrado em: {e.created_at}</p></li>)}</ol><p>A anulação de um lote conserva seus metadados e desfaz a atribuição ao resultado. Não retira produto do estoque nem anula avaliações históricas de qualidade.</p></>}
 </ReadSection>;
}
