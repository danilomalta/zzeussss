import {createContext,useCallback,useContext,useEffect,useId,useRef,useState,type ReactNode} from 'react';
import {useBlocker} from 'react-router-dom';
import {useTranslation} from 'react-i18next';
import {cashBagWords} from '../../../core/cashBags/words.mjs';
const Context=createContext<((id:string,active:boolean)=>void)|null>(null);
export function useCustodyGuard(active:boolean){
 const register=useContext(Context),id=useId();
 if(!register)throw Error('Cash custody guard provider missing');
 useEffect(()=>{register(id,active);return()=>register(id,false);},[register,id,active]);
}
export default function CashCustodyGuard({children}:{children:ReactNode}){
 const [guards,setGuards]=useState<Map<string,boolean>>(()=>new Map()),{i18n}=useTranslation(),w=cashBagWords(i18n.resolvedLanguage||i18n.language);
 const register=useCallback((id:string,active:boolean)=>setGuards(old=>{if(old.has(id)===active)return old;const next=new Map(old);if(active)next.set(id,true);else next.delete(id);return next;}),[]);
 const active=guards.size>0,blocker=useBlocker(active),dialog=useRef<HTMLDialogElement>(null),label=useId();
 useEffect(()=>{if(blocker.state==='blocked'){if(!dialog.current?.open)dialog.current?.showModal();}else dialog.current?.close();},[blocker.state]);
 useEffect(()=>{const guard=(e:BeforeUnloadEvent)=>{if(active){e.preventDefault();e.returnValue='';}};window.addEventListener('beforeunload',guard);return()=>window.removeEventListener('beforeunload',guard);},[active]);
 return <Context.Provider value={register}>{children}<dialog ref={dialog} aria-labelledby={label} className="brand-panel" style={{maxWidth:"min(34rem, calc(100vw - 2rem))",maxHeight:"calc(100dvh - 2rem)",overflow:"auto"}} onCancel={e=>{e.preventDefault();if(blocker.state==='blocked')blocker.reset();}}><p id={label}>{w.leave}</p><button onClick={()=>{if(blocker.state==='blocked')blocker.reset();}}>{w.stay}</button><button onClick={()=>{if(blocker.state==='blocked')blocker.proceed();}}>{w.leaveAction}</button></dialog></Context.Provider>;
}
