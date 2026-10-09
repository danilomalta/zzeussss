import {useState} from 'react';
import {createProductionMutations} from '../../../core/local/productionMutations.mjs';
import type {RecipeState} from '../../../core/local/productionMutations.mjs';
import {useReadTask,ReadState,ReadSection} from './readPanel';
import {useProductionMutations} from './ProductionPending';
const client=createProductionMutations();
export default function RecipeStateEditor(){
 const mutations=useProductionMutations(),loaded=useReadTask<RecipeState>();
 const [recipeID,setRecipeID]=useState(''),[reason,setReason]=useState('');
 async function change(){if(!loaded.data)return;const state=loaded.data;
 const ok=await mutations.submit(()=>({kind:'recipe_state',input:{operation_id:crypto.randomUUID(),recipe_id:state.recipe_id,expected_revision:state.revision,status:state.status==='active'?'inactive':'active',reason:reason.trim()}}));if(ok){loaded.clear();setReason('');}}
 return <ReadSection title="Ativar ou inativar receita"><p>Inativar impede novos planos com esta receita. Versões e ordens existentes ficam preservadas. Publicar uma versão não reativa a receita.</p>
 <form className="operation-form" onSubmit={e=>{e.preventDefault();void loaded.run(t=>client.recipeState(t,recipeID.trim()));}}><label className="brand-field">ID da receita<input required maxLength={128} value={recipeID} onChange={e=>{setRecipeID(e.target.value);loaded.clear();setReason('');}}/></label><button className="brand-secondary" disabled={loaded.busy||mutations.busy}>Consultar estado da receita</button></form><ReadState task={loaded}/>
 {loaded.data&&<><p>{loaded.data.status==='active'?'Ativa':'Inativa'} · revisão do estado {loaded.data.revision}. Esta revisão é independente da versão da receita.</p><label className="brand-field">Motivo<textarea maxLength={255} value={reason} disabled={mutations.blocked} onChange={e=>setReason(e.target.value)}/></label><button className="brand-primary" disabled={mutations.blocked||!reason.trim()||loaded.data.revision>=2147483647} onClick={()=>void change()}>{loaded.data.status==='active'?'Inativar receita':'Ativar receita'}</button></>}
 </ReadSection>;
}
