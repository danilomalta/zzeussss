import {createContext,useCallback,useContext,useEffect,useId,useState,type ReactNode} from 'react';
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
 const active=guards.size>0,blocker=useBlocker(active);
 useEffect(()=>{const guard=(e:BeforeUnloadEvent)=>{if(active){e.preventDefault();e.returnValue='';}};window.addEventListener('beforeunload',guard);return()=>window.removeEventListener('beforeunload',guard);},[active]);
 return <Context.Provider value={register}>{children}{blocker.state==='blocked'&&<div role="dialog" aria-modal="true" className="brand-panel"><p>{w.leave}</p><button onClick={()=>blocker.reset()}>{w.stay}</button><button onClick={()=>blocker.proceed()}>{w.leaveAction}</button></div>}</Context.Provider>;
}
