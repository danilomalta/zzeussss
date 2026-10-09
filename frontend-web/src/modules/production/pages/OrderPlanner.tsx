import {useState} from 'react';
import {createProductionMutations,parseProductionInteger,plannedPreview} from '../../../core/local/productionMutations.mjs';
import type {RecipeVersion} from '../../../core/local/productionRead.mjs';
import {useLocalSession,localErrorMessage} from '../../../core/local/useLocalSession';
import {exactQuantity as q} from '../../../core/local/operationsRead.mjs';
import {useReadTask,ReadState,ReadSection} from './readPanel';
import {useProductionMutations} from './ProductionPending';
const client=createProductionMutations();
export default function OrderPlanner(){
 const session=useLocalSession(s=>s.session),mutations=useProductionMutations(),recipe=useReadTask<RecipeVersion>();
 const [version,setVersion]=useState(''),[location,setLocation]=useState(''),[responsible,setResponsible]=useState(session?.identity_id||''),[batches,setBatches]=useState('1'),[created,setCreated]=useState('');
 let preview:ReturnType<typeof plannedPreview>|null=null,error='';
 if(recipe.data){try{preview=plannedPreview(recipe.data,parseProductionInteger(batches));}catch(e){error=localErrorMessage(e);}}
 const disabled=mutations.blocked||recipe.busy;
 async function create(){
  if(!recipe.data||!preview)return;const saved=recipe.data;let orderID='';
  const ok=await mutations.submit(()=>{orderID=crypto.randomUUID();return {kind:'order',input:{operation_id:crypto.randomUUID(),order_id:orderID,version_id:saved.version_id,location_id:location.trim(),responsible_id:responsible.trim(),planned_batches:parseProductionInteger(batches)}};});
  if(ok)setCreated(orderID);
 }
 return <ReadSection title="Planejar ordem de produção"><p>A ordem preserva a versão da receita e as quantidades planejadas. Criar a ordem não reserva nem consome ingredientes.</p>
 <form className="operation-form" onSubmit={e=>{e.preventDefault();setCreated('');void recipe.run(t=>client.version(t,version.trim()));}}><label className="brand-field">ID da versão imutável<input required maxLength={128} value={version} onChange={e=>{setVersion(e.target.value);recipe.clear();setCreated('');}}/></label><button className="brand-secondary" disabled={disabled}>Consultar versão para planejar</button></form><ReadState task={recipe}/>
 {recipe.data&&<form className="operation-panel" onSubmit={e=>{e.preventDefault();void create();}}><p>{recipe.data.name} · revisão {recipe.data.revision} · rendimento por batida {q(recipe.data.yield_milli)} {recipe.data.output_unit}</p>
 <fieldset disabled={disabled}><div className="operation-form"><label className="brand-field">ID do local de estoque<input required maxLength={128} value={location} onChange={e=>setLocation(e.target.value)}/></label><label className="brand-field">ID do responsável<input required maxLength={128} value={responsible} onChange={e=>setResponsible(e.target.value)}/></label><label className="brand-field">Batidas planejadas (inteiro)<input required inputMode="numeric" value={batches} onChange={e=>setBatches(e.target.value)}/></label></div>
 <p>O servidor confere o local, a receita ativa, unidades atuais e a permissão do responsável na loja. A atribuição não representa uma assinatura dele.</p>
 {error&&<p role="alert">{error}</p>}{preview&&<><dl className="operation-metrics"><div><dt>Produto resultante planejado</dt><dd>{q(preview.output_milli)} {recipe.data.output_unit}</dd></div></dl><ul>{preview.ingredients.map(i=><li key={i.product_id} className="operation-id">{i.product_id}: {q(i.planned_milli)} {i.unit}</li>)}</ul><button className="brand-primary" disabled={!location.trim()||!responsible.trim()}>Criar ordem planejada</button></>}
 </fieldset></form>}{created&&<p role="status" className="operation-id">Ordem confirmada: {created}. Consulte as ordens para ver o estado atual.</p>}
 </ReadSection>;
}
