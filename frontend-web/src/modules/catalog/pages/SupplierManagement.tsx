import {useEffect,useState} from 'react';
import {useLocalSession} from '../../../core/local/useLocalSession';
import {createSupplierMutations,prepareSupplierEdit,prepareSupplierStatus} from '../../../core/local/supplierMutations.mjs';
import type {SupplierState,SupplierInput,SupplierHistory} from '../../../core/local/supplierMutations.mjs';
import {useReadTask,ReadSection,ReadState,Pages} from '../../production/pages/readPanel';
import {SupplierPending,useSupplierMutations} from './SupplierPending';
const client=createSupplierMutations();
const label=(status:string)=>status==='active'?'Ativo':'Inativo';
function SupplierForms(){
 const token=useLocalSession(s=>s.token),state=useReadTask<SupplierState>(),history=useReadTask<SupplierHistory>(),mutations=useSupplierMutations();
 const [id,setID]=useState(''),[name,setName]=useState(''),[reason,setReason]=useState(''),[preview,setPreview]=useState<SupplierInput|null>(null),[confirm,setConfirm]=useState(false),[error,setError]=useState('');
 function reset(){setPreview(null);setConfirm(false);setError('');}
 useEffect(()=>{reset();setName('');setReason('');},[token]);
 function review(action:'name'|'active'|'inactive'){
  reset();if(!state.data)return;
  try{setPreview(action==='name'?prepareSupplierEdit(state.data,name,reason,'preview'):prepareSupplierStatus(state.data,action,reason,'preview'));}catch(e){setError(e instanceof Error?e.message:'Confira os dados.');}
 }
 async function save(){if(!preview||!confirm)return;const input={...preview,operation_id:crypto.randomUUID()};const ok=await mutations.submit(()=>({kind:'edit',input}));if(ok){state.clear();history.clear();setName('');setReason('');reset();}}
 function consult(){reset();setName('');setReason('');history.clear();void state.run(t=>client.state(t,id.trim()));}
 function audit(offset=0){reset();state.clear();setName('');setReason('');void history.run(t=>client.history(t,id.trim(),offset));}
 return <>
 <ReadSection title="Consultar e administrar fornecedor local">
  <p>Cadastro comercial desta loja. Alterar nome ou estado preserva pedidos, aprovações e recebimentos anteriores; não cria acesso para o fornecedor.</p>
  <form className="operation-form" onSubmit={e=>{e.preventDefault();consult();}}><label className="brand-field">ID do fornecedor<input required maxLength={128} value={id} onChange={e=>{setID(e.target.value);state.clear();history.clear();reset();setName('');setReason('');}}/></label><button className="brand-secondary" disabled={state.busy||history.busy||mutations.busy}>Consultar cadastro atual</button><button type="button" className="brand-secondary" disabled={!id.trim()||state.busy||history.busy||mutations.busy} onClick={()=>audit()}>Consultar histórico auditado</button></form>
  <ReadState task={state}/>{error&&<p role="alert">{error}</p>}
  {state.data&&<><p className="operation-id">Fornecedor: {state.data.id}</p><p>Nome: {state.data.name} · {label(state.data.status)} · revisão {state.data.revision}</p>
   <label className="brand-field">Novo nome<input maxLength={255} disabled={mutations.blocked} value={name} onChange={e=>{setName(e.target.value);reset();}}/></label><label className="brand-field">Motivo da alteração<input required maxLength={255} disabled={mutations.blocked} value={reason} onChange={e=>{setReason(e.target.value);reset();}}/></label>
   <div className="operation-actions"><button className="brand-secondary" disabled={mutations.blocked||!name.trim()||!reason.trim()} onClick={()=>review('name')}>Revisar edição do nome</button><button className="brand-secondary" disabled={mutations.blocked||state.data.status==='active'||!reason.trim()} onClick={()=>review('active')}>Revisar ativação</button><button className="brand-secondary" disabled={mutations.blocked||state.data.status==='inactive'||!reason.trim()} onClick={()=>review('inactive')}>Revisar inativação</button></div>
   {preview&&<div className="brand-state"><h3>Revisão da decisão</h3><p>Fornecedor {preview.supplier_id} · revisão consultada {preview.expected_revision}</p><p>Nome: {state.data.name} → {preview.name}</p><p>Estado: {label(state.data.status)} → {label(preview.status)}</p><p>Motivo: {preview.reason}</p><label><input type="checkbox" checked={confirm} disabled={mutations.blocked} onChange={e=>setConfirm(e.target.checked)}/> Confirmo esta alteração neste fornecedor e nesta loja.</label><button className="brand-primary" disabled={mutations.blocked||!confirm} onClick={()=>void save()}>Confirmar alteração</button></div>}
   <p>Em conflito de revisão, consulte novamente. Uma resposta incerta conserva a mesma operação; não corrige nem repete decisões automaticamente. Inativar impede novos pedidos pelos contratos existentes do servidor.</p>
  </>}
 </ReadSection>
 <ReadSection title="Histórico auditado do fornecedor"><ReadState task={history}/>{history.data&&<>
  <p className="operation-id">Fornecedor {history.data.current.id}</p><p>Cadastro atual: {history.data.current.name} · {label(history.data.current.status)} · revisão {history.data.current.revision}</p>
  <h3>Cadastro original</h3><p>{history.data.origin.name} · {label(history.data.origin.status)}</p><p>Operação: {history.data.origin.creation.operation_id} · responsável {history.data.origin.creation.actor_id} · aparelho {history.data.origin.creation.device_id} · {new Date(history.data.origin.creation.created_at).toLocaleString('pt-BR')}</p>
  <p>{history.data.total_count} alterações registradas.</p>{!history.data.items.length&&<p>Nenhuma alteração nesta página.</p>}
  {history.data.items.map(x=><article className="brand-state" key={x.revision}><h3>Revisão {x.revision}</h3><p>Nome: {x.before_name} → {x.name}</p><p>Estado: {label(x.before_status)} → {label(x.status)}</p><p>Motivo: {x.reason}</p><p className="operation-id">Operação {x.operation_id} · responsável {x.actor_id} · aparelho {x.device_id}</p><p>{new Date(x.created_at).toLocaleString('pt-BR')}</p></article>)}
  <Pages offset={history.data.offset} more={history.data.has_more} busy={history.busy||mutations.busy} change={audit}/>
 </>}</ReadSection>
 </>;
}
export default function SupplierManagement(){return <SupplierPending><SupplierForms/></SupplierPending>;}
