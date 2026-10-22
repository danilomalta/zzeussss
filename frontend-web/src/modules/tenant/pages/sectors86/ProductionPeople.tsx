import {createContext,useContext,useEffect,useRef,useState,type ReactNode} from 'react';
import type {ProductionCommands} from '../../../../core/productionRead/commands.mjs';
import type {ProductionPeople as People} from '../../../../core/productionRead/responsibles.mjs';
import {responsibleWords} from '../../../../core/productionRead/responsibleWords';
import {productionCommandWords} from '../../../../core/productionRead/commandWords';
const Context=createContext<{source:ProductionCommands;people:People|null;loading:boolean;error:boolean;load:()=>void}|null>(null);
export function ProductionPeople({source,children}:{source:ProductionCommands;children:ReactNode}){
 const [people,setPeople]=useState<People|null>(null),[loading,setLoading]=useState(false),[error,setError]=useState(false),controller=useRef<AbortController|null>(null),generation=useRef(0);
 useEffect(()=>{setPeople(null);setError(false);setLoading(false);return()=>{generation.current++;controller.current?.abort();controller.current=null;};},[source]);
 const load=()=>{if(controller.current||!source.canListResponsibles)return;const c=new AbortController(),g=++generation.current;controller.current=c;setLoading(true);setPeople(null);setError(false);source.responsibles(c.signal).then(v=>{if(g===generation.current)setPeople(v);}).catch(()=>{if(g===generation.current)setError(true);}).finally(()=>{if(g===generation.current){setLoading(false);controller.current=null;}});};
 return <Context.Provider value={{source,people,loading,error,load}}>{children}</Context.Provider>;
}
export function ResponsibleSelect({actor,value,onChange,locale,disabled,label}:{actor:string;value:string;onChange:(value:string)=>void;locale:string;disabled:boolean;label?:string}){
 const context=useContext(Context),w=responsibleWords(locale),c=productionCommandWords(locale),people=context?.people?.items||[],known=people.some(p=>p.id===value)||value===actor,canList=!!context?.source.canListResponsibles;
 return <div><label>{label||w.label}<select value={value} disabled={disabled} onChange={e=>{if(e.target.value===actor||canList&&people.some(p=>p.id===e.target.value))onChange(e.target.value);}}><option value={actor}>{w.self} · {actor}</option>{!known&&<option value={value}>{value} · {w.retained}</option>}{canList&&people.filter(p=>p.id!==actor).map(p=><option key={p.id} value={p.id}>{p.name} · {p.id}</option>)}</select></label><p>{w.help}</p>{canList?<button type="button" className="brand-secondary" disabled={disabled||context?.loading} onClick={context?.load}>{context?.loading?c.busy:w.load}</button>:<p>{w.restricted}</p>}{context?.error&&<p role="alert">{c.unavailable}</p>}{context?.people&&<p>{w.limit}</p>}</div>;
}
