import {useEffect,useRef,useState} from 'react';
import {useLocation} from 'react-router-dom';
import {useTranslation} from 'react-i18next';
import {moveFocus68} from './model.mjs';
import {words68} from './words.mjs';
import './recovery68.css';

export default function Navigation68(){
 const location=useLocation(),lastPath=useRef(location.pathname),{i18n}=useTranslation();
 const w=words68(i18n.resolvedLanguage||i18n.language);
 const [offline,setOffline]=useState(()=>navigator.onLine===false),[announcement,setAnnouncement]=useState('');
 useEffect(()=>{const update=()=>setOffline(navigator.onLine===false);window.addEventListener('online',update);window.addEventListener('offline',update);return()=>{window.removeEventListener('online',update);window.removeEventListener('offline',update);};},[]);
 useEffect(()=>{
  if(lastPath.current===location.pathname)return;
  lastPath.current=location.pathname;setAnnouncement('');
  if(location.hash)return; // Preserve explicit anchor navigation.
  const container=document.getElementById('titan-content68');if(!container)return;
  const initial=document.activeElement;let observer:MutationObserver|undefined;let done=false;
  const cancel=()=>{done=true;observer?.disconnect();};
  window.addEventListener('pointerdown',cancel,{once:true});window.addEventListener('keydown',cancel,{once:true});
  const focus=()=>{if(done)return;if(moveFocus68(container,initial,document.activeElement,document.body)){done=true;setAnnouncement(w.arrived);observer?.disconnect();}};
  // Wait until the next frame: the new route must replace the previous view first.
  const frame=requestAnimationFrame(()=>{focus();if(!done){observer=new MutationObserver(focus);observer.observe(container,{childList:true,subtree:true});}});
  const timeout=window.setTimeout(()=>{done=true;observer?.disconnect();},15000);
  return()=>{done=true;cancelAnimationFrame(frame);window.clearTimeout(timeout);observer?.disconnect();window.removeEventListener('pointerdown',cancel);window.removeEventListener('keydown',cancel);};
 },[location.pathname,location.hash,w.arrived]);
 return <>
  <a className="skip68" href="#titan-content68" onClick={event=>{event.preventDefault();document.getElementById('titan-content68')?.focus();}}>{w.skip}</a>
  <span className="sr-only68" role="status" aria-live="polite">{announcement}</span>
  {offline&&<aside className="network68" role="status">{w.offline}</aside>}
 </>;
}
