import {useCallback,useMemo,useRef,useState} from 'react';
import {useTranslation} from 'react-i18next';
import {useBlocker,useNavigate} from 'react-router-dom';
import {stageReport76} from '../../../core/loss76/report76.mjs';
import '../../../core/loss76/sessionReport76';
import {sourceAllowed} from '../../../core/sheets70/model.mjs';
import {useEffect} from 'react';
import {useLocalAccess} from '../../../core/local/LocalAccess';
import {useLocalSession} from '../../../core/local/useLocalSession';
import {contextOf,contextKey} from '../../../core/context74/model.mjs';
import {createStockWorkspace} from '../../../core/stockWorkspace/source.mjs';
import {stockWords} from '../../../core/stockWorkspace/words';
import StockWorkspace from './StockWorkspace';
import ReviewedInventory from './ReviewedInventory';
import {createReviewedInventory} from '../../../core/reviewedInventory/source.mjs';
import {reviewedWords} from '../../../core/reviewedInventory/words';
const storage={getItem:(k:string)=>window.localStorage.getItem(k),setItem:(k:string,v:string)=>window.localStorage.setItem(k,v),removeItem:(k:string)=>window.localStorage.removeItem(k)} as Storage;
export default function LocalStock(){
 const navigate=useNavigate();
 const token=useLocalSession(s=>s.token),session=useLocalSession(s=>s.session),invalidate=useLocalSession(s=>s.invalidate),{capabilities}=useLocalAccess(),{i18n}=useTranslation(),locale=i18n.resolvedLanguage||i18n.language,w=stockWords(locale),[stockDirty,setStockDirty]=useState(false),[inventoryDirty,setInventoryDirty]=useState(false),capRef=useRef(capabilities);capRef.current=capabilities;
 let scope='';try{scope=contextKey(contextOf(session,capabilities));}catch{/* A trusted scope is required. */}
 const source=useMemo(()=>{try{return token&&session&&capabilities&&scope?createStockWorkspace({token,session,capabilities,storage,locks:navigator.locks,isCurrent:()=>{const now=useLocalSession.getState();return now.token===token&&now.session===session&&capRef.current===capabilities;}}):null;}catch{return null;}},[token,session,capabilities,scope]);
 const dirty=stockDirty||inventoryDirty;
 const inventory=useMemo(()=>{try{return source&&token&&session&&capabilities?createReviewedInventory({token,session,capabilities,storage,locks:navigator.locks,directory:source,isCurrent:()=>{const now=useLocalSession.getState();return now.token===token&&now.session===session&&capRef.current===capabilities;}}):null;}catch{return null;}},[source,token,session,capabilities]);
 const leave=inventoryDirty?reviewedWords(locale).leave:w.leave;
 const unauthorized=useCallback(()=>{const now=useLocalSession.getState();if(now.token===token&&now.session===session&&capRef.current===capabilities)invalidate();},[token,session,capabilities,invalidate]);
 const blocker=useBlocker(({currentLocation,nextLocation})=>dirty&&(currentLocation.pathname!==nextLocation.pathname||currentLocation.search!==nextLocation.search||currentLocation.hash!==nextLocation.hash));useEffect(()=>{if(blocker.state==='blocked'){if(window.confirm(leave))blocker.proceed();else blocker.reset();}},[blocker,leave]);
 if(!source)return <main className="brand-app-content"><h1>{w.title}</h1><p>{w.denied}</p></main>;
 return <main className="brand-app-content">{inventory&&<ReviewedInventory key={"inventory"+scope+token+JSON.stringify(capabilities)} source={inventory} locale={locale} onUnauthorized={unauthorized} onDirty={setInventoryDirty}/>}<StockWorkspace key={scope+token+JSON.stringify(capabilities)} source={source} locale={locale} onUnauthorized={unauthorized} onDirty={setStockDirty} onSheets={capabilities?.role==='owner'&&sourceAllowed('stock',capabilities)?(matrix,title)=>{const now=useLocalSession.getState();if(now.token!==token||now.session!==session||capRef.current!==capabilities)return;const id=crypto.randomUUID();stageReport76(scope,id,title,matrix);navigate('/local/sheets?report='+encodeURIComponent(id));}:undefined}/></main>;
}
