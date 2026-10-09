import {createContext,useContext,useEffect,useRef,useState} from 'react';
import type {ReactNode} from 'react';
import {useLocalSession,localErrorMessage} from '../../../core/local/useLocalSession';
import {useLocalAccess} from '../../../core/local/LocalAccess';
import {LocalAPIError} from '../../../core/local/localClient.mjs';
import {createProductionMutations,productionPendingKey,readProductionPending,persistProductionPending,resolveProductionPending} from '../../../core/local/productionMutations.mjs';
import type {ProductionOperation,PendingProduction} from '../../../core/local/productionMutations.mjs';
import {ReadSection} from './readPanel';
const client=createProductionMutations();
interface Mutations {blocked:boolean;busy:boolean;submit:(prepare:()=>ProductionOperation)=>Promise<boolean>}
const Context=createContext<Mutations>({blocked:true,busy:false,submit:async()=>false});
export const useProductionMutations=()=>useContext(Context);
export function ProductionPending({children}:{children:ReactNode}){
 const token=useLocalSession(s=>s.token),session=useLocalSession(s=>s.session),invalidate=useLocalSession(s=>s.invalidate);
 const {capabilities}=useLocalAccess();
 const canWrite=!!capabilities?.permissions.includes('manage_production') && capabilities.license.state==='active' && capabilities.license.modules.includes('production');
 const key=session?productionPendingKey(session):'';
 const [pending,setPending]=useState<PendingProduction|null>(null),[busy,setBusy]=useState(false),[error,setError]=useState(''),[notice,setNotice]=useState(''),[storageError,setStorageError]=useState(''),[ready,setReady]=useState(false);
 const latch=useRef(false),context=useRef({key,token});context.current={key,token};
 useEffect(()=>{setPending(null);setError('');setNotice('');setBusy(false);setReady(false);
  function refresh(){try{setPending(readProductionPending(localStorage,key));setStorageError('');setReady(true);}catch(e){setStorageError(localErrorMessage(e));setReady(false);}}
  refresh();const listener=(e:StorageEvent)=>{if(e.key===key||e.key===null)refresh();};window.addEventListener('storage',listener);return()=>window.removeEventListener('storage',listener);
 },[key]);
 async function locked(action:()=>Promise<void>){
  if(!key||!token||latch.current)return false;const captured={key,token};latch.current=true;setBusy(true);setError('');let success=false;
  try {
   if(!navigator.locks)throw new LocalAPIError(0,'Este navegador não oferece bloqueio entre abas. Use um navegador atualizado para gravar produção.');
   await navigator.locks.request(key,{ifAvailable:true},async lock=>{if(!lock)throw new LocalAPIError(0,'Outra aba está resolvendo produção. Aguarde e consulte novamente.');await action();success=true;});
  }catch(e){if(context.current.key===captured.key&&context.current.token===captured.token){setError(localErrorMessage(e));if(e instanceof LocalAPIError&&e.status===401)invalidate();}}
  finally{if(context.current.key===captured.key&&context.current.token===captured.token){try{setPending(readProductionPending(localStorage,key));}catch(e){setStorageError(localErrorMessage(e));setReady(false);}setBusy(false);}latch.current=false;}
  return success;
 }
 async function resolve(send=false,discard=false){
  if(!token)return;const captured={key,token};const out=await resolveProductionPending(client,token,localStorage,key,send,discard);
  if(context.current.key===captured.key&&context.current.token===captured.token)setNotice(out?.state==='confirmed'?`Operação confirmada: ${out.result!.version_id || out.result!.order_id}. Revisão ${out.result!.revision}. Consulte novamente para ver o estado atual.`:out?.state==='discarded'?'Operação não enviada ou recusada, sem resultado confirmado, removida após consulta.':'Nenhum resultado encontrado. Uma repetição explícita usará os mesmos identificadores e dados.');
 }
 const blocked=busy||!!pending||!!storageError||!ready||!canWrite;
 async function submit(prepare:()=>ProductionOperation){if(blocked)return false;return locked(async()=>{const operation=prepare();persistProductionPending(localStorage,key,operation);await resolve(true);});}
 return <Context.Provider value={{blocked,busy,submit}}>
  {!canWrite&&<p className="brand-message">Gravações de produção indisponíveis. Confira a permissão e a vigência do contrato; consultas históricas permanecem sujeitas à API.</p>}
  {error&&<p role="alert" className="brand-message">{error}</p>}{storageError&&<p role="alert">{storageError}</p>}{notice&&<p role="status">{notice}</p>}
  {pending&&<ReadSection title="Operação de produção pendente"><p className="operation-id">{pending.kind==='recipe'?'Publicação de receita':pending.kind==='order'?'Criação de ordem':'Mudança de estado'} · {pending.input.operation_id}</p><p>{pending.uncertain?'O resultado ainda pode existir no servidor. Preserve esta operação.':'Esta operação bloqueia novas gravações até ser resolvida.'}</p><div className="operation-actions"><button className="brand-secondary" disabled={busy} onClick={()=>void locked(()=>resolve())}>Consultar resultado</button><button className="brand-primary" disabled={busy||!canWrite} onClick={()=>void locked(()=>resolve(true))}>Repetir mesma operação</button><button className="brand-secondary" disabled={busy||pending.uncertain} onClick={()=>void locked(()=>resolve(false,true))}>Descartar após conferir ausência</button></div></ReadSection>}
  {children}
 </Context.Provider>;
}
