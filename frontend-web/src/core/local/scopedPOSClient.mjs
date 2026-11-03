import {LocalAPIError} from './localClient.mjs';
import {contextOf,contextKey} from '../context74/model.mjs';
// Bind one POS view to the exact login and four context IDs. Never persist tokens.
export function createScopedPOSClient(client,{token,session,getCurrent}){
 const fields=['tenant_id','store_id','identity_id','device_id'];
 if(!session||!fields.every(k=>typeof session[k]==='string'&&session[k]))throw new LocalAPIError(401,'Contexto do PDV inválido.');
 const scope=contextKey(session);
 function assertCurrent(supplied=token,write=false){
  let state,valid=false;
  try{state=getCurrent();valid=typeof token==='string'&&!!token&&!/[\r\n]/.test(token)&&supplied===token&&state.token===token&&state.session===session&&contextKey(contextOf(state.session,state.capabilities))===scope;}catch{}
  if(!valid){const e=new LocalAPIError(401,'A sessão ou o contexto do PDV mudou. Preserve a operação original e entre novamente.');e.contextChanged=true;throw e;}
  const cap=state.capabilities;
  if(!Array.isArray(cap.permissions)||!cap.permissions.includes('sell')||write&&(cap.license?.state!=='active'||!Array.isArray(cap.license.modules)||!['core','inventory','pos'].every(m=>cap.license.modules.includes(m)))){const e=new LocalAPIError(403,'Permissão ou contrato do PDV indisponível. Nenhuma nova operação foi autorizada.');e.contextChanged=true;throw e;}
 }
 const output={assertCurrent};
 for(const name of ['products','locations','history','currentCash','receipt','staff','quoteSale','completeSale','openCash','closeCash']){
  output[name]=async(supplied,...args)=>{
   assertCurrent(supplied,['quoteSale','completeSale','openCash','closeCash'].includes(name));
   let value;try{value=await client[name](supplied,...args);}catch(e){assertCurrent(supplied);throw e;}
   assertCurrent(supplied);return value;
  };
 }
 return output;
}
