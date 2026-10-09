import {useState} from 'react';
import {createProductionQuality} from '../../../core/local/productionQuality.mjs';
import type {QualityAudit,QualityReview} from '../../../core/local/productionQuality.mjs';
import {exactQuantity as q} from '../../../core/local/operationsRead.mjs';
import {useReadTask,ReadState,ReadSection,Pages} from './readPanel';
const client=createProductionQuality();
const verdict={not_assessed:'Não avaliado',passed:'Aprovado pelo avaliador',failed:'Reprovado pelo avaliador'};
function Review({value:r}:{value:QualityReview}){return <><strong>{verdict[r.status]}</strong> · avaliação {r.revision}<p>Critério: {r.criterion}<br/>Motivo: {r.reason}</p><p className="operation-id">Avaliador: {r.actor_id}<br/>Aparelho: {r.device_id}<br/>Operação: {r.operation_id}<br/>Data: {r.created_at}</p><p>Retrato do lote na avaliação: {r.lot_snapshot.code} · {q(r.lot_snapshot.quantity_milli)} {r.lot_snapshot.unit} · fabricação {r.lot_snapshot.manufactured_on} · validade {r.lot_snapshot.expires_on||'Não informada'}.</p></>;}
export default function ProductionQualityHistory(){
 const loaded=useReadTask<QualityAudit>();const [lotID,setLotID]=useState(''),[offset,setOffset]=useState(0);
 function load(n=0){setOffset(n);void loaded.run(t=>client.audit(t,lotID.trim(),n));}
 return <ReadSection title="Avaliações humanas de qualidade do lote"><p>Consulte o último parecer e o histórico. Um parecer não constitui liberação sanitária, baixa de estoque ou bloqueio automático de venda.</p>
 <form className="operation-form" onSubmit={e=>{e.preventDefault();load();}}><label className="brand-field">ID do lote<input required maxLength={128} value={lotID} onChange={e=>{setLotID(e.target.value);setOffset(0);loaded.clear();}}/></label><button className="brand-secondary" disabled={loaded.busy}>Consultar qualidade e histórico</button></form><ReadState task={loaded}/>
 {loaded.data&&<><p className="operation-id">Lote: {loaded.data.lot.id}<br/>Resultado: {loaded.data.lot.result_id}<br/>Produto: {loaded.data.lot.product_id}</p><p>Estado atual do lote: {loaded.data.lot.status==='recorded'?'Declarado':'Anulado'} · quantidade original {q(loaded.data.lot.quantity_milli)} {loaded.data.lot.unit}.</p>{loaded.data.lot.status==='voided'&&<p>O lote está anulado. Avaliações anteriores permanecem como registros históricos e não reativam sua atribuição.</p>}<h3>Última avaliação consultada</h3>{loaded.data.quality.latest?<Review value={loaded.data.quality.latest}/>:<p>Não avaliado. Ausência de parecer não significa aprovação.</p>}<h3>Histórico das avaliações</h3>{loaded.data.items.length===0&&<p>Nenhuma avaliação nesta página.</p>}<ol start={offset+1}>{loaded.data.items.map(r=><li key={r.revision}><Review value={r}/></li>)}</ol><Pages offset={offset} more={loaded.data.has_more} busy={loaded.busy} change={load}/><p>Os pareceres são decisões humanas independentes preservadas em sequência. Consultas podem mudar entre páginas; atualize se houver conflito.</p></>}
 </ReadSection>;
}
