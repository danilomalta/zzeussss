import CatalogManagement,{DraftLeaveDialog} from './CatalogManagement';
import {createCatalogCreator} from '../../../core/catalogManagement/creation.mjs';
import {catalogCents} from '../../../core/catalogManagement/model.mjs';
import {catalogWords} from '../../../core/catalogManagement/words.mjs';
import {lazy,Suspense,useEffect,useMemo,useRef,useState} from 'react';
import {useTranslation} from 'react-i18next';
import {stockWords} from '../../../core/stockWorkspace/words';
import { createCatalogSearch } from '../../../core/local/catalogSearch.mjs';
import type { CatalogSearchProduct } from '../../../core/local/catalogSearch.mjs';
import './catalogManagement.css';
import { useLocation } from 'react-router-dom';
import { useLocalAccess } from '../../../core/local/LocalAccess';
import { LocalAPIError, formatLocalCents } from '../../../core/local/localClient.mjs';
import type { LocalProduct, LocalLocation, StockEntry } from '../../../core/local/localClient.mjs';
import { parseQuantity } from '../../../core/local/posModel.mjs';
import { localClient, localErrorMessage, useLocalSession } from '../../../core/local/useLocalSession';

const searchCatalog = createCatalogSearch();
const units = [['unit','Unidade'],['kg','Quilograma'],['g','Grama'],['liter','Litro'],['ml','Mililitro'],['meter','Metro']];

