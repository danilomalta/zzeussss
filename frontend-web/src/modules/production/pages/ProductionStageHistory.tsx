import {useState} from 'react';
import {createProductionStageHistory} from '../../../core/local/productionStageHistory.mjs';
import type {StageHistoryEvent} from '../../../core/local/productionStageHistory.mjs';
import {useReadTask,ReadState,ReadSection,Pages} from './readPanel';
const client=createProductionStageHistory();
const names={configured:'Plano definido',running:'Etapa iniciada',completed:'Etapa concluída'};
export default function ProductionStageHistory(){
 const loaded=useReadTask<StageHistoryEvent[]>();const [orderID,setOrderID]=useState(''),[offset,setOffset]=useState(0);
 function load(next=0){setOffset(next);void loaded.run(t=>client.history(t,orderID.trim(),next));}
 return <ReadSection title="Histórico auditado das etapas"><p>Consulta os eventos originais em ordem de registro, com responsável pela decisão, aparelho, motivo e revisão resultante. Este histórico não avança etapas nem altera estoque.</p>
 <form className="operation-form" onSubmit={e=>{e.preventDefault();load();}}><label className="brand-field">ID da ordem<input required maxLength={128} value={orderID} onChange={e=>{setOrderID(e.target.value);setOffset(0);loaded.clear();}}/></label><button className="brand-secondary" disabled={loaded.busy}>Consultar histórico de etapas</button></form><ReadState task={loaded}/>
 {loaded.data&&<>{loaded.data.length===0?<p>Nenhum evento nesta página.</p>:<ol>{loaded.data.map(e=><li key={e.sequence}><strong>{names[e.kind]}</strong> · revisão {e.result.revision}<p>{e.reason}</p><p className="operation-id">Sequência: {e.sequence}<br/>Operação: {e.operation_id}<br/>Operador: {e.actor_id}<br/>Aparelho: {e.device_id}<br/>Registrado em: {e.created_at}</p>{'stages' in e.request?<ol>{e.request.stages.map(s=><li key={s.stage_id}>{s.name} · etapa {s.stage_id} · responsável {s.responsible_id}</li>)}</ol>:<p className="operation-id">Etapa: {e.request.stage_id}<br/>Revisão anterior: {e.request.expected_revision}</p>}</li>)}</ol>}
 <Pages offset={offset} more={loaded.data.length===50} busy={loaded.busy} change={load}/><p>Páginas de até 50 eventos. A sequência é global no banco e pode ter intervalos; ela não representa a quantidade de etapas desta ordem.</p></>}
 </ReadSection>;
}
