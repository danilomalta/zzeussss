import {useState} from 'react';
import {useLocalSession} from '../../../core/local/useLocalSession';
import {createProductionExecution} from '../../../core/local/productionExecution.mjs';
import type {ExecutionTrace} from '../../../core/local/productionExecution.mjs';
import {canConfigureStages,prepareStagePlan} from '../../../core/local/productionStages.mjs';
import {useReadTask,ReadState,ReadSection} from './readPanel';
import {useProductionMutations} from './ProductionPending';
const client=createProductionExecution();
export default function ProductionStagePlan(){
 const session=useLocalSession(s=>s.session),loaded=useReadTask<ExecutionTrace>(),mutations=useProductionMutations();
 const [orderID,setOrderID]=useState(''),[rows,setRows]=useState<{name:string;responsible_id:string}[]>([]),[reason,setReason]=useState('');
 const allowed=loaded.data?canConfigureStages(loaded.data):false;
 function update(n:number,k:'name'|'responsible_id',value:string){setRows(old=>old.map((r,i)=>i===n?{...r,[k]:value}:r));}
 async function save(){if(!loaded.data)return;const trace=loaded.data;const ok=await mutations.submit(()=>({kind:'stage_plan',input:prepareStagePlan(trace,rows,reason,crypto.randomUUID(),()=>crypto.randomUUID())}));if(ok){loaded.clear();setRows([]);setReason('');}}
 return <ReadSection title="Definir etapas da ordem"><p>Defina de 1 a 20 etapas na ordem de execução, antes de reservar ou consumir ingredientes. O plano é imutável depois de registrado; cada responsável precisa ter permissão de produção. Esta definição não movimenta estoque.</p>
 <form className="operation-form" onSubmit={e=>{e.preventDefault();setRows([]);setReason('');void loaded.run(t=>client.trace(t,orderID.trim()));}}><label className="brand-field">ID da ordem<input required maxLength={128} value={orderID} onChange={e=>{setOrderID(e.target.value);loaded.clear();setRows([]);setReason('');}}/></label><button className="brand-secondary" disabled={loaded.busy||mutations.busy}>Consultar plano e materiais</button></form><ReadState task={loaded}/>
 {loaded.data&&<><p className="operation-id">Ordem: {loaded.data.order.id}<br/>Versão: {loaded.data.order.version_id}<br/>Local: {loaded.data.order.location_id}</p>{loaded.data.stages?<ol>{loaded.data.stages.items.map(s=><li key={s.stage_id}>{s.name} · responsável {s.responsible_id} · {s.status}</li>)}</ol>:allowed?<>
 {rows.map((r,n)=><fieldset key={n} disabled={mutations.blocked}><legend>Etapa {n+1}</legend><label className="brand-field">Nome<input maxLength={120} value={r.name} onChange={e=>update(n,'name',e.target.value)}/></label><label className="brand-field">ID do responsável<input maxLength={128} value={r.responsible_id} onChange={e=>update(n,'responsible_id',e.target.value)}/></label><button className="brand-secondary" onClick={()=>setRows(old=>old.filter((_,i)=>i!==n))}>Remover etapa {n+1}</button></fieldset>)}
 <button className="brand-secondary" disabled={mutations.blocked||rows.length>=20} onClick={()=>setRows(old=>[...old,{name:'',responsible_id:session?.identity_id||''}])}>Adicionar próxima etapa</button>
 <label className="brand-field">Motivo do plano<textarea maxLength={255} value={reason} disabled={mutations.blocked} onChange={e=>setReason(e.target.value)}/></label><button className="brand-primary" disabled={mutations.blocked||!rows.length||rows.some(r=>!r.name.trim()||!r.responsible_id.trim())||!reason.trim()} onClick={()=>void save()}>Registrar plano imutável de etapas</button>
 </>:<p>Não é possível definir etapas no estado atual ou após reserva/consumo. Reservas liberadas não impedem a definição; consulte o estado atual antes de decidir.</p>}</>}
 </ReadSection>;
}
