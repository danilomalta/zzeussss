import {useState} from 'react';
import {createProductionMutations} from '../../../core/local/productionMutations.mjs';
import {transitionChoices,prepareProductionState} from '../../../core/local/productionState.mjs';
import type {ProductionOrder} from '../../../core/local/productionRead.mjs';
import {exactQuantity as q} from '../../../core/local/operationsRead.mjs';
import {useReadTask,ReadState,ReadSection} from './readPanel';
import {useProductionMutations} from './ProductionPending';
const client=createProductionMutations();
const states:Record<string,string>={planned:'Planejada',approved:'Aprovada',cancelled:'Cancelada',completed:'Concluída'};
export default function OrderStateEditor(){
 const mutations=useProductionMutations(),loaded=useReadTask<ProductionOrder>();
 const [orderID,setOrderID]=useState(''),[reason,setReason]=useState('');
 const choices=loaded.data?transitionChoices(loaded.data):[];
 async function change(status:'approved'|'cancelled'){
  if(!loaded.data)return;const order=loaded.data;
  const ok=await mutations.submit(()=>({kind:'state',input:prepareProductionState(order,status,reason,crypto.randomUUID())}));
  if(ok){loaded.clear();setReason('');}
 }
 return <ReadSection title="Aprovar ou cancelar ordem"><p>A aprovação autoriza o plano. Ela não reserva nem consome estoque. O cancelamento preserva a auditoria e não libera reservas automaticamente.</p>
 <form className="operation-form" onSubmit={e=>{e.preventDefault();void loaded.run(t=>client.order(t,orderID.trim()));}}><label className="brand-field">ID da ordem<input required maxLength={128} value={orderID} onChange={e=>{setOrderID(e.target.value);loaded.clear();setReason('');}}/></label><button className="brand-secondary" disabled={loaded.busy||mutations.busy}>Consultar estado atual</button></form><ReadState task={loaded}/>
 {loaded.data&&<><p>{loaded.data.recipe.name} · {states[loaded.data.status]} · revisão da ordem {loaded.data.revision}</p><p className="operation-id">Versão da receita: {loaded.data.version_id}<br/>Local: {loaded.data.location_id}<br/>Responsável: {loaded.data.responsible_id}<br/>Planejado: {q(loaded.data.planned_output_milli)} {loaded.data.recipe.output_unit}</p>
 {choices.length?<><label className="brand-field">Motivo da decisão<textarea required maxLength={255} value={reason} disabled={mutations.blocked} onChange={e=>setReason(e.target.value)}/></label><p>A decisão usa exatamente a revisão consultada acima. Alteração concorrente provoca conflito. Reservas ativas ou consumo podem impedir o cancelamento; resolva-os pelo fluxo autorizado.</p><div className="operation-actions">{choices.map(status=><button key={status} className={status==='approved'?'brand-primary':'brand-secondary'} disabled={mutations.blocked||!reason.trim()} onClick={()=>void change(status)}>{status==='approved'?'Aprovar plano':'Cancelar ordem'}</button>)}</div></>:<p>Esta ordem não admite aprovação ou cancelamento pelo estado atual.</p>}
 </>}
 </ReadSection>;
}
