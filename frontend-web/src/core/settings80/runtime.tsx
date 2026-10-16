import {createContext,useCallback,useContext,useEffect,useMemo,useRef,useState,type CSSProperties,type ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import {useLocalAccess} from '../local/LocalAccess';
import {useLocalSession} from '../local/useLocalSession';
import {contextOf} from '../context74/model.mjs';
import {useTheme} from '../../shared/brand/theme';
import {defaults80,palette80,settingsKey80,validate80,type Settings80} from './model.mjs';
import {readSettings80,writeSettings80} from './storage';
import {words80} from './words';
import './settings80.css';
interface Appearance80 {stored:Settings80;prefs:Settings80;photoWarning:string;loaded:boolean;save:(value:Settings80)=>Promise<void>;preview:(value:Settings80)=>void;clearPreview:()=>void;}
const Context=createContext<Appearance80|null>(null);
export const useAppearance80=()=>useContext(Context);
export function AppearanceProvider80({children}:{children:ReactNode}){
 const {capabilities}=useLocalAccess(),session=useLocalSession(s=>s.session),token=useLocalSession(s=>s.token),{theme}=useTheme(),{i18n}=useTranslation();
 const base=useRef(defaults80()).current;
 let scope='';try{if(token)scope=settingsKey80(contextOf(session,capabilities));}catch{/* Context gate handles invalid sessions. */}
 const [data,setData]=useState<{scope:string;value:Settings80}|null>(null),[preview,setPreview]=useState<{scope:string;value:Settings80}|null>(null);
 const stored=data?.scope===scope?data.value:base,prefs=preview?.scope===scope?preview.value:stored;
 const loaded=!!scope&&data?.scope===scope;
 const photo=prefs.mode==='photo'&&!!prefs.background;
 useEffect(()=>{
  if(!scope)return;
  let live=true;
  readSettings80(scope).then(value=>{if(live)setData({scope,value:value||defaults80()});}).catch(()=>{if(live)setData({scope,value:defaults80()});});
  return ()=>{live=false;};
 },[scope]);
 const clearPreview=useCallback(()=>setPreview(null),[]);
 const showPreview=useCallback((value:Settings80)=>{if(!scope)return;setPreview({scope,value:validate80(value)});},[scope]);
 const save=useCallback(async(value:Settings80)=>{
  if(!scope||!token)throw Error('context');
  const next=validate80(value);
  if(capabilities?.role!=='owner'&&(JSON.stringify(next.fiscal)!==JSON.stringify(stored.fiscal)||JSON.stringify(next.equipment)!==JSON.stringify(stored.equipment)))throw Error('permission');
  await writeSettings80(scope,next);setData({scope,value:next});setPreview(null);
 },[scope,token,capabilities?.role,stored]);
 useEffect(()=>{
  if(!photo)return;
  const root=document.documentElement,oldTheme=root.dataset.theme,oldScheme=root.style.colorScheme;
  root.dataset.appearance='photo';root.dataset.theme='dark';root.style.colorScheme='dark';
  return ()=>{delete root.dataset.appearance;if(oldTheme)root.dataset.theme=oldTheme;else delete root.dataset.theme;root.style.colorScheme=oldScheme;};
 },[photo]);
 const style=useMemo(()=>{
  const vars:Record<string,string>={};
  if(photo){
   Object.assign(vars,{'--bg':'#0b0b0d','--surface':prefs.lowEffects?'rgba(20,20,23,.96)':'rgba(20,20,23,.88)','--border':'rgba(255,255,255,.24)','--text':'#f5f5f4','--text-muted':'#d1d1ce','--btn-bg':'#fafaf9','--btn-text':'#0b0b0d','--brand-shadow':'0 12px 40px rgba(0,0,0,.3)','--wallpaper80':"url("+prefs.background+")",'--wallpaper-fit80':prefs.backgroundFit});
  }
  const palette=palette80(prefs.accent,photo||theme==='dark');
  if(palette)Object.assign(vars,{'--accent':palette.accent,'--focus':palette.accent,'--btn-bg':palette.accent,'--btn-text':palette.buttonText,'--accent-soft':palette.accent+'24'});
  Object.assign(vars,{'--brand-bg':'var(--bg)','--brand-surface':'var(--surface)','--brand-border':'var(--border)','--brand-ink':'var(--text)','--brand-muted':'var(--text-muted)'});
  return vars as CSSProperties;
 },[prefs,theme,photo]);
 const words=words80(i18n.resolvedLanguage||i18n.language);
 return <Context.Provider value={{stored,prefs,photoWarning:words.photoWarning,loaded,save,preview:showPreview,clearPreview}}><div className={'appearance80-scope'+(photo?' appearance80-photo':'')+(photo&&prefs.backgroundBlur===3?' appearance80-photo-soft':'')+(prefs.lowEffects?' appearance80-reduced':'')+(prefs.compact?' appearance80-compact':'')} style={style}>{children}</div></Context.Provider>;
}
