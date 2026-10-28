import React,{useEffect,useRef,useState} from 'react';
import {isRouteErrorResponse,Link,useInRouterContext,useLocation,useRouteError} from 'react-router-dom';
import {useTranslation} from 'react-i18next';
import {diagnostic68,moveFocus68,recovery68} from './model.mjs';
import {words68} from './words.mjs';
import './recovery68.css';

export function RecoveryPanel68({path,notFound=false}:{path:string;notFound?:boolean}){
 const {i18n}=useTranslation();const w=words68(i18n.resolvedLanguage||i18n.language);
 const inRouter=useInRouterContext(),info=recovery68(path,notFound),[feedback,setFeedback]=useState('');
 const report=useRef<string>();
 const panel=useRef<HTMLElement>(null);
 useEffect(()=>{moveFocus68(panel.current,document.body,document.activeElement,document.body);},[]);
 async function copy(){
  try {report.current??=diagnostic68(info);await navigator.clipboard.writeText(report.current);setFeedback(w.copied);}
  catch {setFeedback(w.copyFailed);}
 }
 function reload(){if(window.confirm(w.confirm))window.location.reload();}
 return <main ref={panel} className="recovery68 brand-panel" aria-labelledby="recovery68-title">
  <h1 id="recovery68-title">{notFound?w.missing:w.title}</h1>
  <p>{w.detail}</p><p>{w.pending}</p>
  <p>{w.reference}: <strong>{info.code}</strong></p>
  <div className="recovery68-actions">
   {inRouter?<Link className="brand-primary" to={info.home}>{w.home}</Link>:<a className="brand-primary" href={info.home}>{w.home}</a>}
   <button type="button" className="brand-secondary" onClick={reload}>{w.reload}</button>
   <button type="button" className="brand-secondary" onClick={()=>void copy()}>{w.report}</button>
  </div>
  <p role="status" aria-live="polite">{feedback}</p><p>{w.note}</p>
 </main>;
}
export function RouteRecovery68(){
 const error=useRouteError(),location=useLocation();
 return <RecoveryPanel68 path={location.pathname} notFound={isRouteErrorResponse(error)&&error.status===404}/>;
}
export class ApplicationBoundary68 extends React.Component<{children:React.ReactNode},{failed:boolean}>{
 state={failed:false};
 static getDerivedStateFromError(){return {failed:true};}
 render(){return this.state.failed?<RecoveryPanel68 path={window.location.pathname}/>:this.props.children;}
}
