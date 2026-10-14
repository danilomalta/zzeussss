import {useEffect,useRef,useState} from 'react';
import {Link} from 'react-router-dom';
import {useTranslation} from 'react-i18next';
import states from './brazilStates.json';
import pt from '../../../i18n/map/pt-BR.json';
import en from '../../../i18n/map/en.json';
import es from '../../../i18n/map/es.json';
import fr from '../../../i18n/map/fr.json';
import zh from '../../../i18n/map/zh-CN.json';
import './distributionMap.css';

const translations:Record<string,typeof pt>={'pt-BR':pt,pt,en,es,fr,'zh-CN':zh,zh};
const extent=states.reduce((box,state)=>[
 Math.min(box[0],state.bounds[0]),Math.min(box[1],state.bounds[1]),
 Math.max(box[2],state.bounds[0]+state.bounds[2]),Math.max(box[3],state.bounds[1]+state.bounds[3])
],[Infinity,Infinity,-Infinity,-Infinity]);
const brazil:[number,number,number,number]=[extent[0]-12,extent[1]-12,extent[2]-extent[0]+24,extent[3]-extent[1]+24];
type Point={x:number;y:number};
export default function LocalDistributionMap(){
 const {i18n}=useTranslation();
 const words=translations[i18n.resolvedLanguage||i18n.language]||pt;
 const [uf,setUF]=useState(''),[zoom,setZoom]=useState(1),[focused,setFocused]=useState(false),[offset,setOffset]=useState<Point>({x:0,y:0}),[period,setPeriod]=useState('30');
 const drag=useRef<{x:number;y:number;offset:Point;moved:boolean}|null>(null);
 const ignoreClick=useRef(false);
 const svg=useRef<SVGSVGElement>(null);
 useEffect(()=>{
  const node=svg.current;
  if(!node)return;
  const wheel=(event:WheelEvent)=>{event.preventDefault();setZoom(value=>Math.max(1,Math.min(5,value+(event.deltaY<0?.35:-.35))));};
  node.addEventListener('wheel',wheel,{passive:false});
  return ()=>node.removeEventListener('wheel',wheel);
 },[]);
 const selected=states.find(state=>state.uf===uf);
 const bounds=focused&&selected?[selected.bounds[0]-8,selected.bounds[1]-8,selected.bounds[2]+16,selected.bounds[3]+16]:brazil;
 const width=bounds[2]/zoom,height=bounds[3]/zoom;
 const viewBox=`${bounds[0]+bounds[2]/2-width/2+offset.x} ${bounds[1]+bounds[3]/2-height/2+offset.y} ${width} ${height}`;
 function select(value:string){setUF(value);setZoom(1);setFocused(false);setOffset({x:0,y:0});}
 function zoomTo(next:number){setZoom(Math.max(1,Math.min(5,next)));setOffset({x:0,y:0});}
 function move(event:React.PointerEvent<SVGSVGElement>){
  if(!drag.current||!svg.current)return;
  const rect=svg.current.getBoundingClientRect();
  if(!drag.current.moved&&Math.abs(event.clientX-drag.current.x)+Math.abs(event.clientY-drag.current.y)>5){drag.current.moved=true;event.currentTarget.setPointerCapture(event.pointerId);}
  if(drag.current.moved)setOffset({x:drag.current.offset.x-(event.clientX-drag.current.x)*width/rect.width,y:drag.current.offset.y-(event.clientY-drag.current.y)*height/rect.height});
 }
 function stop(){if(drag.current?.moved){ignoreClick.current=true;setTimeout(()=>{ignoreClick.current=false;},0);}drag.current=null;}
 return <main className="brand-app-content distribution-page">
  <header className="distribution-heading"><h1>{words.title}</h1><p>{words.subtitle}</p></header>
  <div className="distribution-status"><span className="distribution-badge">{words.offline}</span><span className="distribution-locked" aria-disabled="true" title={words.pending}>🔒 {words.locations} · {words.soon}</span></div>
  <div className="distribution-layout">
   <section className="brand-panel distribution-map" aria-label={words.title}>
    <div className="distribution-toolbar"><label>{words.state}<select value={uf} onChange={event=>select(event.target.value)}><option value="">{words.all}</option>{states.map(state=><option key={state.uf} value={state.uf}>{state.uf} · {state.name}</option>)}</select></label><label>{words.period}<select value={period} onChange={event=>setPeriod(event.target.value)}><option value="today">{words.today}</option><option value="7">{words.sevenDays}</option><option value="30">{words.thirtyDays}</option></select></label></div>
    <div className="distribution-canvas">
     <svg ref={svg} className="distribution-svg" viewBox={viewBox} preserveAspectRatio="xMidYMid meet" aria-label={words.title} role="group" onPointerDown={event=>{if(event.button!==0)return;drag.current={x:event.clientX,y:event.clientY,offset,moved:false};}} onPointerMove={move} onPointerUp={stop} onPointerCancel={stop}>
      {states.map(state=><path key={state.uf} d={state.path} className={uf===state.uf?'distribution-state selected':'distribution-state'} fillRule="evenodd" tabIndex={0} role="button" aria-label={`${state.name} (${state.uf})`} aria-pressed={uf===state.uf} onClick={event=>{if(ignoreClick.current){event.preventDefault();return;}select(state.uf);}} onKeyDown={event=>{if(event.key==='Enter'||event.key===' '){event.preventDefault();select(state.uf);}}}><title>{state.name} ({state.uf})</title></path>)}
     </svg>
     <div className="distribution-controls"><button className="brand-secondary" onClick={()=>zoomTo(zoom+.5)} disabled={zoom>=5} aria-label={words.zoomIn} title={words.zoomIn}>+</button><button className="brand-secondary" onClick={()=>zoomTo(zoom-.5)} disabled={zoom<=1} aria-label={words.zoomOut} title={words.zoomOut}>−</button><button className="brand-secondary" onClick={()=>select('')} title={words.reset}>{words.reset}</button></div>
    </div>
    <div className="distribution-selection" role="status"><div><small>{words.selected}</small><strong>{selected?`${selected.name} · ${selected.uf}`:words.all}</strong></div>{selected?<button className="brand-secondary" onClick={()=>{setFocused(!focused);zoomTo(1);}}>{focused?words.overview:words.focus}</button>:<span className="distribution-tip">{words.selectFirst}</span>}</div>
    <p className="distribution-help">{words.hint}</p><p className="distribution-source">{words.source}</p>
   </section>
   <aside className="distribution-results" aria-label={words.indicators}>
    <section className="brand-panel distribution-empty"><span className="distribution-empty-icon" aria-hidden="true">◎</span><h2>{words.emptyTitle}</h2><p>{words.unavailable}</p><div className="distribution-empty-actions"><Link className="brand-secondary" to="/local/orders">{words.viewOrders}</Link></div><h3>{words.indicators}</h3><ul>{[words.purchases,words.profit,words.losses,words.issues,words.demand].map(label=><li key={label}><span>{label}</span><strong aria-label={words.noData}>—</strong></li>)}</ul></section>
    <div className="distribution-footnote"><strong>{words.legend}</strong><span>{words.legendUnavailable}</span><span>{words.noLocations}</span></div>
   </aside>
  </div>
 </main>;
}
