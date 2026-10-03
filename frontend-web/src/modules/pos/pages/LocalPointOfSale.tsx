import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { localClient, localErrorMessage, useLocalSession } from '../../../core/local/useLocalSession';
import { LocalAPIError, formatLocalCents } from '../../../core/local/localClient.mjs';
import type { CashSession, LocalLocation, LocalProduct, LocalSession, SaleReceipt } from '../../../core/local/localClient.mjs';
import { cartTotal, lineCents, makeSale, parseMoney, parseQuantity } from '../../../core/local/posModel.mjs';
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
      setReceipt(outcome.result); setCart([]); setReceived(''); setHistoryRevision(v=>v+1);
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
      const quantityMilli = parseQuantity(quantity, product.unit);
      const old = cart.find((item) => item.product.id === product.id && item.location_id === locationID);
      const combined = (old?.quantity_milli || 0) + quantityMilli;
      if (!Number.isSafeInteger(combined)) throw new Error('Quantidade fora do intervalo suportado.');
      const next = old ? cart.map((item) => item === old ? { product, location_id: locationID, quantity_milli: combined } : item) : [...cart, { product, location_id: locationID, quantity_milli: quantityMilli }];
      if (next.length > 500) throw new Error('Limite de 500 linhas por venda.');
      cartTotal(next); setCart(next); setReceipt(null); setError('');
    } catch (failure) { setError(failureText(failure)); }
  }
  function submitSale() {
    if (!operations || !cash || locked || change === null) return;
    try { const input = makeSale(cart, cash.session_id); void run(() => operations.start(token, 'sale', input)); }
    catch (failure) { setError(failureText(failure)); }
  }
  return <div className="brand-page brand-app-page pos-page">

    <main className="brand-app-content pos-content">
      <div className="brand-app-heading"><div><p className="brand-eyebrow">INSTALAÇÃO LOCAL</p><h1>Frente de caixa</h1><p className="brand-subtitle">Operador {session.identity_id} · Venda em dinheiro</p></div>
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
      <div className="pos-grid">
        <section className="brand-panel pos-products"><div className="brand-panel-top"><h2>Produtos</h2><button className="brand-secondary" disabled={loading || busy} onClick={() => { setError(''); setReload((value) => value + 1); }}>Atualizar catálogo</button></div>
          <label>Gôndola de saída<select value={locationID} disabled={locked} onChange={(event) => setLocationID(event.target.value)}><option value="">Selecione a gôndola</option>{locations.map((place) => <option key={place.id} value={place.id}>{place.name}</option>)}</select></label>
          {!loading && locations.length === 0 && <p className="brand-state">Nenhuma gôndola cadastrada. O administrador precisa preparar catálogo, locais e estoque.</p>}
          <div className="pos-filters"><label>Buscar nesta página<input type="search" value={search} placeholder="Nome, SKU ou código de barras" onChange={(event) => setSearch(event.target.value)} /></label>
            <label>Quantidade<input inputMode="decimal" value={quantity} disabled={locked} onChange={(event) => setQuantity(event.target.value)} /></label></div>
          <div className="pos-product-list">{loading ? <p role="status">Consultando produtos…</p> : filtered.length === 0 ? <p className="brand-state">Nenhum produto nesta página.</p> : filtered.map((product) => <button type="button" className="pos-product" key={product.id} disabled={locked || !cash || !locationID || closing} onClick={() => addProduct(product)}><span><strong>{product.name}</strong><small>{product.sku} · {product.unit}</small></span><b>{formatLocalCents(product.price_cents)}</b><span aria-hidden="true">＋</span></button>)}</div>
          <nav className="brand-pagination" aria-label="Páginas de produtos"><button className="brand-secondary" disabled={loading || busy || offset === 0} onClick={() => setOffset((value) => Math.max(0, value - 50))}>Anterior</button><span>Página {offset / 50 + 1}</span><button className="brand-secondary" disabled={loading || busy || products.length < 50} onClick={() => setOffset((value) => value + 50)}>Próxima</button></nav>
        </section>
        <section className="brand-panel pos-cart"><h2>Venda atual</h2><p className="brand-state">Preços conferidos pela API ao concluir a venda.</p>
          <div className="pos-cart-items">{cart.length === 0 ? <p className="brand-state">Selecione produtos para montar o carrinho.</p> : cart.map((item, index) => <div className="pos-cart-item" key={`${item.product.id}:${item.location_id}`}><div><strong>{item.product.name}</strong><small>{quantityText(item.quantity_milli)} {item.product.unit} · {locations.find((place) => place.id === item.location_id)?.name || item.location_id}</small></div><b>{formatLocalCents(lineCents(item.product.price_cents, item.quantity_milli))}</b><button type="button" className="brand-secondary" aria-label={`Remover ${item.product.name}`} disabled={locked} onClick={() => setCart((items) => items.filter((_, position) => position !== index))}>×</button></div>)}</div>
          <div className="pos-total"><span>Total estimado</span><strong>{formatLocalCents(total)}</strong></div>
          <label>Dinheiro recebido (R$)<input inputMode="decimal" disabled={locked || cart.length === 0} placeholder="0,00" value={received} onChange={(event) => setReceived(event.target.value)} /></label>
          <p className="pos-change">Troco: {change === null || !cart.length ? '—' : formatLocalCents(change)}</p>
          <button type="button" className="brand-primary" disabled={locked || !cash || cart.length === 0 || total <= 0 || change === null || closing} onClick={submitSale}>{busy ? 'Conferindo operação…' : 'Concluir venda em dinheiro'}</button>
          <p className="pos-note">Cartão e Pix aguardam integração com confirmação de pagamento.</p>
        </section>
      </div>
      {receipt && <section className="brand-panel pos-receipt"><p className="brand-eyebrow">REGISTRO OPERACIONAL · NÃO FISCAL</p><h2>{receipt.status === 'cancelled' ? 'Venda cancelada' : 'Venda confirmada'}</h2><p>ID {receipt.sale_id}</p><p>Total registrado: <strong>{formatLocalCents(receipt.total_cents)}</strong></p><p>{receipt.items.length} linha(s) · Pagamento em dinheiro confirmado.</p></section>}
      <SalesHistory token={token} revision={historyRevision} onRead={setReceipt} />
    </main>
  </div>;
}
