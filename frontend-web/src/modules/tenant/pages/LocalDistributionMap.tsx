import {useState} from 'react';
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
export default function LocalDistributionMap(){
 const {i18n}=useTranslation();
 const words=translations[i18n.resolvedLanguage||i18n.language]||pt;
 const [uf,setUF]=useState(''),[zoom,setZoom]=useState(1),[focused,setFocused]=useState(false);
 const selected=states.find(state=>state.uf===uf);
 const base=focused&&selected?selected.bounds:[0,0,660,620];
 const width=(base[2]+40)/zoom,height=(base[3]+40)/zoom;
 const viewBox=`${base[0]+base[2]/2-width/2} ${base[1]+base[3]/2-height/2} ${width} ${height}`;
 function select(value:string){setUF(value);setZoom(1);setFocused(false);}
 function reset(){setUF('');setZoom(1);setFocused(false);}
 const indicators=[words.purchases,words.profit,words.losses,words.issues,words.demand];
 return <main className="brand-app-content distribution-page">
  <header className="distribution-heading"><div><h1>{words.title}</h1><p>{words.subtitle}</p></div><Link className="brand-secondary" to="/local/home">← {words.back}</Link></header>
  <div className="distribution-status"><span className="distribution-badge">{words.offline}</span><span>{words.pending}</span></div>
  <div className="distribution-layout">
   <section className="brand-panel distribution-map" aria-label={words.title}>
    <div className="distribution-toolbar"><label>{words.state}<select value={uf} onChange={event=>select(event.target.value)}><option value="">{words.all}</option>{states.map(state=><option key={state.uf} value={state.uf}>{state.uf} · {state.name}</option>)}</select></label><div className="distribution-controls"><button className="brand-secondary" onClick={()=>setZoom(value=>Math.min(4,value+.5))} disabled={zoom>=4} aria-label={words.zoomIn} title={words.zoomIn}>+</button><button className="brand-secondary" onClick={()=>setZoom(value=>Math.max(1,value-.5))} disabled={zoom<=1} aria-label={words.zoomOut} title={words.zoomOut}>−</button><button className="brand-secondary" onClick={reset}>{words.reset}</button></div></div>
    <svg className="distribution-svg" viewBox={viewBox} aria-label={words.title} role="group">
     {states.map(state=><path key={state.uf} d={state.path} className={uf===state.uf?'distribution-state selected':'distribution-state'} fillRule="evenodd" tabIndex={0} role="button" aria-label={`${state.name} (${state.uf})`} aria-pressed={uf===state.uf} onClick={()=>select(state.uf)} onKeyDown={event=>{if(event.key==='Enter'||event.key===' '){event.preventDefault();select(state.uf);}}}><title>{state.name} ({state.uf})</title></path>)}
    </svg>
    <div className="distribution-selection" role="status"><div><small>{words.selected}</small><strong>{selected?`${selected.name} · ${selected.uf}`:words.all}</strong></div><button className="brand-secondary" disabled={!selected} onClick={()=>{setFocused(!focused);setZoom(1);}}>{focused?words.overview:words.focus}</button></div>
    <p className="distribution-help">{words.hint}</p><p className="distribution-source">{words.source}</p>
   </section>
   <aside className="distribution-results" aria-label={words.indicators}><div className="brand-panel distribution-explanation"><h2>{words.indicators}</h2><p>{words.unavailable}</p></div>{indicators.map((title,index)=><section key={index} className="brand-panel distribution-indicator"><span className="distribution-indicator-icon" aria-hidden="true">{['↗','◇','!','⇄','▤'][index]}</span><div><h2>{title}</h2><p>{words.noData}</p></div><span className="distribution-missing" aria-label={words.noData}>—</span></section>)}</aside>
  </div>
  <section className="brand-panel distribution-locations"><span aria-hidden="true">⌖</span><div><h2>{words.lastLocation}</h2><p>{words.noLocations}</p><p className="distribution-help">{words.locationHelp}</p></div></section>
 </main>;
}
