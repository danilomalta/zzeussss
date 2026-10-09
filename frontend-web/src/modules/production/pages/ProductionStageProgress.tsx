import {useState} from 'react';
import {useLocalSession} from '../../../core/local/useLocalSession';
import {createProductionExecution} from '../../../core/local/productionExecution.mjs';
import type {ExecutionTrace} from '../../../core/local/productionExecution.mjs';
import {stageDecision,prepareStageState} from '../../../core/local/productionStages.mjs';
import {useReadTask,ReadState,ReadSection} from './readPanel';
import {useProductionMutations} from './ProductionPending';
const client=createProductionExecution();
const states:Record<string,string>={pending:'Pendente',running:'Em andamento',completed:'Concluída'};
export default function ProductionStageProgress(){
 const session=useLocalSession(s=>s.session),loaded=useReadTask<ExecutionTrace>(),mutations=useProductionMutations();
 const [orderID,setOrderID]=useState(''),[reason,setReason]=useState('');
 const choice=loaded.data&&session?stageDecision(loaded.data,session.identity_id):null;
 async function change(){if(!loaded.data||!session||!choice)return;const trace=loaded.data;const actor=session.identity_id;
 const ok=await mutations.submit(()=>({kind:'stage_state',input:prepareStageState(trace,actor,reason,crypto.randomUUID())}));if(ok){loaded.clear();setReason('');}}
 return <ReadSection title="Executar etapas em sequência"><p>Cada responsável inicia e conclui sua própria etapa. A ordem precisa estar aprovada, com ingredientes consumidos e etapas anteriores concluídas. Avançar etapas não baixa estoque nem conclui a ordem.</p>
 <form className="operation-form" onSubmit={e=>{e.preventDefault();setReason('');void loaded.run(t=>client.trace(t,orderID.trim()));}}><label className="brand-field">ID da ordem<input required maxLength={128} value={orderID} onChange={e=>{setOrderID(e.target.value);loaded.clear();setReason('');}}/></label><button className="brand-secondary" disabled={loaded.busy||mutations.busy}>Consultar próxima etapa</button></form><ReadState task={loaded}/>
 {loaded.data&&<><p className="operation-id">Ordem: {loaded.data.order.id}<br/>Versão: {loaded.data.order.version_id}<br/>Local: {loaded.data.order.location_id}<br/>Operador atual: {session?.identity_id}</p>
 {loaded.data.stages?<ol>{loaded.data.stages.items.map(s=><li key={s.stage_id}><strong>{s.name}</strong> · {states[s.status]} · revisão da etapa {s.revision}<p className="operation-id">Etapa: {s.stage_id}<br/>Responsável: {s.responsible_id}</p></li>)}</ol>:<p>Nenhum plano de etapas registrado.</p>}
 {choice?<><p>Próxima decisão: {choice.status==='running'?'iniciar':'concluir'} a etapa <span className="operation-id">{choice.stage_id}</span>, usando a revisão {choice.expected_revision}.</p><label className="brand-field">Motivo da decisão<textarea maxLength={255} disabled={mutations.blocked} value={reason} onChange={e=>setReason(e.target.value)}/></label><button className="brand-primary" disabled={mutations.blocked||!reason.trim()} onClick={()=>void change()}>{choice.status==='running'?'Iniciar minha etapa':'Concluir minha etapa'}</button></>:<p>Nenhuma etapa disponível para este operador neste estado. Confira aprovação, consumo, sequência e responsável. Se todas estiverem concluídas, registre o resultado medido no painel de conclusão.</p>}
 <p>A revisão é da etapa e não da ordem. Conflito não avança para uma revisão nova automaticamente; consulte e decida novamente após resolver a pendência.</p></>}
 </ReadSection>;
}
