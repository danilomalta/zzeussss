import { useEffect, useRef, useState } from 'react';
import { useLocation } from 'react-router-dom';
import { useLocalAccess } from '../../../core/local/LocalAccess';
import { LocalAPIError, formatLocalCents } from '../../../core/local/localClient.mjs';
import type { LocalProduct, LocalLocation, StockEntry } from '../../../core/local/localClient.mjs';
import { parseMoney, parseQuantity } from '../../../core/local/posModel.mjs';
import { localClient, localErrorMessage, useLocalSession } from '../../../core/local/useLocalSession';

export default function LocalCatalog() {
 const token = useLocalSession(s => s.token);
 const session = useLocalSession(s => s.session);
 const invalidate = useLocalSession(s => s.invalidate);
 const location = useLocation();
 const tab = location.pathname === '/local/stock' ? 'stock' : 'products';
 const {capabilities}=useLocalAccess();
 const canWrite = capabilities?.permissions.includes('manage_stock') && capabilities.license.state==='active';
 const [items,setItems] = useState<LocalProduct[]>([]);
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
  if(!token) return;
  let live=true; setLoading(true);
  Promise.all([localClient.products(token,offset),localClient.locations(token)]).then(([rows,locations]) => { if(live){setItems(rows);setPlaces(locations);} }).catch(e => { if(live){setError(localErrorMessage(e));if(e instanceof LocalAPIError && e.status===401) invalidate();} }).finally(() => {if(live)setLoading(false);});
  return () => {live=false;};
 },[token,offset,reload,invalidate]);
 async function save(action:()=>Promise<void>) {
  if(latch.current || !token || !canWrite) return;
  latch.current=true; setBusy(true);setError('');setMessage('');
  try { await action();setReload(v=>v+1); }
  catch(e){setError(e instanceof Error ? e.message : localErrorMessage(e));if(e instanceof LocalAPIError && e.status===401)invalidate();}
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
  <div className="brand-app-heading"><div><p className="brand-eyebrow">GESTÃO LOCAL</p><h1>{tab==='stock'?'Estoque e locais':'Catálogo de produtos'}</h1><p className="brand-subtitle">Cadastros da empresa · As permissões e o plano são conferidos pelo servidor.</p></div><button className="brand-secondary" disabled={busy || loading} onClick={()=>{setError('');setReload(v=>v+1);}}>Atualizar</button></div>

  {error && <p role="alert" className="brand-message error">{error}</p>}{message && <p role="status" className="brand-message">{message}</p>}
  {tab==='products' ? <>
   {canWrite && <section className="brand-panel"><h2>Cadastrar produto</h2><p className="brand-state">Cadastro real na API. Para vender, registre também a quantidade em uma gôndola na aba de estoque.</p>
    <form className="catalog-form" onSubmit={e=>{e.preventDefault();void save(async()=>{await localClient.createProduct(token!,{sku:sku.trim(),name:name.trim(),barcode:barcode.trim(),unit,price_cents:parseMoney(price),cost_cents:parseMoney(cost)});setSKU('');setName('');setBarcode('');setPrice('');setMessage('Produto cadastrado. Registre o estoque antes de vender.');setOffset(0);});}}>
     <label>Nome<input required maxLength={255} disabled={busy || !canWrite} value={name} onChange={e=>setName(e.target.value)}/></label><label>SKU / referência<input required maxLength={100} disabled={busy || !canWrite} value={sku} onChange={e=>setSKU(e.target.value)}/></label>
     <label>Código de barras (opcional)<input disabled={busy || !canWrite} value={barcode} onChange={e=>setBarcode(e.target.value)}/></label><label>Unidade<select disabled={busy || !canWrite} value={unit} onChange={e=>setUnit(e.target.value)}>{[['unit','Unidade'],['kg','Quilograma'],['g','Grama'],['liter','Litro'],['ml','Mililitro'],['meter','Metro']].map(([v,n])=><option key={v} value={v}>{n}</option>)}</select></label>
     <label>Preço de venda (R$ por unidade selecionada)<input required inputMode="decimal" disabled={busy || !canWrite} value={price} onChange={e=>setPrice(e.target.value)}/></label><label>Custo (R$)<input required inputMode="decimal" disabled={busy || !canWrite} value={cost} onChange={e=>setCost(e.target.value)}/></label>
     <button className="brand-small-primary catalog-wide" disabled={busy || !canWrite}>{busy?'Registrando…':'Cadastrar produto'}</button>
    </form><p className="pos-note">Se a resposta se perder, consulte o SKU no catálogo antes de repetir o cadastro. Cadastro da empresa e planos continuam em preparação.</p>
   </section>}
   {!canWrite && <p className="brand-message">Consulta autorizada. Cadastro e alterações exigem permissão de estoque e licença vigente.</p>}
   <section className="brand-panel" style={{marginTop:20}}><h2>Produtos cadastrados</h2>{loading?<p role="status">Consultando…</p>:<div className="brand-table-scroll"><table className="brand-table"><thead><tr><th>SKU</th><th>Produto</th><th>Unidade</th><th>Preço</th></tr></thead><tbody>{items.map(p=><tr key={p.id}><td>{p.sku}</td><td>{p.name}</td><td>{p.unit}</td><td>{formatLocalCents(p.price_cents)}</td></tr>)}</tbody></table>{!items.length && <p>Nenhum produto nesta página.</p>}</div>}</section>
  </> : <>
   <section className="brand-panel"><h2>Cadastrar local de estoque</h2><form className="catalog-form" onSubmit={e=>{e.preventDefault();void save(async()=>{await localClient.createLocation(token!,{kind,name:placeName.trim()});setPlaceName('');setMessage('Local cadastrado.');});}}><label>Nome do local<input required maxLength={255} disabled={busy || !canWrite} value={placeName} onChange={e=>setPlaceName(e.target.value)}/></label><label>Tipo<select value={kind} disabled={busy || !canWrite} onChange={e=>setKind(e.target.value)}><option value="shelf">Gôndola / saída de venda</option><option value="backroom">Depósito</option><option value="receiving">Recebimento</option><option value="production">Produção</option></select></label><button className="brand-small-primary catalog-wide" disabled={busy || !canWrite}>Cadastrar local</button></form><p className="brand-state">{places.length} local(is) cadastrado(s). Se houver perda de resposta, confira a lista antes de repetir.</p></section>
   <section className="brand-panel" style={{marginTop:20}}><h2>Entrada de estoque</h2>{pending ? <div className="brand-message"><p>Entrada pendente de confirmação. Reenvie a mesma operação para evitar duplicar estoque.</p><button className="brand-secondary" disabled={busy || !pending.operation_id} onClick={()=>void save(()=>sendEntry(pending))}>Conferir / reenviar entrada original</button></div>:<form className="catalog-form" onSubmit={e=>{e.preventDefault();void save(async()=>{const p=items.find(p=>p.id===productID);if(!p)throw new Error('Selecione o produto desta página.');await sendEntry({operation_id:crypto.randomUUID(),kind:'entry',product_id:productID,to_location_id:placeID,quantity_milli:parseQuantity(quantity,p.unit),reason:reason.trim()});});}}>
    <label>Produto desta página<select required disabled={busy || !canWrite} value={productID} onChange={e=>setProductID(e.target.value)}><option value="">Selecione</option>{items.map(p=><option key={p.id} value={p.id}>{p.name} · {p.unit}</option>)}</select></label><label>Local de destino<select required disabled={busy || !canWrite} value={placeID} onChange={e=>setPlaceID(e.target.value)}><option value="">Selecione</option>{places.map(p=><option key={p.id} value={p.id}>{p.name}</option>)}</select></label><label>Quantidade na unidade do produto<input required disabled={busy || !canWrite} inputMode="decimal" value={quantity} onChange={e=>setQuantity(e.target.value)}/></label><label>Motivo<input required disabled={busy || !canWrite} value={reason} onChange={e=>setReason(e.target.value)}/></label><button className="brand-small-primary catalog-wide" disabled={busy || !canWrite}>Registrar entrada</button></form>}</section>
  </>}
  <nav className="brand-pagination" aria-label="Páginas de produtos"><button className="brand-secondary" disabled={loading || busy || offset===0} onClick={()=>setOffset(v=>Math.max(0,v-50))}>Anterior</button><span>Página {offset/50+1}</span><button className="brand-secondary" disabled={loading || busy || items.length<50} onClick={()=>setOffset(v=>v+50)}>Próxima</button></nav>
 </main>;
}
