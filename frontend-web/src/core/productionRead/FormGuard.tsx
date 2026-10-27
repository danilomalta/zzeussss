import {createContext,useContext,useId,useLayoutEffect,useMemo,type ReactNode} from 'react';
import {createDraftRegistry} from './formGuard.mjs';
type Guard=ReturnType<typeof createDraftRegistry>&{protect:(run:()=>void)=>void;protectSelection:(run:()=>void)=>void};
const Context=createContext<Guard|null>(null);
export function ProductionFormGuard({children,onChange,onLeave}:{children:ReactNode;onChange:(dirty:boolean)=>void;onLeave:(run:()=>void)=>void}){
 const registry=useMemo(()=>createDraftRegistry(onChange),[onChange]);
 const value=useMemo(()=>({...registry,protect:(run:()=>void)=>{if(registry.has())onLeave(run);else run();},protectSelection:(run:()=>void)=>{if(registry.hasSelection())onLeave(run);else run();}}),[registry,onLeave]);
 return <Context.Provider value={value}>{children}</Context.Provider>;
}
export function useProductionDraft(dirty:boolean,warnOnSelection=true){
 const registry=useContext(Context),id=useId(),set=registry?.set;
 useLayoutEffect(()=>{set?.(id,dirty,warnOnSelection);return()=>set?.(id,false);},[set,id,dirty,warnOnSelection]);
 return registry?.protect||((run:()=>void)=>run());
}

export function useProductionSelectionGuard(){return useContext(Context)?.protectSelection||((run:()=>void)=>run());}