const LocalStock=lazy(()=>import('./LocalStock'));
export default function LocalCatalog(){const location=useLocation(),{i18n}=useTranslation(),token=useLocalSession(s=>s.token),session=useLocalSession(s=>s.session);return location.pathname==='/local/stock'?<Suspense fallback={<p role="status">{stockWords(i18n.resolvedLanguage||i18n.language).loading}</p>}><LocalStock/></Suspense>:<CatalogContent key={JSON.stringify([token,session?.tenant_id,session?.store_id,session?.identity_id,session?.device_id])}/>;}
function CatalogContent() {
 const {i18n}=useTranslation(),locale=i18n.resolvedLanguage||i18n.language,managementWords=catalogWords(locale);
 const [managedProduct,setManagedProduct]=useState<{id:string;name:string}|null>(null),[managementDirty,setManagementDirty]=useState(false),[managementBusy,setManagementBusy]=useState(false);
 const [leaveProduct,setLeaveProduct]=useState<{product:{id:string;name:string}|null}|null>(null);
 const chooseProduct=(product:{id:string;name:string}|null)=>{if(managementBusy||product?.id===managedProduct?.id)return;if(managementDirty){setLeaveProduct({product});return;}setManagedProduct(product);};
 const token = useLocalSession(s => s.token);
 const session = useLocalSession(s => s.session);
 const invalidate = useLocalSession(s => s.invalidate);
 const location = useLocation();
 const tab = location.pathname === '/local/stock' ? 'stock' : 'products';
 const {capabilities}=useLocalAccess();
 const currentCapabilities=useRef(capabilities);currentCapabilities.current=capabilities;
 const stillCurrent=()=>{const state=useLocalSession.getState();return state.token===token&&state.session===session&&currentCapabilities.current===capabilities;};
 const creator=useMemo(()=>{try{return token&&session&&capabilities?createCatalogCreator({token,session,capabilities,isCurrent:stillCurrent,send:localClient.createProduct}):null;}catch{return null;}},[token,session,capabilities]);
 const canWrite = capabilities&&['view_catalog','manage_stock'].every(p=>capabilities.permissions.includes(p))&&capabilities.license.state==='active'&&['core','inventory'].every(m=>capabilities.license.modules.includes(m));
 const [items,setItems] = useState<LocalProduct[]>([]);
 const [results,setResults] = useState<CatalogSearchProduct[]>([]);
 const [total,setTotal] = useState(0);
 const [costVisible,setCostVisible] = useState(false);
 const [query,setQuery] = useState('');
 const [filterUnit,setFilterUnit] = useState('');
 const [filterPending,setFilterPending] = useState('');
 const [search,setSearch] = useState({q:'',unit:'',pending:'',offset:0});
 const [columns,setColumns] = useState({barcode:true,unit:true,cost:false,minimum:true});
 const [places,setPlaces] = useState<LocalLocation[]>([]);
 const [offset,setOffset] = useState(0);
 const [reload,setReload] = useState(0);
 const [loading,setLoading] = useState(true);
 const [busy,setBusy] = useState(false);
 const latch = useRef(false);
 const [error,setError] = useState('');
 const [message,setMessage] = useState('');
 const [sku,setSKU] = useState(''); const [name,setName] = useState(''); const [barcode,setBarcode] = useState('');
 const [unit,setUnit] = useState('unit'); const [price,setPrice] = useState(''); const [cost,setCost] = useState('0');
 const [placeName,setPlaceName] = useState(''); const [kind,setKind] = useState('shelf');
 const [productID,setProductID] = useState(''); const [placeID,setPlaceID] = useState(''); const [quantity,setQuantity] = useState(''); const [reason,setReason] = useState('Recebimento de mercadoria');
 const [pending,setPending] = useState<StockEntry|null>(null);
 const key = `titan-stock-entry:${session?.tenant_id}:${session?.store_id}:${session?.device_id}:${session?.identity_id}`;
 useEffect(() => {
  try { const raw=localStorage.getItem(key); if(raw) { const p=JSON.parse(raw); if(!p || p.kind!=='entry' || typeof p.operation_id!=='string' || typeof p.product_id!=='string' || typeof p.to_location_id!=='string' || !Number.isSafeInteger(p.quantity_milli) || p.quantity_milli<=0 || typeof p.reason!=='string') throw new Error('Entrada pendente inválida. Não envie uma nova entrada antes da conferência técnica.'); setPending(p); } }
  catch { setError('Não foi possível ler a entrada pendente. As entradas ficam bloqueadas; confira o armazenamento do navegador.'); setPending({} as StockEntry); }
 },[key]);
 useEffect(() => {
  if(!token || tab!=='stock') return;
  let live=true; setLoading(true);setItems([]);
  Promise.all([localClient.products(token,offset),localClient.locations(token)]).then(([rows,locations]) => { if(live){setItems(rows);setPlaces(locations);} }).catch(e => { if(live){setError(localErrorMessage(e));if(e instanceof LocalAPIError && e.status===401) invalidate();} }).finally(() => {if(live)setLoading(false);});
  return () => {live=false;};
 },[token,offset,reload,invalidate,tab]);
 useEffect(() => {
  if(!token || tab!=='products') return;
  const controller=new AbortController();setLoading(true);setResults([]);setTotal(0);setError('');
  searchCatalog(token,search,controller.signal).then(value=>{if(!controller.signal.aborted){setResults(value.items);setTotal(value.total);setCostVisible(value.cost_visible);}}).catch(e=>{if(!controller.signal.aborted){setError(e instanceof LocalAPIError&&[401,403].includes(e.status)?'denied':'unavailable');if(e instanceof LocalAPIError && e.status===401)invalidate();}}).finally(()=>{if(!controller.signal.aborted)setLoading(false);});
  return ()=>controller.abort();
 },[token,search,reload,invalidate,tab]);
 async function save(action:()=>Promise<void>) {
  if(latch.current || !token || !canWrite) return;
  latch.current=true; setBusy(true);setError('');setMessage('');
  try { await action();if(stillCurrent())setReload(v=>v+1); }
  catch(e){if(!stillCurrent())return;setError(tab==='products'?(e instanceof Error&&e.message==='catalogInvalid'?'invalid':e instanceof LocalAPIError&&[401,403].includes(e.status)?'denied':'unavailable'):(e instanceof Error ? e.message : localErrorMessage(e)));if(e instanceof LocalAPIError && e.status===401)invalidate();}
  finally {latch.current=false;setBusy(false);}
 }
 async function sendEntry(input: StockEntry) {
  if(!token || !navigator.locks) throw new Error('Entrada de estoque exige navegador com proteção entre abas em localhost.');
  await navigator.locks.request(key,{ifAvailable:true},async lock=>{
   if(!lock) throw new Error('Outra aba está registrando estoque.');
   const stored=localStorage.getItem(key);
   if(stored && stored!==JSON.stringify(input)) throw new Error('Existe outra entrada pendente nesta instalação. Atualize a página para conferir.');
   localStorage.setItem(key,JSON.stringify(input));setPending(input);
   await localClient.stockEntry(token,input);
   localStorage.removeItem(key);setPending(null);setQuantity('');setMessage('Entrada confirmada no estoque. O produto já pode ser vendido neste local.');
  });
 }
 return <main className="brand-app-content">
  <div className="brand-app-heading"><div><p className="brand-eyebrow">{managementWords.local}</p><h1>{tab==='stock'?'Estoque e locais':managementWords.catalog}</h1><p className="brand-subtitle">{managementWords.subtitle}</p></div><button className="brand-secondary" disabled={busy || loading} onClick={()=>{setError('');setReload(v=>v+1);}}>{managementWords.refresh}</button></div>

  {error && <p role="alert" className="brand-message error">{managementWords[error]||error}</p>}{message && <p role="status" className="brand-message">{message}</p>}
  {tab==='products' ? <>
   {canWrite && <details className="brand-panel catalog-create"><summary>{managementWords.create}</summary><p className="brand-state">{managementWords.createHelp}</p>
    <form className="catalog-form" onSubmit={e=>{e.preventDefault();void save(async()=>{if(!creator)return;await creator.create({sku:sku.trim(),name:name.trim(),barcode:barcode.trim(),unit,price_cents:catalogCents(price),cost_cents:catalogCents(cost)});if(!stillCurrent())return;setSKU('');setName('');setBarcode('');setPrice('');setCost('0');setUnit('unit');setMessage(managementWords.created);setSearch(v=>({...v,offset:0}));});}}>
     <label>{managementWords.name}<input required maxLength={255} disabled={busy || !canWrite} value={name} onChange={e=>setName(e.target.value)}/></label><label>{managementWords.sku}<input required maxLength={100} disabled={busy || !canWrite} value={sku} onChange={e=>setSKU(e.target.value)}/></label>
     <label>{managementWords.barcode}<input disabled={busy || !canWrite} value={barcode} onChange={e=>setBarcode(e.target.value)}/></label><label>{managementWords.unit}<select disabled={busy || !canWrite} value={unit} onChange={e=>setUnit(e.target.value)}>{[['unit','Unidade'],['kg','Quilograma'],['g','Grama'],['liter','Litro'],['ml','Mililitro'],['meter','Metro']].map(([v])=><option key={v} value={v}>{managementWords['unit_'+v]}</option>)}</select></label>
     <label>{managementWords.price}<input required inputMode="decimal" disabled={busy || !canWrite} value={price} onChange={e=>setPrice(e.target.value)}/></label><label>{managementWords.cost}<input required inputMode="decimal" disabled={busy || !canWrite} value={cost} onChange={e=>setCost(e.target.value)}/></label>
     <button className="brand-small-primary catalog-wide" disabled={busy || !canWrite}>{busy?managementWords.creating:managementWords.create}</button>
    </form><p className="pos-note">{managementWords.creationHelp}</p>
   </details>}
   {!canWrite && <p className="brand-message">{managementWords.readOnly}</p>}
   <section className="brand-panel catalog-search-panel">
    <form className="catalog-search-form" onSubmit={e=>{e.preventDefault();setSearch({q:query.trim(),unit:filterUnit,pending:filterPending,offset:0});}}>
     <label className="catalog-search-query">{managementWords.searchLabel}<input type="search" value={query} maxLength={240} placeholder={managementWords.searchPlaceholder} onChange={e=>setQuery(e.target.value)}/></label>
     <label>{managementWords.unit}<select value={filterUnit} onChange={e=>setFilterUnit(e.target.value)}><option value="">{managementWords.allUnits}</option>{units.map(([v])=><option key={v} value={v}>{managementWords['unit_'+v]}</option>)}</select></label>
     <label>{managementWords.check}<select value={filterPending} onChange={e=>setFilterPending(e.target.value)}><option value="">{managementWords.allProducts}</option><option value="barcode">{managementWords.noBarcode}</option><option value="minimum">{managementWords.noPolicy}</option>{costVisible && <option value="cost">{managementWords.zeroCost}</option>}</select></label>
     <button className="brand-small-primary">{managementWords.search}</button>
    </form>
    <p className="brand-state">{managementWords.searchHelp}</p>
   </section>
   <section className="brand-panel catalog-results">
    <div className="catalog-table-heading"><div><h2>{managementWords.registered}</h2><p role="status">{loading?managementWords.loading:`${total} ${managementWords.results}${search.q?' · “'+search.q+'”':''}`}</p></div>
     <details className="catalog-columns"><summary>{managementWords.columns}</summary><fieldset><legend>{managementWords.tableInfo}</legend>{([['barcode',managementWords.barcodeShort],['unit',managementWords.unit],['cost',managementWords.costShort],['minimum',managementWords.restock]] as const).filter(([key])=>key!=='cost'||costVisible).map(([key,label])=><label key={key}><input type="checkbox" checked={columns[key]} onChange={e=>setColumns(v=>({...v,[key]:e.target.checked}))}/>{label}</label>)}</fieldset></details>
    </div>
    {!loading && <div className="brand-table-scroll"><table className="brand-table catalog-data-table"><thead><tr><th scope="col">SKU</th><th scope="col">{managementWords.product}</th>{columns.barcode && <th scope="col">{managementWords.barcodeShort}</th>}{columns.unit && <th scope="col">{managementWords.unit}</th>}<th scope="col" className="catalog-money">{managementWords.priceShort}</th>{columns.cost && costVisible && <th scope="col" className="catalog-money">{managementWords.costShort}</th>}{columns.minimum && <th scope="col">{managementWords.storePolicy}</th>}<th scope="col">{managementWords.title}</th></tr></thead><tbody>{results.map(p=><tr key={p.id}><td>{p.sku}</td><td><strong>{p.name}</strong>{p.approximate && <span className="catalog-approximate">{managementWords.approximate}</span>}</td>{columns.barcode && <td>{p.barcode||managementWords.notProvided}</td>}{columns.unit && <td>{managementWords['unit_'+p.unit]||p.unit}</td>}<td className="catalog-money">{formatLocalCents(p.price_cents)}</td>{columns.cost && costVisible && <td className="catalog-money">{formatLocalCents(p.cost_cents!)}</td>}{columns.minimum && <td>{p.minimum_configured?managementWords.policyYes:managementWords.policyNo}</td>}<td><button className="brand-secondary" disabled={managementBusy||busy} aria-label={`${managementWords.details}: ${p.name}`} onClick={()=>chooseProduct(p)}>{managementWords.details}</button></td></tr>)}</tbody></table>{!results.length && !error && <p className="catalog-empty">{total===0?managementWords.emptySearch:managementWords.emptyPage}</p>}</div>}
    <nav className="brand-pagination" aria-label={managementWords.pages}><button className="brand-secondary" disabled={loading || busy || search.offset===0} onClick={()=>setSearch(v=>({...v,offset:Math.max(0,v.offset-50)}))}>{managementWords.previous}</button><span>{managementWords.page} {search.offset/50+1} · {managementWords.pageSize}</span><button className="brand-secondary" disabled={loading || busy || search.offset+50>=total} onClick={()=>setSearch(v=>({...v,offset:v.offset+50}))}>{managementWords.next}</button></nav>
   </section>
   <DraftLeaveDialog open={leaveProduct!==null} locale={locale} onCancel={()=>setLeaveProduct(null)} onLeave={()=>{if(leaveProduct)setManagedProduct(leaveProduct.product);setLeaveProduct(null);}}/>
   <CatalogManagement externalDirty={!!(sku||name||barcode||price)||cost!=='0'} selected={managedProduct} locale={locale} onClose={()=>chooseProduct(null)} onChanged={()=>setReload(value=>value+1)} onDirty={setManagementDirty} onBusy={setManagementBusy}/>
  </> : <>
   <section className="brand-panel"><h2>Cadastrar local de estoque</h2><form className="catalog-form" onSubmit={e=>{e.preventDefault();void save(async()=>{await localClient.createLocation(token!,{kind,name:placeName.trim()});setPlaceName('');setMessage('Local cadastrado.');});}}><label>Nome do local<input required maxLength={255} disabled={busy || !canWrite} value={placeName} onChange={e=>setPlaceName(e.target.value)}/></label><label>Tipo<select value={kind} disabled={busy || !canWrite} onChange={e=>setKind(e.target.value)}><option value="shelf">Gôndola / saída de venda</option><option value="backroom">Depósito</option><option value="receiving">Recebimento</option><option value="production">Produção</option></select></label><button className="brand-small-primary catalog-wide" disabled={busy || !canWrite}>Cadastrar local</button></form><p className="brand-state">{places.length} local(is) cadastrado(s). Se houver perda de resposta, confira a lista antes de repetir.</p></section>
   <section className="brand-panel" style={{marginTop:20}}><h2>Entrada de estoque</h2>{pending ? <div className="brand-message"><p>Entrada pendente de confirmação. Reenvie a mesma operação para evitar duplicar estoque.</p><button className="brand-secondary" disabled={busy || !pending.operation_id} onClick={()=>void save(()=>sendEntry(pending))}>Conferir / reenviar entrada original</button></div>:<form className="catalog-form" onSubmit={e=>{e.preventDefault();void save(async()=>{const p=items.find(p=>p.id===productID);if(!p)throw new Error('Selecione o produto desta página.');await sendEntry({operation_id:crypto.randomUUID(),kind:'entry',product_id:productID,to_location_id:placeID,quantity_milli:parseQuantity(quantity,p.unit),reason:reason.trim()});});}}>
    <label>Produto desta página<select required disabled={busy || !canWrite} value={productID} onChange={e=>setProductID(e.target.value)}><option value="">Selecione</option>{items.map(p=><option key={p.id} value={p.id}>{p.name} · {p.unit}</option>)}</select></label><label>Local de destino<select required disabled={busy || !canWrite} value={placeID} onChange={e=>setPlaceID(e.target.value)}><option value="">Selecione</option>{places.map(p=><option key={p.id} value={p.id}>{p.name}</option>)}</select></label><label>Quantidade na unidade do produto<input required disabled={busy || !canWrite} inputMode="decimal" value={quantity} onChange={e=>setQuantity(e.target.value)}/></label><label>Motivo<input required disabled={busy || !canWrite} value={reason} onChange={e=>setReason(e.target.value)}/></label><button className="brand-small-primary catalog-wide" disabled={busy || !canWrite}>Registrar entrada</button></form>}</section>
  </>}
  {tab==='stock' && <nav className="brand-pagination" aria-label="Páginas de produtos"><button className="brand-secondary" disabled={loading || busy || offset===0} onClick={()=>setOffset(v=>Math.max(0,v-50))}>Anterior</button><span>Página {offset/50+1}</span><button className="brand-secondary" disabled={loading || busy || items.length<50} onClick={()=>setOffset(v=>v+50)}>Próxima</button></nav>}
 </main>;
}
