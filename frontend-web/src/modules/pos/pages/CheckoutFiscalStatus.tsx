import {useContext,useEffect,useMemo,useRef,useState} from 'react';
import {useTranslation} from 'react-i18next';
import {checkoutFiscalScope,type CheckoutFiscalView} from '../../../core/local/checkoutFiscal.mjs';
import {checkoutFiscalClient} from '../../../core/local/checkoutFiscalService';
import {useLocalSession} from '../../../core/local/useLocalSession';
import {GatewayContext95} from '../../../core/local/service95';
import {printFiscalReceipt} from '../../../core/local/fiscalPrint.mjs';
import {words96} from '../../../core/local/words96';

export default function CheckoutFiscalStatus({checkout,canReprocess=true}:{checkout:string;canReprocess?:boolean}){
 const {i18n}=useTranslation(),w=words96(i18n.resolvedLanguage||i18n.language),token=useLocalSession(s=>s.token),session=useLocalSession(s=>s.session),demo=useContext(GatewayContext95).mode==='demo';
 const scope=token&&session?checkoutFiscalScope(session):'';
 const identity=useMemo(()=>({scope,checkout}),[scope,checkout,token]);
 const client=useMemo(()=>token&&session?checkoutFiscalClient.bind({token,session}):null,[token,session]);
 const [result,setResult]=useState<{identity:{scope:string;checkout:string};view:CheckoutFiscalView|null;busy:boolean;error:boolean}|null>(null),generation=useRef(0),abort=useRef<AbortController|null>(null),latch=useRef(false);
 async function load(action:'read'|'retry'|'recover'='read'){
  if(!client||demo||latch.current)return;latch.current=true;const g=++generation.current,c=new AbortController();abort.current?.abort();abort.current=c;
  setResult(previous=>({identity,view:previous?.identity===identity?previous.view:null,busy:true,error:false}));
  try{const view=await (action==='recover'?client.recover(checkout,c.signal):action==='retry'?client.retry(checkout,c.signal):client.read(checkout,c.signal));if(g===generation.current&&!c.signal.aborted)setResult({identity,view,busy:false,error:false});}
  catch{if(g===generation.current&&!c.signal.aborted)setResult({identity,view:null,busy:false,error:true});}
  finally{if(g===generation.current)latch.current=false;}
 }
 useEffect(()=>{latch.current=false;void load();return()=>{generation.current++;abort.current?.abort();latch.current=false;};},[identity,demo,client]);
 const [printing,setPrinting]=useState(false),[printError,setPrintError]=useState(false);
 const printTask=useRef<AbortController|null>(null);
 useEffect(()=>{setPrinting(false);setPrintError(false);return()=>{printTask.current?.abort();printTask.current=null;};},[identity]);
 async function print(paper:58|80){
  if(!client||demo||printTask.current)return;setPrinting(true);setPrintError(false);
  const controller=new AbortController();printTask.current=controller;
  try{await printFiscalReceipt({signal:controller.signal,load:()=>client.document(checkout,{width:paper,timezone:Intl.DateTimeFormat().resolvedOptions().timeZone},controller.signal)});}
  catch{if(!controller.signal.aborted)setPrintError(true);}finally{if(printTask.current===controller){printTask.current=null;setPrinting(false);}}
 }
 useEffect(()=>{setPrintError(false);},[identity]);
 const current=result?.identity===identity?result:null,view=current?.view,busy=!!current?.busy;
 return <CheckoutFiscalStatusView view={view||null} busy={busy} unavailable={!client||!!current?.error} demo={demo} canReprocess={canReprocess} connected={!!client} w={w} printing={printing} printError={printError} onPrint={paper=>void print(paper)} onRetry={()=>void load('retry')} onRecover={()=>void load('recover')} onRefresh={()=>void load()}/>;
}

export function CheckoutFiscalStatusView({view,busy,unavailable,demo,canReprocess=true,connected,w,onRetry,onRefresh,onRecover,onPrint,printing=false,printError=false}:{view:CheckoutFiscalView|null;busy:boolean;unavailable:boolean;demo:boolean;canReprocess?:boolean;connected:boolean;w:Record<string,string>;onRetry:()=>void;onRefresh:()=>void;onRecover?:()=>void;onPrint?:(paper:58|80)=>void;printing?:boolean;printError?:boolean}){
 const [paper,setPaper]=useState<58|80>(80);
 return <section className="brand-panel" aria-label={w.checkoutFiscalTitle} aria-busy={busy}>
  <strong>{w.checkoutFiscalTitle}</strong>
  <p role="status" aria-live="polite">{demo?w.checkoutFiscalDemo:unavailable?w.checkoutFiscalUnavailable:!view?w.checkoutFiscalLoading:w[{authorized:'checkoutFiscalAuthorized',failed:'checkoutFiscalFailed',pending:'checkoutFiscalPending',none:'checkoutFiscalNone',cancelled:'checkoutFiscalCancelled',cancellationFailed:'checkoutFiscalCancellationFailed'}[view.state]]}</p>
  {view?.state==='cancellationFailed'&&view.message&&<p role="alert">{view.message}</p>}
  {view?.contingencyPending&&<p role="alert">{w.checkoutFiscalContingencyPending}</p>}
  {view?.state==='authorized'&&<><p>{w.checkoutFiscalHomologation}</p>{view.archivePending&&<p role="alert">{w.checkoutFiscalArchivePending}</p>}{!view.accountantApproved&&<p role="note">{w.checkoutFiscalAccountantUnapproved}</p>}</>}
  {view?.state==='authorized'&&!demo&&<div><label>{w.checkoutFiscalPaper}<select value={paper} disabled={busy||printing} onChange={e=>setPaper(Number(e.target.value) as 58|80)}><option value={58}>58 mm</option><option value={80}>80 mm</option></select></label><button type="button" disabled={busy||printing||!connected||!onPrint} onClick={()=>onPrint?.(paper)}>{w.checkoutFiscalPrint}</button><p>{w.checkoutFiscalPrintHelp}</p></div>}
  {printError&&<p role="alert">{w.checkoutFiscalPrintUnavailable}</p>}
  {view?.state==='failed'&&canReprocess&&<><button type="button" className="brand-secondary" disabled={busy||!view.canRetry||!connected||demo} onClick={onRetry}>{w.checkoutFiscalRetry}</button>{!view.canRetry&&<p>{w.checkoutFiscalMissingRequest}</p>}</>}
  {view?.state==='pending'&&view.canRecover&&canReprocess&&!demo&&onRecover&&<><button type="button" className="brand-secondary" disabled={busy||!connected} onClick={onRecover}>{w.checkoutFiscalRecover}</button><p>{w.checkoutFiscalRecoverHelp}</p></>}
  {!demo&&<button type="button" className="brand-secondary" disabled={busy||!connected} onClick={onRefresh}>{w.checkoutFiscalRefresh}</button>}
 </section>;
}
