import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { localClient, localErrorMessage, useLocalSession } from '../../../core/local/useLocalSession';
import { LocalAPIError, formatLocalCents } from '../../../core/local/localClient.mjs';
import type { CashSession, LocalLocation, LocalProduct, LocalSession, SaleReceipt } from '../../../core/local/localClient.mjs';
import { cartTotal, lineCents, makeSale, parseMoney } from '../../../core/local/posModel.mjs';
import { addCartProduct, replaceCartQuantity, restoreCartItem, selectCatalogProduct } from '../../../core/local/cartEditing.mjs';
import type { CartItem } from '../../../core/local/posModel.mjs';
import { createPOSOperations } from '../../../core/local/posOperations.mjs';
import type { OperationOutcome, PendingOperation } from '../../../core/local/posOperations.mjs';
import CashCount from './CashCount';
import SalesHistory from './SalesHistory';
import './localPOS.css';

async function browserLock<T>(key: string, action: () => Promise<T>): Promise<T> {
  if (!navigator.locks) throw new Error('Abra o sistema em localhost usando um navegador com proteção entre abas.');
  return navigator.locks.request(key, { ifAvailable: true }, async (lock) => {
    if (!lock) throw new Error('Outra aba está enviando uma operação. Aguarde e atualize o caixa.');
    return action();
  });
}
const failureText = (error: unknown) => error instanceof LocalAPIError ? localErrorMessage(error) :
  error instanceof Error ? error.message : 'Não foi possível concluir a operação.';
const quantityText = (value: number) => `${Math.trunc(value / 1000)},${String(value % 1000).padStart(3, '0')}`;

function CartQuantity({item,disabled,onCommit}:{item:CartItem;disabled:boolean;onCommit:(value:string)=>boolean}) {
  const [value,setValue]=useState(quantityText(item.quantity_milli));
  useEffect(()=>setValue(quantityText(item.quantity_milli)),[item.quantity_milli]);
  function commit(){if(!onCommit(value))setValue(quantityText(item.quantity_milli));}
  return <input aria-label={`Quantidade de ${item.product.name} em ${item.product.unit}`} inputMode="decimal" maxLength={32} disabled={disabled} value={value} onChange={e=>setValue(e.target.value)} onBlur={commit} onKeyDown={e=>{if(e.key==='Enter'){e.preventDefault();e.currentTarget.blur();}}}/>;
}

export default function LocalPointOfSale() {
  const token = useLocalSession((state) => state.token);
  const session = useLocalSession((state) => state.session);
  if (!token || !session) return null;
  return <POSWorkspace token={token} session={session} />;
}

