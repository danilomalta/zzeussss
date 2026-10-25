import Operations105 from './requests105/Operations105';
import Suspended104 from './Suspended104';
import Returns100 from './Returns100';
import Ledger99 from './Ledger99';import Bag99 from './Bag99';
import {useEffect,useState} from 'react';import {Link} from 'react-router-dom';import {useCustodyGuard} from './CashCustodyGuard';import {useTranslation} from 'react-i18next';
import {words94} from '../../../core/local/words94';import {words95} from '../../../core/local/words95';import {manager94,resume94,type Actor94,type Till94 as Data94} from '../../../core/local/till94.mjs';
import {useTill94} from '../../../core/local/useTill94';import {type CartItem} from '../../../core/local/posModel.mjs';
import type {LocalProduct} from '../../../core/local/localClient.mjs';
import './till94.css';
import {listStation94,sameStation94} from '../../../core/local/tillStorage94';
type Store94=ReturnType<typeof useTill94>;
export const tillTabs94=['sale','suspended','movements','approvals','returns','bag','journal','complete','connections'];
export function TillNav94({tab,onChange,disabled=false,closeBlocked=false,onCloseBlocked}:{tab:string;onChange:(tab:string)=>void;disabled?:boolean;closeBlocked?:boolean;onCloseBlocked?:()=>void}){
 const {i18n}=useTranslation(),w=words95(i18n.resolvedLanguage||i18n.language);return <nav className="till94-nav" aria-label={w.operations}>{tillTabs94.map(key=><button type="button" disabled={disabled} key={key} aria-current={tab===key?'page':undefined} onClick={()=>onChange(key)}>{w[key]}</button>)}<Link to="/local/cash/register" aria-disabled={disabled||closeBlocked} onClick={e=>{if(disabled||closeBlocked){e.preventDefault();onCloseBlocked?.();}}}>{w.closing}</Link></nav>;}
export default function Till94({store:primaryStore,actor,tab,token,reference,gross,products,onDirty,onResume}:{store:Store94;actor:Actor94;tab:string;token:string;reference:string;gross:number;products:LocalProduct[];onDirty:(dirty:boolean)=>void;onResume:(cart:CartItem[],reference:string)=>void}){
 const manager=manager94(actor),[selectedWorkspace,setSelectedWorkspace]=useState(''),[workspaces,setWorkspaces]=useState<{scope:string;updated:string;requests:number;bags:number}[]>([]);
 const selected=manager&&sameStation94(primaryStore.data.scope,selectedWorkspace)?selectedWorkspace:'';
 const alternate=useTill94(selected||primaryStore.data.scope,primaryStore.data.cash),store=selected?alternate:primaryStore;
 const {i18n}=useTranslation(),w=words94(i18n.resolvedLanguage||i18n.language),locale=i18n.resolvedLanguage||i18n.language,date=(s:string)=>Number.isFinite(Date.parse(s))?new Intl.DateTimeFormat(locale,{dateStyle:'short',timeStyle:'short'}).format(new Date(s)):'—',{data,mutate}=store;
 const [dirty,setDirty]=useState(false),[notice,setNotice]=useState('');
 const locked=store.busy||!store.ready||!data.cash||data.scope!==(selected||primaryStore.data.scope),formLocked=locked||!!selected;useCustodyGuard(dirty||store.busy);
 useEffect(()=>{let active=true;if(!manager){setSelectedWorkspace("");setWorkspaces([]);return;}void listStation94(primaryStore.data.scope,primaryStore.data.cash,actor).then(rows=>{if(active)setWorkspaces(rows);}).catch(()=>{});return()=>{active=false;};},[primaryStore.data.scope,primaryStore.data.cash,primaryStore.data.revision,manager,tab]);
 useEffect(()=>{onDirty(dirty);return()=>onDirty(false);},[dirty,onDirty]);
 useEffect(()=>{setDirty(false);setNotice('');},[tab,data.cash]);
 async function act(fn:(d:Data94)=>void){try{const ok=await mutate(fn);if(ok){setDirty(false);setNotice('savedLocal');}return ok;}catch{setNotice('invalid');return false;}}
 return <section className="brand-panel till94">{!['bag','journal','returns'].includes(tab)&&<header><h2>{w[tab]}</h2><span className="till94-badge">{w.localWorkspace}</span></header>}{manager&&workspaces.some(row=>row.scope!==primaryStore.data.scope)&&<label>{w.otherOperators}<select aria-label={w.otherOperators} disabled={store.busy||dirty} value={selected} onChange={e=>{setSelectedWorkspace(e.target.value);setNotice('');}}><option value="">{w.currentUser}</option>{workspaces.filter(row=>row.scope!==primaryStore.data.scope).map((row,index)=><option value={row.scope} key={row.scope}>{w.otherUser} {index+1} · {date(row.updated)} · {row.requests} {w.requests} · {row.bags} {w.bag}</option>)}</select></label>}{!['bag','journal','returns','movements','approvals'].includes(tab)&&<p className="till94-note">{w.pendingExplanation}</p>}{store.error&&<p role="alert">{w[store.error]||w.invalid} <button disabled={store.busy} onClick={store.refresh}>{w.reload}</button></p>}{notice&&<p role="status">{w[notice]||w.invalid}</p>}{!data.cash&&<p role="alert">{w.needCash}</p>}
 {tab==='suspended'&&<Suspended104 key={JSON.stringify([data.scope,data.cash])} rows={data.suspended} products={products} locked={formLocked} cartBusy={gross>0} onResume={async id=>{let restored:CartItem[]=[];let reference='';const ok=await act(d=>{const row=d.suspended.find(r=>r.id===id);if(!row)throw Error('invalid');reference=row.reference;restored=resume94(d,actor,id,products);});if(!ok)throw Error('invalid');onResume(restored,reference);}}/>}
 {['movements','approvals'].includes(tab)&&<Operations105 key={JSON.stringify([data.scope,data.cash,tab,actor.id])} data={data} actor={actor} mode={tab==='movements'?'movements':'approvals'} locked={locked} canCreate={!selected} reference={reference} gross={gross} mutate={mutate} onDirty={setDirty}/>}
 {tab==='returns'&&<Returns100 key={JSON.stringify([data.scope,data.cash,token])} data={data} actor={actor} token={token} products={products} locked={locked} canCreate={!formLocked} mutate={mutate} onDirty={setDirty}/>}
 {tab==='bag'&&<Bag99 key={JSON.stringify([data.scope,data.cash])} data={data} actor={actor} locked={locked} canCreate={!formLocked} mutate={mutate} onDirty={setDirty}/>}
 {tab==='journal'&&<Ledger99 data={data} actor={actor} token={token} ready={!locked}/>}
 {tab==='connections'&&<><p>{w.connectionsHelp}</p><div className="till94-grid">{['cards','pix','fiscal','club','drawer','printer','mobileCamera','backup'].map(k=><article className="till94-equipment" key={k}><h3>{w[k]}</h3><span className="till94-badge">{w.awaitingConnection}</span><p>{w[k+'Help']}</p></article>)}</div><Link className="brand-secondary" to="/local/settings">{w.openSettings}</Link><Link className="brand-secondary" to="/local/operation?area=fiscal">{w.fiscalPreparation}</Link></>}

 </section>;
}
