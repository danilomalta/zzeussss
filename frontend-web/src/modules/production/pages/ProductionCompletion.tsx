import {useState} from 'react';
import {createProductionExecution,completionReady,prepareCompletion} from '../../../core/local/productionExecution.mjs';
import type {ExecutionTrace} from '../../../core/local/productionExecution.mjs';
import {exactQuantity as q} from '../../../core/local/operationsRead.mjs';
import {localErrorMessage} from '../../../core/local/useLocalSession';
import {useReadTask,ReadState,ReadSection} from './readPanel';
import {useProductionMutations} from './ProductionPending';
const client=createProductionExecution();
export default function ProductionCompletion(){
 const loaded=useReadTask<ExecutionTrace>(),mutations=useProductionMutations();
 const [orderID,setOrderID]=useState(''),[quantity,setQuantity]=useState(''),[reason,setReason]=useState(''),[confirm,setConfirm]=useState(false);
 const trace=loaded.data,ready=trace?completionReady(trace):false;
 let produced:number|undefined,invalid='';
 if(trace&&ready&&quantity.trim()&&reason.trim()){try{produced=prepareCompletion(trace,quantity,reason,'preview','preview-result').produced_milli;}catch(e){invalid=localErrorMessage(e);}}
 function reset(){setQuantity('');setReason('');setConfirm(false);}
 async function complete(){if(!trace||!ready||produced===undefined||!confirm)return;
 const ok=await mutations.submit(()=>({kind:'result',input:prepareCompletion(trace,quantity,reason,crypto.randomUUID(),crypto.randomUUID())}));if(ok){loaded.clear();reset();}}
 return <ReadSection title="Registrar resultado medido e concluir"><p>Informe a quantidade efetivamente produzida, na unidade do produto resultante. A conclusão registra sua entrada no mesmo local e encerra a ordem. Ela não reserva nem consome ingredientes automaticamente.</p>
 <form className="operation-form" onSubmit={e=>{e.preventDefault();reset();void loaded.run(t=>client.trace(t,orderID.trim()));}}><label className="brand-field">ID da ordem<input required maxLength={128} value={orderID} onChange={e=>{setOrderID(e.target.value);loaded.clear();reset();}}/></label><button className="brand-secondary" disabled={loaded.busy||mutations.busy}>Conferir ordem, consumo e etapas</button></form><ReadState task={loaded}/>
 {trace&&<><p>{trace.order.recipe.name} · {trace.order.status} · revisão {trace.order.revision}</p><p className="operation-id">Versão imutável: {trace.order.version_id}<br/>Produto resultante: {trace.order.recipe.output_product_id}<br/>Local: {trace.order.location_id}<br/>Planejado: {q(trace.order.planned_output_milli)} {trace.order.recipe.output_unit}<br/>Ingredientes: {trace.materials.current?.status||'sem reserva'}</p>
 {trace.stages?<ul>{trace.stages.items.map(s=><li key={s.stage_id}>{s.position}. {s.name}: {s.status==='completed'?'Concluída':s.status==='running'?'Em andamento':'Pendente'}</li>)}</ul>:<p>Esta ordem não possui plano de etapas.</p>}
 {trace.result?<p role="status">Resultado já registrado: {trace.result.result_id} · produzido {q(trace.result.produced_milli)} {trace.result.unit} · diferença {q(trace.result.shortfall_milli)} {trace.result.unit}. Consulte novamente para acompanhar o histórico.</p>:ready?<>
 <label className="brand-field">Quantidade medida ({trace.order.recipe.output_unit})<input inputMode="decimal" maxLength={32} placeholder="Ex.: 27 ou 0" disabled={mutations.blocked} value={quantity} onChange={e=>{setQuantity(e.target.value);setConfirm(false);}}/></label>
 <label className="brand-field">Motivo ou observação da medição<textarea maxLength={255} disabled={mutations.blocked} value={reason} onChange={e=>{setReason(e.target.value);setConfirm(false);}}/></label>
 {invalid&&<p role="alert">{invalid}</p>}{produced!==undefined&&<p>Entrada de produto acabado: {q(produced)} {trace.order.recipe.output_unit}. Diferença frente ao plano: {q(trace.order.planned_output_milli-produced)} {trace.order.recipe.output_unit}.{produced===0?' Resultado zero: encerra a ordem sem entrada de produto acabado.':''}</p>}
 <p>A diferença é documental; não classifica perdas automaticamente. São aceitas até três casas decimais, sem separador de milhar, e unidades inteiras quando a unidade é unit. Resultado acima do planejado é recusado.</p>
 <label><input type="checkbox" disabled={mutations.blocked||produced===undefined} checked={confirm} onChange={e=>setConfirm(e.target.checked)}/> Confirmo a quantidade medida e a conclusão desta ordem no local indicado.</label>
 <div className="operation-actions"><button className="brand-primary" disabled={mutations.blocked||produced===undefined||!confirm} onClick={()=>void complete()}>Registrar resultado e concluir</button></div>
 </>:<p>Conclusão indisponível: a ordem precisa estar aprovada, com ingredientes consumidos e todas as etapas existentes concluídas. Atualize a consulta após cada operação.</p>}
 <p>A revisão consultada e a quantidade medida ficam preservadas na operação. Alterações concorrentes são conferidas pelo servidor antes de qualquer entrada de estoque.</p></>}
 </ReadSection>;
}