function POSWorkspace({ token, session }: { token: string; session: LocalSession }) {
  const [params] = useSearchParams();
  const [historyRevision, setHistoryRevision] = useState(0);
  const [turnBusy, setTurnBusy] = useState(false);
  const turnLatch = useRef(false);
  const [closedReport, setClosedReport] = useState<{expected_cents:number;difference_cents:number;declared_cents:number}|null>(null);
  const invalidate = useLocalSession((state) => state.invalidate);
  const setup = useMemo(() => {
    try { return { operations: createPOSOperations(localClient, window.localStorage, session, browserLock), error: '' }; }
    catch (error) { return { operations: null, error: failureText(error) }; }
  }, [session]);
  const operations = setup.operations;
  const [cash, setCash] = useState<CashSession | null>(null);
  const [knownCash, setKnownCash] = useState(false);
  const [products, setProducts] = useState<LocalProduct[]>([]);
  const [locations, setLocations] = useState<LocalLocation[]>([]);
  const [locationID, setLocationID] = useState('');
  const [search, setSearch] = useState('');
  const scanRef = useRef<HTMLInputElement>(null);
  const paymentRef = useRef<HTMLDialogElement>(null);
  const receivedRef = useRef<HTMLInputElement>(null);
  const [paying,setPaying]=useState(false);
  const [help,setHelp]=useState(false);
  const [lastProduct,setLastProduct]=useState('');
  const [removed,setRemoved]=useState<CartItem|null>(null);
  const [searchIndex,setSearchIndex]=useState(0);
  useEffect(()=>{const dialog=paymentRef.current;if(!dialog)return;if(paying&&!dialog.open){dialog.showModal();receivedRef.current?.focus();}else if(!paying&&dialog.open){dialog.close();scanRef.current?.focus();}},[paying]);
  const [offset, setOffset] = useState(0);
  const [reload, setReload] = useState(0);
  const [quantity, setQuantity] = useState('1');
  const [cart, setCart] = useState<CartItem[]>([]);
  const [opening, setOpening] = useState('');
  const [received, setReceived] = useState('');
  const [declared, setDeclared] = useState('');
  const [closing, setClosing] = useState(false);
  const [pending, setPending] = useState<PendingOperation | null>(null);
  const [canDiscard, setCanDiscard] = useState(false);
  const [receipt, setReceipt] = useState<SaleReceipt | null>(null);
  const [message, setMessage] = useState('');
  const [error, setError] = useState(setup.error);
  const [busy, setBusy] = useState(false);
  const [loading, setLoading] = useState(true);
  useEffect(()=>{if(!loading&&cash&&!paying&&!closing)scanRef.current?.focus();},[loading,cash,paying,closing]);
  const active = useRef(true);
  const actionBusy = useRef(false);
  useEffect(() => { active.current = true; return () => { active.current = false; }; }, []);

  const syncPending = useCallback(() => {
    if (!operations) return;
    try { setPending(operations.pending()); setCanDiscard(operations.canDiscard()); }
    catch (failure) { setError(failureText(failure)); setKnownCash(false); }
  }, [operations]);

  useEffect(() => {
    let live = true;
    setLoading(true); setKnownCash(false);
    Promise.all([localClient.currentCash(token), localClient.locations(token), localClient.products(token, offset)])
      .then(([current, places, rows]) => {
        if (!live) return;
        setCash(current); setKnownCash(true); setLocations(places.filter((place) => place.kind === 'shelf')); setProducts(rows);
        setLocationID((id) => places.some((place) => place.id === id && place.kind === 'shelf') ? id : '');
      }).catch((failure) => {
        if (!live) return;
        setError(failureText(failure));
        if (failure instanceof LocalAPIError && failure.status === 401) invalidate();
      }).finally(() => { if (live) { setLoading(false); syncPending(); } });
    return () => { live = false; };
  }, [token, offset, reload, invalidate, syncPending]);
  useEffect(() => {
    const update = () => syncPending();
    window.addEventListener('storage', update); window.addEventListener('focus', update);
    return () => { window.removeEventListener('storage', update); window.removeEventListener('focus', update); };
  }, [syncPending]);

  useEffect(() => { if (params.get('panel') === 'cash' && cash) setClosing(true); }, [params, cash]);
  async function refreshCash(notify = false) {
    if (turnLatch.current) return;
    turnLatch.current = true; setTurnBusy(true);
    try {
      const current = await localClient.currentCash(token);
      if (active.current) { setCash(current); setKnownCash(true); if(notify){setError('');setMessage(current ? `Turno consultado: aberto desde ${new Date(current.opened_at).toLocaleString('pt-BR')}. ID ${current.session_id}.` : 'Consulta concluída: não há turno aberto para este operador neste aparelho.');} }
    } catch (failure) {
      if (active.current) { setKnownCash(false); setError(`Confira o turno antes de continuar. ${failureText(failure)}`); }
    } finally { turnLatch.current=false; if(active.current)setTurnBusy(false); }
  }
  function confirmed(outcome: OperationOutcome) {
    if (outcome.kind === 'sale') {
      setReceipt(outcome.result); setCart([]); setRemoved(null); setLastProduct(''); setPaying(false); setReceived(''); setHistoryRevision(v=>v+1);
      setMessage(outcome.result.status === 'cancelled' ? 'A operação foi localizada e consta como cancelada.' : 'Venda confirmada pela API local.');
    } else if (outcome.kind === 'open') { setOpening(''); setMessage('Abertura de caixa confirmada.'); }
    else {
      const result = outcome.result;
      const difference = result.difference_cents;
      setClosedReport({expected_cents:result.expected_cents,difference_cents:difference,declared_cents:result.expected_cents+difference});
      setClosing(false); setDeclared('');
      setMessage(`Caixa fechado. Valor esperado: ${formatLocalCents(result.expected_cents)}. Diferença declarada: ${difference < 0 ? '−' : '+'}${formatLocalCents(Math.abs(difference))}.`);
    }
  }
  async function run(action: () => Promise<OperationOutcome | void>) {
    if (actionBusy.current) return;
    actionBusy.current = true; setBusy(true); setError(''); setMessage('');
    try {
      const outcome = await action();
      if (active.current) {
        if (outcome) confirmed(outcome);
        else { setCart([]); setReceived(''); setReload((value) => value + 1); setMessage('Operação recusada descartada. Selecione novamente os produtos e confira os preços atuais.'); }
        await refreshCash();
      }
    } catch (failure) {
      if (active.current) {
        setError(failureText(failure));
        if (failure instanceof LocalAPIError && failure.status === 401) invalidate();
      }
    } finally {
      actionBusy.current = false;
      if (active.current) { syncPending(); setBusy(false); }
    }
  }
  let total = 0, calculationError = '';
  try { total = cartTotal(cart); } catch (failure) { calculationError = failureText(failure); }
  let change: number | null = null;
  try { const amount = parseMoney(received); if (amount >= total) change = amount - total; } catch { /* Keep payment button disabled. */ }
  const locked = busy || turnBusy || loading || !!pending || !operations || !knownCash || !!calculationError;
  const filtered = products.filter((product) => `${product.name} ${product.sku} ${product.barcode || ''}`.toLocaleLowerCase('pt-BR').includes(search.toLocaleLowerCase('pt-BR')));
  function addProduct(product: LocalProduct) {
    try {
      if (locked || !cash || !locationID) throw new Error('Abra seu caixa e selecione uma gôndola de venda.');
      const next=addCartProduct(cart,product,locationID,quantity);
      setCart(next); setReceipt(null); setRemoved(null); setLastProduct(product.id); setSearch(''); setQuantity('1'); setSearchIndex(0); setError('');scanRef.current?.focus();
    } catch (failure) { setError(failureText(failure)); }
  }
  function editQuantity(index:number,value:string):boolean {
    if(locked||paying)return false;
    try {setCart(replaceCartQuantity(cart,index,value));setRemoved(null);setError('');return true;}
    catch(failure){setError(failureText(failure));return false;}
  }
  function undoRemove(){if(!removed||locked||paying)return;try{setCart(restoreCartItem(cart,removed));setLastProduct(removed.product.id);setRemoved(null);setError('');scanRef.current?.focus();}catch(failure){setError(failureText(failure));}}
  function scan(){if(locked||paying||closing)return;try{addProduct(selectCatalogProduct(products,search,searchIndex));}catch(failure){setError(failureText(failure));}}

  function submitSale() {
    if (!operations || !cash || locked || change === null) return;
    try { const input = makeSale(cart, cash.session_id); void run(() => operations.start(token, 'sale', input)); }
    catch (failure) { setError(failureText(failure)); }
  }
  return <div className="brand-page brand-app-page pos-page">

    <main className="brand-app-content pos-content">
      <div className="brand-app-heading pos-heading"><div><p className="brand-eyebrow">FRENTE DE CAIXA · API LOCAL</p><h1>Uma venda de cada vez.</h1><p className="brand-subtitle">Produtos, caixa e vendas desta instalação.</p></div><div className="pos-operator"><span>Operador</span><strong title={session.identity_id}>{session.identity_id}</strong><small>{cash ? 'Turno aberto' : knownCash ? 'Turno fechado' : 'Turno não confirmado'}</small></div>
        <button className="brand-secondary" disabled={busy || loading || turnBusy} onClick={() => void refreshCash(true)}>{turnBusy ? 'Consultando turno…' : 'Consultar turno'}</button></div>
      {(error || calculationError) && <p role="alert" className="brand-message error">{error || calculationError}</p>}
      {message && <p role="status" className="brand-message">{message}</p>}
      {pending && <section className="brand-panel pos-pending" role="status"><h2>Operação pendente de conferência</h2>
        <p>{pending.kind === 'sale' ? `Venda ${pending.input.sale_id}` : `Turno ${pending.input.session_id}`}. O carrinho fica bloqueado até confirmar o resultado.</p>
        <button className="brand-small-primary" disabled={busy} onClick={() => operations && void run(() => operations.retry(token))}>Verificar / reenviar mesma operação</button>
        {canDiscard && <button className="brand-secondary" disabled={busy} onClick={() => operations && void run(() => operations.discardRejected(token))}>Editar após recusa confirmada</button>}
      </section>}
      <section className="brand-panel pos-cash"><div><h2>{loading ? 'Consultando caixa…' : cash ? 'Seu caixa está aberto' : knownCash ? 'Seu caixa está fechado' : 'Turno não confirmado'}</h2>
        {cash && <p>Turno {cash.session_id}</p>}</div>
        {!cash && knownCash && <form onSubmit={(event) => { event.preventDefault(); if (!operations || locked) return; try { const input = { session_id: crypto.randomUUID(), opening_cents: parseMoney(opening) }; void run(() => operations.start(token, 'open', input)); } catch (failure) { setError(failureText(failure)); } }}>
          <label>Fundo de abertura (R$)<input required inputMode="decimal" value={opening} disabled={locked} placeholder="0,00" onChange={(event) => setOpening(event.target.value)} /></label>
          <button className="brand-small-primary" disabled={locked}>Abrir caixa</button></form>}
        {cash && <button className="brand-secondary" disabled={locked || cart.length > 0} onClick={() => setClosing((value) => !value)}>Fechamento cego</button>}
      </section>
      {closing && cash && <section className="brand-panel pos-closing"><p className="brand-eyebrow">ENCERRAMENTO DO TURNO</p><h2>Fechamento de caixa</h2><p>1. Finalize as vendas pendentes. 2. Conte o dinheiro. 3. Confirme a declaração. A diferença é revelada depois do fechamento.</p>
        <div className="pos-payment-status"><div><strong>Dinheiro</strong><span>Contagem assistida disponível</span></div><div><strong>Crédito / débito / Pix</strong><span>Conciliação e leitura por smartphone em preparação. Nenhum comprovante será confirmado automaticamente.</span></div></div>
        <CashCount disabled={locked} onTotal={setDeclared} />
        <form onSubmit={(event) => { event.preventDefault(); if (!operations || locked || cart.length) return; try { const input = { session_id: cash.session_id, operation_id: crypto.randomUUID(), declared_cents: parseMoney(declared) }; void run(() => operations.start(token, 'close', input)); } catch (failure) { setError(failureText(failure)); } }}>
          <label>Dinheiro contado (R$)<input required inputMode="decimal" disabled={locked} value={declared} placeholder="0,00" onChange={(event) => setDeclared(event.target.value)} /></label><button className="brand-small-primary" disabled={locked || cart.length > 0}>Confirmar fechamento</button></form></section>}
      {closedReport && <section className="brand-panel pos-closing-report" role="status"><h2>Resultado do último fechamento nesta sessão</h2><div className="pos-payment-status"><div><span>Dinheiro declarado</span><strong>{formatLocalCents(closedReport.declared_cents)}</strong></div><div><span>Saldo esperado confirmado pela API</span><strong>{formatLocalCents(closedReport.expected_cents)}</strong></div><div><span>{closedReport.difference_cents===0?'Caixa conferido':closedReport.difference_cents<0?'Falta de dinheiro':'Sobra de dinheiro'}</span><strong>{formatLocalCents(Math.abs(closedReport.difference_cents))}</strong></div></div></section>}
      <div className="pos-retail-grid">
        <div className="pos-selling">
          <form className="pos-scan" onSubmit={e=>{e.preventDefault();scan();}}>
            <label className="pos-scan-search">Produto ou código de barras<input ref={scanRef} autoFocus autoComplete="off" value={search} disabled={locked||paying||closing} placeholder="Leia o código ou procure pelo nome…" onChange={e=>{setSearch(e.target.value);setSearchIndex(0);}} onKeyDown={e=>{if(e.key==='ArrowDown'||e.key==='ArrowUp'){e.preventDefault();setSearchIndex(i=>Math.min(Math.max(i+(e.key==='ArrowDown'?1:-1),0),Math.max(0,filtered.length-1)));}if(e.key==='Escape'){setSearch('');setSearchIndex(0);}}}/></label>
            <label className="pos-scan-qty">Quantidade<input inputMode="decimal" maxLength={32} value={quantity} disabled={locked||paying||closing} onChange={e=>setQuantity(e.target.value)}/></label>
            <button className="brand-small-primary" disabled={locked||!cash||!locationID||!search.trim()||closing||paying}>Adicionar ↵</button>
          </form>
          <div className="pos-stock-source"><label>Gôndola de saída<select value={locationID} disabled={locked||paying} onChange={e=>setLocationID(e.target.value)}><option value="">Selecione a gôndola</option>{locations.map(place=><option key={place.id} value={place.id}>{place.name}</option>)}</select></label><span>Leituras repetidas agrupam o mesmo produto na mesma gôndola.</span></div>
          {!loading&&locations.length===0&&<p className="brand-state">Nenhuma gôndola cadastrada. Prepare locais e estoque no catálogo.</p>}
          {search.trim()&&<section className="brand-panel pos-search-results" aria-label="Resultados da busca"><p>Busca na página {offset/50+1} · Use ↑ ↓ e Enter para adicionar</p>{filtered.length===0?<p>Produto não encontrado nesta página.</p>:filtered.map((product,i)=><button type="button" className={`pos-product ${i===searchIndex?'pos-search-active':''}`} key={product.id} disabled={locked||!cash||!locationID||closing||paying} onClick={()=>addProduct(product)}><span><strong>{product.name}</strong><small>{product.sku} · {product.unit}</small></span><b>{formatLocalCents(product.price_cents)}</b><span aria-hidden="true">＋</span></button>)}</section>}
          <section className="brand-panel pos-basket"><div className="pos-basket-heading"><div><h2>Venda em andamento</h2><p>Preços conferidos pela API ao concluir.</p></div><span className="pos-pill">{cart.length} linha(s)</span></div>
            <div className="pos-table-scroll"><table className="pos-sale-table"><thead><tr><th scope="col">Produto</th><th scope="col">Quantidade</th><th scope="col">Preço / unidade</th><th scope="col">Subtotal</th><th scope="col"><span className="pos-sr-only">Ações</span></th></tr></thead><tbody>{cart.map((item,index)=><tr className={lastProduct===item.product.id?'pos-last-row':''} key={`${item.product.id}:${item.location_id}`}><td><strong>{item.product.name}</strong><small>{item.product.sku} · {item.product.unit} · {locations.find(p=>p.id===item.location_id)?.name||item.location_id}</small></td><td><CartQuantity item={item} disabled={locked||paying} onCommit={value=>editQuantity(index,value)}/></td><td>{formatLocalCents(item.product.price_cents)}</td><td><strong>{formatLocalCents(lineCents(item.product.price_cents,item.quantity_milli))}</strong></td><td><button type="button" className="pos-remove" disabled={locked||paying} aria-label={`Remover ${item.product.name}`} onClick={()=>{setRemoved(item);setCart(items=>items.filter((_,i)=>i!==index));scanRef.current?.focus();}}>×</button></td></tr>)}</tbody></table>{cart.length===0&&<div className="pos-empty"><span aria-hidden="true">▤</span><h3>Pronto para começar</h3><p>Selecione a gôndola e leia um código ou busque um produto.</p></div>}</div>
            <div className="pos-last-item" aria-live="polite">{lastProduct&&cart.find(i=>i.product.id===lastProduct)?`Último produto: ${cart.find(i=>i.product.id===lastProduct)?.product.name}`:'Aguardando o próximo produto'}{removed&&<button className="brand-secondary" disabled={locked||paying} onClick={undoRemove}>Desfazer remoção de {removed.product.name}</button>}</div>
          </section>
          <details className="pos-catalog-browser"><summary>Consultar catálogo · página {offset/50+1}</summary><div className="pos-product-list">{products.map(product=><button type="button" className="pos-product" key={product.id} disabled={locked||!cash||!locationID||closing||paying} onClick={()=>addProduct(product)}><span><strong>{product.name}</strong><small>{product.sku} · {product.unit}</small></span><b>{formatLocalCents(product.price_cents)}</b><span aria-hidden="true">＋</span></button>)}</div></details>
          <nav className="brand-pagination" aria-label="Páginas de produtos"><button className="brand-secondary" disabled={loading||busy||paying||offset===0} onClick={()=>setOffset(v=>Math.max(0,v-50))}>Anterior</button><span>Página {offset/50+1}</span><button className="brand-secondary" disabled={loading||busy||paying||products.length<50} onClick={()=>setOffset(v=>v+50)}>Próxima</button><button className="brand-secondary" disabled={loading||busy||paying} onClick={()=>{setError('');setReload(v=>v+1);}}>Atualizar catálogo</button></nav>
        </div>
        <aside className="brand-panel pos-summary"><p className="brand-eyebrow">RESUMO DA COMPRA</p><h2>Total estimado</h2><strong className="pos-grand-total">{formatLocalCents(total)}</strong><dl className="pos-summary-lines"><div><dt>Subtotal</dt><dd>{formatLocalCents(total)}</dd></div><div><dt>Linhas de produtos</dt><dd>{cart.length}</dd></div><div><dt>Unidades (produtos unitários)</dt><dd>{String(cart.filter(i=>i.product.unit==='unit').reduce((n,i)=>n+BigInt(i.quantity_milli),0n)/1000n)}</dd></div></dl><p className="pos-note">Produtos por peso, volume ou comprimento mantêm a quantidade e a unidade de cada linha.</p>
          <div className="pos-status-stack"><div><span>Venda</span><strong>{pending?.kind==='sale'?'Pendente de conferência':cart.length?'Em edição':receipt?'Consulte o registro abaixo':'Não iniciada'}</strong></div><div><span>Pagamento atual</span><strong>{pending?.kind==='sale'?'Resultado não confirmado':busy&&paying?'Conferindo na API':'Não registrado'}</strong></div><div><span>Documento fiscal</span><strong>Não emitido</strong></div></div>
          <button type="button" className="brand-primary pos-checkout" disabled={locked||!cash||!cart.length||total<=0||closing} onClick={()=>{setReceived('');setPaying(true);}}>Ir para pagamento →</button>
          <p className="pos-note">Dinheiro disponível. Pix, cartão, desconto, CPF fiscal e pagamento dividido aguardam integração.</p>
        </aside>
      </div>
      <footer className="pos-statusbar"><span>{loading?'Consultando API…':knownCash?'Turno consultado na API local':'API / turno não confirmado'}</span><span>Enter: adicionar · ↑ ↓: selecionar busca · Tab: navegar</span><button className="brand-secondary" aria-expanded={help} onClick={()=>setHelp(v=>!v)}>Ajuda</button></footer>
      {help&&<section className="brand-panel pos-help"><h2>Operação do caixa</h2><p>Abra seu turno, selecione a gôndola e registre por SKU, código de barras ou nome. A busca consulta a página carregada; navegue pelo catálogo para outros produtos. Altere quantidades diretamente na linha; Enter confirma a correção. Remova e desfaça a última remoção quando necessário.</p><p>A venda só é registrada após confirmação da API. Se houver perda de resposta, use a operação pendente; não abra outra venda para substituir a anterior. O carrinho ainda em edição não é salvo ao recarregar. Suspensão e retomada de rascunhos não estão disponíveis nesta entrega.</p></section>}
      <dialog ref={paymentRef} className="pos-payment-dialog" aria-labelledby="pos-payment-title" onCancel={e=>{if(busy){e.preventDefault();return;}setPaying(false);}} onClose={()=>{setPaying(false);scanRef.current?.focus();}}><div className="pos-dialog-heading"><div><p className="brand-eyebrow">PAGAMENTO EM DINHEIRO</p><h2 id="pos-payment-title">Conferir e concluir</h2></div><button className="brand-secondary" aria-label="Voltar à venda" disabled={busy} onClick={()=>setPaying(false)}>×</button></div><div className="pos-total"><span>Total estimado</span><strong>{formatLocalCents(total)}</strong></div><p>Confira os itens. O servidor valida preços, estoque e turno antes de registrar.</p><form onSubmit={e=>{e.preventDefault();submitSale();}}><label>Dinheiro recebido (R$)<input ref={receivedRef} autoComplete="off" inputMode="decimal" maxLength={32} disabled={locked||!cart.length} placeholder="0,00" value={received} onChange={e=>setReceived(e.target.value)}/></label><p className="pos-change">Troco: {change===null||!cart.length?'—':formatLocalCents(change)}</p>{error&&<p role="alert">{error}</p>}{pending&&<p role="status">Resultado não confirmado. Volte à tela e confira a operação pendente.</p>}<button className="brand-primary" disabled={locked||!cash||!cart.length||total<=0||change===null||closing}>{busy?'Conferindo operação…':'Confirmar venda em dinheiro'}</button></form><p className="pos-note">Não emite documento fiscal. Não há cobrança via Pix ou cartão.</p></dialog>
      {receipt && <section className="brand-panel pos-receipt"><p className="brand-eyebrow">REGISTRO OPERACIONAL · NÃO FISCAL</p><h2>{receipt.status === 'cancelled' ? 'Venda cancelada' : 'Venda confirmada'}</h2><p>ID {receipt.sale_id}</p><p>Total registrado: <strong>{formatLocalCents(receipt.total_cents)}</strong></p><p>{receipt.items.length} linha(s) · {receipt.status === 'cancelled' ? 'Registro original de venda cancelada.' : 'Pagamento em dinheiro confirmado pela API.'}</p></section>}
      <SalesHistory token={token} revision={historyRevision} onRead={setReceipt} />
    </main>
  </div>;
}
