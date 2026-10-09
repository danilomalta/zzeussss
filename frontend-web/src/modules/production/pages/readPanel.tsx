import {useEffect,useRef,useState} from 'react';
import type {ReactNode} from 'react';
import {LocalAPIError} from '../../../core/local/localClient.mjs';
import {useLocalSession,localErrorMessage} from '../../../core/local/useLocalSession';
import './productionRead.css';

export function useReadTask<T>() {
  const token=useLocalSession(s=>s.token), invalidate=useLocalSession(s=>s.invalidate);
  const [data,setData]=useState<T|null>(null),[error,setError]=useState(''),[busy,setBusy]=useState(false);
  const revision=useRef(0),currentToken=useRef(token);currentToken.current=token;
  useEffect(()=>{revision.current++;setData(null);setError('');setBusy(false);return()=>{revision.current++;};},[token]);
  function clear(){revision.current++;setData(null);setError('');setBusy(false);}
  async function run(action:(token:string)=>Promise<T>){
    if(!token)return;const n=++revision.current;setData(null);setError('');setBusy(true);
    try {const value=await action(token);if(n===revision.current && currentToken.current===token)setData(value);}
    catch(e){if(n===revision.current && currentToken.current===token){setError(localErrorMessage(e));if(e instanceof LocalAPIError && e.status===401)invalidate();}}
    finally{if(n===revision.current && currentToken.current===token)setBusy(false);}
  }
  return {data,error,busy,run,clear};
}
export function ReadState({task}:{task:{error:string;busy:boolean}}){return <>{task.busy&&<p role="status">Consultando a instalação…</p>}{task.error&&<p className="brand-message" role="alert">{task.error}</p>}</>;}
export function ReadSection({title,children}:{title:string;children:ReactNode}){return <section className="brand-panel operation-panel"><h2>{title}</h2>{children}</section>;}
export function Pages({offset,more,busy,change}:{offset:number;more:boolean;busy:boolean;change:(value:number)=>void}){return <nav aria-label="Páginas" className="operation-actions"><button className="brand-secondary" disabled={busy||offset===0} onClick={()=>change(Math.max(0,offset-50))}>Anterior</button><span>Itens a partir de {offset+1}</span><button className="brand-secondary" disabled={busy||!more||offset>Number.MAX_SAFE_INTEGER-50} onClick={()=>change(offset+50)}>Próxima</button></nav>;}
