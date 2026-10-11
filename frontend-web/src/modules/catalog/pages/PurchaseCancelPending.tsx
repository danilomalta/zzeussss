import {createContext,useContext,useEffect,useRef,useState} from 'react';
import type {ReactNode} from 'react';
import {useLocalSession,localErrorMessage} from '../../../core/local/useLocalSession';
import {useLocalAccess} from '../../../core/local/LocalAccess';
import {LocalAPIError} from '../../../core/local/localClient.mjs';
import {createPurchaseCancelClient,purchaseCancelPendingKey,readPurchaseCancelPending,persistPurchaseCancelPending,resolvePurchaseCancelPending} from '../../../core/local/purchaseCancel.mjs';
import type {CancelOperation,PendingCancel} from '../../../core/local/purchaseCancel.mjs';
import {ReadSection} from '../../production/pages/readPanel';
const client=createPurchaseCancelClient();
interface Mutations {blocked:boolean;busy:boolean;submit:(prepare:()=>CancelOperation)=>Promise<boolean>}
const Context=createContext<Mutations>({blocked:true,busy:false,submit:async()=>false});
export const usePurchaseCancel=()=>useContext(Context);
export function PurchaseCancelPending({children}:{children:ReactNode}){
 const token=useLocalSession(s=>s.token),session=useLocalSession(s=>s.session),invalidate=useLocalSession(s=>s.invalidate);
 const {capabilities}=useLocalAccess();
 const canWrite=!!capabilities?.permissions.includes('manage_replenishment') && capabilities.license.state==='active' && capabilities.license.modules.includes('orders');
 const key=session?purchaseCancelPendingKey(session):'';
 const [pending,setPending]=useState<PendingCancel|null>(null),[busy,setBusy]=useState(false),[error,setError]=useState(''),[notice,setNotice]=useState(''),[storageError,setStorageError]=useState(''),[ready,setReady]=useState(false);
 const latch=useRef(false),context=useRef({key,token});context.current={key,token};
 useEffect(()=>{setPending(null);setError('');setNotice('');setBusy(false);setReady(false);
  function refresh(){try{setPending(readPurchaseCancelPending(localStorage,key));setStorageError('');setReady(true);}catch(e){setStorageError(localErrorMessage(e));setReady(false);}}
  refresh();const listener=(e:StorageEvent)=>{if(e.key===key||e.key===null)refresh();};window.addEventListener('storage',listener);return()=>window.removeEventListener('storage',listener);
 },[key]);
 async function locked(action:()=>Promise<void>){
  if(!key||!token||latch.current)return false;const captured={key,token};latch.current=true;setBusy(true);setError('');let success=false;
  try {
   if(!navigator.locks)throw new LocalAPIError(0,'Este navegador não oferece bloqueio entre abas. Use um navegador atualizado para gravar cancelamento de pedidos.');
   await navigator.locks.request(key,{ifAvailable:true},async lock=>{if(!lock)throw new LocalAPIError(0,'Outra aba está resolvendo cancelamento de pedidos. Aguarde e consulte novamente.');await action();success=true;});
  }catch(e){if(context.current.key===captured.key&&context.current.token===captured.token){setError(localErrorMessage(e));if(e instanceof LocalAPIError&&e.status===401)invalidate();}}
  finally{if(context.current.key===captured.key&&context.current.token===captured.token){try{setPending(readPurchaseCancelPending(localStorage,key));}catch(e){setStorageError(localErrorMessage(e));setReady(false);}setBusy(false);}latch.current=false;}
  return success;
 }
 async function resolve(send=false,discard=false){
  if(!token)return;const captured={key,token};const out=await resolvePurchaseCancelPending(client,token,localStorage,key,send,discard);
  if(context.current.key===captured.key&&context.current.token===captured.token)setNotice(out?.state==='confirmed'?`Operação confirmada: ${out.result!.order_id}. Consulte novamente para ver o estado atual.`:out?.state==='discarded'?'Operação não enviada ou recusada, sem resultado confirmado, removida após consulta.':'Nenhum resultado encontrado. Uma repetição explícita usará os mesmos identificadores e dados.');
 }
 const blocked=busy||!!pending||!!storageError||!ready||!canWrite;
 async function submit(prepare:()=>CancelOperation){if(blocked)return false;return locked(async()=>{const operation=prepare();persistPurchaseCancelPending(localStorage,key,operation);await resolve(true);});}
 return <Context.Provider value={{blocked,busy,submit}}>
  {!canWrite&&<p className="brand-message">Gravações de cancelamento de pedidos indisponíveis. Confira a permissão e a vigência do contrato; consultas históricas permanecem sujeitas à API.</p>}
  {error&&<p role="alert" className="brand-message">{error}</p>}{storageError&&<p role="alert">{storageError}</p>}{notice&&<p role="status">{notice}</p>}
  {pending&&<ReadSection title="Operação de cancelamento de pedidos pendente"><p className="operation-id">{'Cancelamento de pedido'} · {pending.input.operation_id}</p><p>{pending.uncertain?'O resultado ainda pode existir no servidor. Preserve esta operação.':'Esta operação bloqueia novas gravações até ser resolvida.'}</p><div className="operation-actions"><button className="brand-secondary" disabled={busy} onClick={()=>void locked(()=>resolve())}>Consultar resultado</button><button className="brand-primary" disabled={busy||!canWrite} onClick={()=>void locked(()=>resolve(true))}>Repetir mesma operação</button><button className="brand-secondary" disabled={busy||pending.uncertain} onClick={()=>void locked(()=>resolve(false,true))}>Descartar após conferir ausência</button></div></ReadSection>}
  {children}
 </Context.Provider>;
}
