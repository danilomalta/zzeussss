import {useContext,useEffect,useMemo,useRef,useState} from 'react';
import {useTranslation} from 'react-i18next';
import {checkoutFiscalScope,type CheckoutFiscalView} from '../../../core/local/checkoutFiscal.mjs';
import {checkoutFiscalClient} from '../../../core/local/checkoutFiscalService';
import {useLocalSession} from '../../../core/local/useLocalSession';
import {GatewayContext95} from '../../../core/local/service95';
import {words96} from '../../../core/local/words96';

export default function CheckoutFiscalStatus({checkout,canReprocess=true}:{checkout:string;canReprocess?:boolean}){
 const {i18n}=useTranslation(),w=words96(i18n.resolvedLanguage||i18n.language),token=useLocalSession(s=>s.token),session=useLocalSession(s=>s.session),demo=useContext(GatewayContext95).mode==='demo';
 const scope=token&&session?checkoutFiscalScope(session):'';
 const identity=useMemo(()=>({scope,checkout}),[scope,checkout,token]);
 const client=useMemo(()=>token&&session?checkoutFiscalClient.bind({token,session}):null,[token,session]);
 const [result,setResult]=useState<{identity:{scope:string;checkout:string};view:CheckoutFiscalView|null;busy:boolean;error:boolean}|null>(null),generation=useRef(0),abort=useRef<AbortController|null>(null),latch=useRef(false);
 async function load(retry=false){
  if(!client||demo||latch.current)return;latch.current=true;const g=++generation.current,c=new AbortController();abort.current?.abort();abort.current=c;
  setResult(previous=>({identity,view:previous?.identity===identity?previous.view:null,busy:true,error:false}));
  try{const view=await (retry?client.retry(checkout,c.signal):client.read(checkout,c.signal));if(g===generation.current&&!c.signal.aborted)setResult({identity,view,busy:false,error:false});}
  catch{if(g===generation.current&&!c.signal.aborted)setResult({identity,view:null,busy:false,error:true});}
  finally{if(g===generation.current)latch.current=false;}
 }
 useEffect(()=>{latch.current=false;void load();return()=>{generation.current++;abort.current?.abort();latch.current=false;};},[identity,demo,client]);
 const current=result?.identity===identity?result:null,view=current?.view,busy=!!current?.busy;
 return <CheckoutFiscalStatusView view={view||null} busy={busy} unavailable={!client||!!current?.error} demo={demo} canReprocess={canReprocess} connected={!!client} w={w} onRetry={()=>void load(true)} onRefresh={()=>void load()}/>;
}

export function CheckoutFiscalStatusView({view,busy,unavailable,demo,canReprocess=true,connected,w,onRetry,onRefresh}:{view:CheckoutFiscalView|null;busy:boolean;unavailable:boolean;demo:boolean;canReprocess?:boolean;connected:boolean;w:Record<string,string>;onRetry:()=>void;onRefresh:()=>void}){
 return <section className="brand-panel" aria-label={w.checkoutFiscalTitle} aria-busy={busy}>
  <strong>{w.checkoutFiscalTitle}</strong>
  <p role="status" aria-live="polite">{demo?w.checkoutFiscalDemo:unavailable?w.checkoutFiscalUnavailable:!view?w.checkoutFiscalLoading:w[{authorized:'checkoutFiscalAuthorized',failed:'checkoutFiscalFailed',pending:'checkoutFiscalPending',none:'checkoutFiscalNone',cancelled:'checkoutFiscalCancelled',cancellationFailed:'checkoutFiscalCancellationFailed'}[view.state]]}</p>
  {view?.state==='cancellationFailed'&&view.message&&<p role="alert">{view.message}</p>}
  {view?.state==='authorized'&&<p>{w.checkoutFiscalHomologation}</p>}
  {view?.state==='failed'&&canReprocess&&<><button type="button" className="brand-secondary" disabled={busy||!view.canRetry||!connected||demo} onClick={onRetry}>{w.checkoutFiscalRetry}</button>{!view.canRetry&&<p>{w.checkoutFiscalMissingRequest}</p>}</>}
  {!demo&&<button type="button" className="brand-secondary" disabled={busy||!connected} onClick={onRefresh}>{w.checkoutFiscalRefresh}</button>}
 </section>;
}
