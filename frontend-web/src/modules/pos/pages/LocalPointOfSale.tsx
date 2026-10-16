import {read95} from '../../../core/local/store95';
import Desk95 from './Desk95';import Report95 from './Report95';import Receipt95 from './Receipt95';import {GatewayContext95} from '../../../core/local/service95';import {archive95} from '../../../core/local/reportStore95';import type {Summary95} from '../../../core/local/report95.mjs';import type {Domain95} from '../../../core/local/desk95.mjs';import {words95} from '../../../core/local/words95';
import { useCallback, useContext, useEffect, useMemo, useRef, useState } from 'react';
import { Link } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { useLocalAccess } from '../../../core/local/LocalAccess';
import { localClient, localErrorMessage, useLocalSession } from '../../../core/local/useLocalSession';
import { LocalAPIError, formatLocalCents } from '../../../core/local/localClient.mjs';
import type { CashSession, LocalLocation, LocalProduct, LocalSession, SaleReceipt } from '../../../core/local/localClient.mjs';
import { cartTotal, lineCents, makeSale, parseMoney } from '../../../core/local/posModel.mjs';
import { addCartProduct, replaceCartQuantity, restoreCartItem, selectCatalogProduct } from '../../../core/local/cartEditing.mjs';
import type { CartItem } from '../../../core/local/posModel.mjs';
import { createPOSOperations } from '../../../core/local/posOperations.mjs';
import type { OperationOutcome, PendingOperation } from '../../../core/local/posOperations.mjs';
import CashClosing92 from './CashClosing92';
import Till94,{TillNav94} from './Till94';
import PaymentParts94 from './PaymentParts94';
import {useTill94} from '../../../core/local/useTill94';
import {fingerprint94,request94,withdraw94,suspend94,completeSuspended94,type Part94} from '../../../core/local/till94.mjs';
import {words94} from '../../../core/local/words94';
import SaleCustomer93 from './SaleCustomer93';
import {words93} from '../../../core/local/words93';
import type {Customer93} from '../../../core/local/reconciliation93.mjs';
import {checkoutScope91} from '../../../core/local/checkout91.mjs';
import SalesHistory from './SalesHistory';
import './localPOS.css';
import {words90} from '../../../core/operation90/words';
import {can90} from '../../../core/operation90/engine.mjs';
import {words91} from '../../../core/local/words91';
import {canSubmit91,confirmedChange91,tenderStore91,type Method91,type Fiscal91} from '../../../core/local/checkout91.mjs';
import {Payment91,Completion91,Club91} from './Checkout91';
import './checkout91.css';

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
// Keep the unfinished cart in this tab while the operator visits another area.
// A reload still clears it; the server always rechecks stock, price and shift.
let activeDraft: { token: string; items: CartItem[] } | null = null;

function CartQuantity({item,disabled,onCommit}:{item:CartItem;disabled:boolean;onCommit:(value:string)=>boolean}) {
  const [value,setValue]=useState(quantityText(item.quantity_milli));
  useEffect(()=>setValue(quantityText(item.quantity_milli)),[item.quantity_milli]);
  function commit(){if(!onCommit(value))setValue(quantityText(item.quantity_milli));}
  return <input aria-label={`Quantidade de ${item.product.name} em ${item.product.unit}`} inputMode="decimal" maxLength={32} disabled={disabled} value={value} onChange={e=>setValue(e.target.value)} onBlur={commit} onKeyDown={e=>{if(e.key==='Enter'){e.preventDefault();e.currentTarget.blur();}}}/>;
}

export default function LocalPointOfSale({ mode = 'sale' }: { mode?: 'sale' | 'cash' }) {
  const token = useLocalSession((state) => state.token);
  const session = useLocalSession((state) => state.session);
  if (!token || !session) return null;
  return <POSWorkspace key={checkoutScope91(session)} token={token} session={session} mode={mode} />;
}

function POSWorkspace({ token, session, mode }: { token: string; session: LocalSession; mode: 'sale' | 'cash' }) {
  const {t,i18n}=useTranslation();
  const operationWords90=words90(i18n.resolvedLanguage||i18n.language);
  const checkoutWords91=words91(i18n.resolvedLanguage||i18n.language);
  const cashPage=mode==='cash';const gateway95=useContext(GatewayContext95),completeWords95=words95(i18n.resolvedLanguage||i18n.language);const [domain95,setDomain95]=useState<Domain95>('payments'),[summary95,setSummary95]=useState<Summary95|undefined>();function details95(domain:Domain95){setPaying(false);setDomain95(domain);selectWork94('complete');}
  const tillWords94=words94(i18n.resolvedLanguage||i18n.language);
  const [work94,setWork94]=useState('sale'),[workDirty94,setWorkDirty94]=useState(false),[parts94,setParts94]=useState<Part94[]>([]),[paymentNotice94,setPaymentNotice94]=useState('');
  const draftID94=useRef<string>(crypto.randomUUID());
  const customerWords93=words93(i18n.resolvedLanguage||i18n.language);
  const [customer93,setCustomer93]=useState<Customer93>({document:'',club:'skip'});
  const [customerOpen93,setCustomerOpen93]=useState(false);
  const customerAsked93=useRef(false);
  const {capabilities}=useLocalAccess();
  const [operatorName,setOperatorName]=useState('');
  useEffect(()=>{if(!capabilities?.permissions?.includes('manage_staff'))return;let alive=true;
    void localClient.staff(token).then(rows=>{if(alive)setOperatorName(rows.find(row=>row.identity_id===session.identity_id)?.name||'');}).catch(()=>{});
    return()=>{alive=false;};},[capabilities?.permissions,session.identity_id,token]);
  const [historyRevision, setHistoryRevision] = useState(0);
  const [turnBusy, setTurnBusy] = useState(false);
  const turnLatch = useRef(false);
  const [closedReport, setClosedReport] = useState<{expected_cents:number;difference_cents:number;declared_cents:number}|null>(null);
  const invalidate = useLocalSession((state) => state.invalidate);
  const setup = useMemo(() => {
    try { return { operations: createPOSOperations(localClient, window.localStorage, session, browserLock),tenders:tenderStore91(window.localStorage,session), error: '' }; }
    catch (error) { return { operations: null,tenders:null, error: failureText(error) }; }
  }, [session]);
  const operations = setup.operations;
  const [cash, setCash] = useState<CashSession | null>(null);
  const till94=useTill94(checkoutScope91(session),cash?.session_id||'');
  const actor94={id:session.identity_id,role:capabilities?.role||'',permissions:capabilities?.permissions||[]};
  function selectWork94(next:string){if(workDirty94&&!window.confirm(tillWords94.unsaved))return;setWork94(next);setWorkDirty94(false);if(next!=='sale')setFocused(false);}
  const [gatewayGate95,setGatewayGate95]=useState(true);const [gatewayError95,setGatewayError95]=useState(false);
  const [knownCash, setKnownCash] = useState(false);
  const [products, setProducts] = useState<LocalProduct[]>([]);
  const [locations, setLocations] = useState<LocalLocation[]>([]);
  const [locationID, setLocationID] = useState('');
  const [search, setSearch] = useState('');
  const scanRef = useRef<HTMLInputElement>(null);
  const retailRef = useRef<HTMLDivElement>(null);
  const openingRef = useRef<HTMLInputElement>(null);
  const paymentRef = useRef<HTMLDialogElement>(null);
  const receivedRef = useRef<HTMLInputElement>(null);
  const [paying,setPaying]=useState(false);
  const [paymentMethod91,setPaymentMethod91]=useState<Method91>('cash');
  const [fiscal91,setFiscal91]=useState<Fiscal91>('none');
  const [fiscalDocument91,setFiscalDocument91]=useState('');
  const [club91,setClub91]=useState(false);
  const [completion91,setCompletion91]=useState<{receipt:SaleReceipt;names:Record<string,{name:string;unit:string}>;change:{total:number;received:number;change:number}|null}|null>(null);
  const [help,setHelp]=useState(false);
  const [lastProduct,setLastProduct]=useState('');
  const [removed,setRemoved]=useState<CartItem|null>(null);
  const [searchIndex,setSearchIndex]=useState(0);
  useEffect(()=>{const dialog=paymentRef.current;if(!dialog)return;if(paying&&!dialog.open){dialog.showModal();receivedRef.current?.focus();}else if(!paying&&dialog.open){dialog.close();scanRef.current?.focus();}},[paying]);
  const [offset, setOffset] = useState(0);
  const [reload, setReload] = useState(0);
  const [quantity, setQuantity] = useState('1');
  const [cart, setCart] = useState<CartItem[]>(()=>activeDraft && activeDraft.token===token ? activeDraft.items : []);
  useEffect(()=>{if(cashPage)return;if(!cart.length){draftID94.current=crypto.randomUUID();setParts94([]);setPaymentNotice94('');setFiscal91('none');setFiscalDocument91('');customerAsked93.current=false;setCustomer93({document:"",club:"skip"});setCustomerOpen93(false);}else if(!customerAsked93.current){customerAsked93.current=true;setCustomerOpen93(true);}},[cart.length,cashPage]);
  useEffect(()=>{activeDraft={token,items:cart};},[cart,token]);
  useEffect(()=>{if(knownCash&&!cash&&cart.length)setCart([]);},[knownCash,cash,cart.length]);
  const [opening, setOpening] = useState('');
  const [received, setReceived] = useState('');
  const [closing, setClosing] = useState(cashPage);
  const [focused,setFocused]=useState(false);
  const focusedOnce=useRef(false);
  useEffect(()=>{if(cash && !cashPage && !focusedOnce.current){focusedOnce.current=true;setFocused(true);}},[cash,cashPage]);
  useEffect(()=>{const workspace=document.querySelector('.workspace');if(!workspace)return;workspace.classList.toggle('workspace-pos-focused',focused && !cashPage && !!cash);return()=>workspace.classList.remove('workspace-pos-focused');},[focused,cashPage,cash]);
  useEffect(()=>{if(!cash && knownCash && !cashPage)openingRef.current?.focus();},[cash,knownCash,cashPage]);
  useEffect(()=>{if(retailRef.current)retailRef.current.inert=!cash;},[cash]);
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
        setLocationID((id) => places.some((place) => place.id === id && place.kind === 'shelf') ? id : places.find((place) => place.kind === 'shelf')?.id ?? '');
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

  useEffect(()=>{if(cashPage)setClosing(true);},[cashPage]);
  async function refreshCash(notify = false) {
    if (turnLatch.current) return;
    turnLatch.current = true; setTurnBusy(true);
    try {
      const current = await localClient.currentCash(token);
      if (active.current) { setCash(current); setKnownCash(true); if(notify){setError('');setMessage(current ? t('posUi.shiftOpen') : t('posUi.shiftClosed'));} }
    } catch (failure) {
      if (active.current) { setKnownCash(false); setError(`Confira o turno antes de continuar. ${failureText(failure)}`); }
    } finally { turnLatch.current=false; if(active.current)setTurnBusy(false); }
  }
  function confirmed(outcome: OperationOutcome) {
    if (outcome.kind === 'sale') {
      if(till94.data.suspended.some(r=>r.state==="resumed"&&r.reference===reference94))void till94.mutate(d=>completeSuspended94(d,actor94,reference94,outcome.result.sale_id));
      let tender=null;try{tender=setup.tenders?.get(outcome.result.operation_id)||null;}catch{/* The sale is confirmed even if tender metadata is unreadable. */}
      setCompletion91({receipt:outcome.result,names:Object.fromEntries([...products,...cart.map(item=>item.product)].map(product=>[product.id,{name:product.name,unit:product.unit}])),change:confirmedChange91(outcome.result,tender)});
      setReceipt(outcome.result); setCart([]); setRemoved(null); setLastProduct(''); setPaying(false); setReceived(''); setHistoryRevision(v=>v+1);
      setPaymentMethod91('cash');setFiscal91('none');setFiscalDocument91('');
      setCustomer93({document:'',club:'skip'});setCustomerOpen93(false);customerAsked93.current=false;
      setMessage(t(outcome.result.status === 'cancelled' ? 'posUi.saleCancelled' : 'posUi.saleConfirmed'));
    } else if (outcome.kind === 'open') { setOpening(''); setMessage('Abertura de caixa confirmada.'); }
    else {
      const result = outcome.result;
      const difference = result.difference_cents;
      const archived95:Summary95={scope:checkoutScope91(session),mode:gateway95.mode,cash:result.session_id,expected:result.expected_cents,counted:result.expected_cents+difference,difference,at:new Date().toISOString()};setSummary95(archived95);void archive95(archived95).catch(()=>setError(completeWords95.storageError));
      setClosedReport({expected_cents:result.expected_cents,difference_cents:difference,declared_cents:result.expected_cents+difference});
      setClosing(false);
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
  let receivedCents91:number|null=null;try{receivedCents91=parseMoney(received);}catch{/* Incomplete input remains unconfirmed. */}
  const reference94=draftID94.current+':'+fingerprint94(cart);
  useEffect(()=>{let alive=true;setGatewayGate95(true);setGatewayError95(false);void read95(checkoutScope91(session),gateway95.mode).then(rows=>{if(alive)setGatewayGate95(rows.some(r=>r.source.startsWith(draftID94.current+':')&&['payments','credit','discount'].includes(r.domain)&&['pending','unknown','approved'].includes(r.state)));}).catch(()=>{if(alive)setGatewayError95(true);});return()=>{alive=false;};},[session,gateway95.mode,reference94,work94]);
  function changeParts94(next:Part94[],clear=false){const apply=()=>{setParts94(next);setPaymentNotice94("");if(clear){setPaymentMethod91("cash");setReceived("");}};const ids=till94.data.requests.filter(r=>r.kind==="payment"&&r.reference===reference94&&["requested","approved"].includes(r.state)).map(r=>r.id);if(ids.length)void till94.mutate(d=>{for(const id of ids)withdraw94(d,actor94,id);}).then(ok=>{if(ok)apply();});else apply();}
  const discountBlocking94=till94.ready&&till94.data.requests.some(r=>r.kind==='discount'&&r.reference===reference94&&['requested','approved'].includes(r.state));
  const canCheckout91=! gatewayGate95&&!gatewayError95&&till94.ready&&!till94.busy&&till94.data.cash===cash?.session_id&&!till94.error&&!parts94.length&&!discountBlocking94&&canSubmit91({method:paymentMethod91,fiscal:fiscal91,total,received:receivedCents91});
  const locked = busy || turnBusy || loading || !!pending || !operations || !knownCash || !!calculationError;
  const [quickIDs,setQuickIDs]=useState<string[]>([]);
  const quickKey=`titan-pos-quick:${session.tenant_id}:${session.store_id}:${session.identity_id}`;
  useEffect(()=>{try{const value:unknown=JSON.parse(localStorage.getItem(quickKey)||'[]');setQuickIDs(Array.isArray(value)?value.filter((id:unknown):id is string=>typeof id==='string').slice(0,8):[]);}catch{setQuickIDs([]);}},[quickKey]);
  function toggleQuick(id:string){
    const next=quickIDs.includes(id)?quickIDs.filter(value=>value!==id):[...quickIDs,id].slice(-8);
    try{localStorage.setItem(quickKey,JSON.stringify(next));setQuickIDs(next);}catch{setError(t('posUi.quickSaveError'));}
  }
  function discardDraft(){
    if(locked||paying||!cart.length)return;
    if(!window.confirm(t('posUi.discardConfirm')))return;
    setCustomer93({document:'',club:'skip'});customerAsked93.current=false;setCustomerOpen93(false);
    setCart([]);setRemoved(null);setLastProduct('');setReceived('');setSearch('');setError('');setMessage(t('posUi.discarded'));scanRef.current?.focus();
  }
  useEffect(()=>{if(cashPage||customerOpen93||club91||work94!=='sale')return;const onKey=(event:KeyboardEvent)=>{if(event.altKey||event.ctrlKey||event.metaKey||event.repeat)return;
    if(event.key==='Escape'){if(paying){setPaying(false);}else if(focused){setFocused(false);}else{setSearch('');scanRef.current?.focus();}return;}
    if(!cash || locked || closing)return;
    if(event.key==='F2'){event.preventDefault();scanRef.current?.focus();}
    if(event.key==='F4' && cart.length && !paying){event.preventDefault();setReceived('');setPaying(true);}
    if(event.key==='F8'){event.preventDefault();selectWork94('approvals');}
  };window.addEventListener('keydown',onKey);return()=>window.removeEventListener('keydown',onKey);},[cashPage,cash,locked,closing,cart.length,paying,focused,t,customerOpen93,club91,work94]);
  const filtered = products.filter((product) => `${product.name} ${product.sku} ${product.barcode || ''}`.toLocaleLowerCase('pt-BR').includes(search.toLocaleLowerCase('pt-BR')));
  function addProduct(product: LocalProduct) {
    try {
      if (locked || customerOpen93 || !cash || !locationID) throw new Error(t('posUi.needLocation'));
      const next=addCartProduct(cart,product,locationID,quantity);
      if(!customerAsked93.current){customerAsked93.current=true;setCustomerOpen93(true);}
      setCart(next); setReceipt(null); setRemoved(null); setLastProduct(product.id); setSearch(''); setQuantity('1'); setSearchIndex(0); setError('');scanRef.current?.focus();
    } catch (failure) { setError(failureText(failure)); }
  }
  function editQuantity(index:number,value:string):boolean {
    if(locked||paying)return false;
    try {setCart(replaceCartQuantity(cart,index,value));setRemoved(null);setError('');return true;}
    catch(failure){setError(failureText(failure));return false;}
  }
  function undoRemove(){if(!removed||locked||paying)return;try{setCart(restoreCartItem(cart,removed));setLastProduct(removed.product.id);setRemoved(null);setError('');scanRef.current?.focus();}catch(failure){setError(failureText(failure));}}
  function scan(){if(locked||paying||closing||customerOpen93)return;try{addProduct(selectCatalogProduct(products,search,searchIndex));}catch(failure){setError(failureText(failure));}}

  function submitSale() {
    if (!operations || !cash || locked || customerOpen93 || club91 || change === null) return;
    if(!canCheckout91||receivedCents91===null||!setup.tenders)return;
    try { const input = makeSale(cart, cash.session_id);setup.tenders.save(input,receivedCents91); void run(() => operations.start(token, 'sale', input)); }
    catch (failure) { setError(failure instanceof Error&&checkoutWords91[failure.message]?checkoutWords91[failure.message]:failureText(failure)); }
  }
  return <div className={`brand-page brand-app-page pos-page ${cashPage?"pos-cash-page":""} ${!cash&&knownCash&&!cashPage?"pos-turn-closed":""}`}>

    <main className="brand-app-content pos-content">
      <div className="brand-app-heading pos-heading"><div><h1>{t(cashPage?"posUi.cashTitle":"posUi.title")}</h1><p className="brand-subtitle">{t("posUi.operator")}: {operatorName||t("posUi.operatorFallback")} · {cash?t("posUi.open"):knownCash?t("posUi.closed"):t("posUi.unconfirmed")}</p></div>{!cashPage&&<button type="button" className="brand-secondary pos-focus-toggle" aria-pressed={focused} onClick={()=>setFocused(v=>!v)}>{t(focused?"posUi.reduce":"posUi.expand")}</button>}
        {focused&&!cashPage&&<Link className="brand-secondary pos-menu-return" to="/local/home" onClick={()=>setFocused(false)}>← {t('navigation.backToMenu')}</Link>}
        {!cashPage&&cash&&<Link className="brand-secondary checkout91-close-register" to="/local/cash/register" aria-disabled={locked||paying||cart.length>0} onClick={e=>{if(locked||paying||cart.length){e.preventDefault();setError(checkoutWords91.finishSaleFirst);}else setFocused(false);}}>{checkoutWords91.closeRegister}</Link>}
        {!cashPage&&<button type="button" className="brand-secondary" disabled={busy||paying} onClick={()=>setClub91(true)}>{checkoutWords91.clubRegister}</button>}
        {!focused&&<Link className="brand-secondary" to={'/local/operation?area='+(cashPage?'closing':'pos')}>{operationWords90.operationCenter}</Link>}
        <button className="brand-secondary" disabled={busy || loading || turnBusy} onClick={() => void refreshCash(true)}>{t(turnBusy?"posUi.checkingShift":"posUi.checkShift")}</button></div>
      {(error || calculationError) && <p role="alert" className="brand-message error">{error || calculationError}</p>}
      {message && <p role="status" className="brand-message">{message}</p>}
      {pending && <section className="brand-panel pos-pending" role="status"><h2>Operação pendente de conferência</h2>
        <p>{pending.kind === 'sale' ? `Venda ${pending.input.sale_id}` : `Turno ${pending.input.session_id}`}. O carrinho fica bloqueado até confirmar o resultado.</p>
        <button className="brand-small-primary" disabled={busy} onClick={() => operations && void run(() => operations.retry(token))}>Verificar / reenviar mesma operação</button>
        {canDiscard && <button className="brand-secondary" disabled={busy} onClick={() => operations && void run(() => operations.discardRejected(token))}>Editar após recusa confirmada</button>}
      </section>}
      {(cashPage||!cash)&&<section className="brand-panel pos-cash"><div><h2>{loading ? t('posUi.checkingShift') : cash ? t('posUi.open') : knownCash ? t('posUi.openPrompt') : t('posUi.unconfirmed')}</h2>
        {cash && <p>{t("posUi.open")}</p>}</div>
        {!cash && knownCash && <form onSubmit={(event) => { event.preventDefault(); if (!operations || locked) return; try { const input = { session_id: crypto.randomUUID(), opening_cents: parseMoney(opening) }; void run(() => operations.start(token, 'open', input)); } catch (failure) { setError(failureText(failure)); } }}>
          <label>{t("posUi.openingAmount")}<input ref={openingRef} required inputMode="decimal" value={opening} disabled={locked} placeholder="0,00" onChange={(event) => setOpening(event.target.value)} /></label>
          <button className="brand-small-primary" disabled={locked}>{t("posUi.openRegister")}</button></form>}

      </section>}
      {cashPage&&<p><Link className="brand-secondary pos-back" to="/local/pos">{t("posUi.backToSale")}</Link></p>}
      {cashPage && cash && work94!=='complete' && <CashClosing92 key={cash.session_id} token={token} scope={checkoutScope91(session)} cash={cash.session_id} disabled={locked || cart.length > 0} manager={capabilities?.role==="owner" || capabilities?.role==="manager"} onClose={amount=>{if(!operations || locked || cart.length)return;void run(()=>operations.start(token, 'close', {session_id:cash.session_id,operation_id:crypto.randomUUID(),declared_cents:amount}));}}/>}
      {cashPage&&cash&&<button className="brand-secondary" onClick={()=>{setDomain95('pairing');setWork94('complete');}}>{completeWords95.viewPairing}</button>}
      {cashPage&&cash&&work94==='complete'&&<button className="brand-secondary" onClick={()=>selectWork94('sale')}>{checkoutWords91.backPayment}</button>}
      {cashPage&&cash&&work94==='complete'&&<Desk95 scope={checkoutScope91(session)} token={token} cash={cash.session_id} reference={reference94} total={total} parts={[]} actor={actor94} initial="pairing" onDirty={setWorkDirty94}/>}
      {cashPage && closedReport && <section className="brand-panel pos-closing-report" role="status"><h2>{t("posUi.closeResult")}</h2><div className="pos-payment-status"><div><span>{t("posUi.declaredCash")}</span><strong>{formatLocalCents(closedReport.declared_cents)}</strong></div><div><span>{t("posUi.expectedCash")}</span><strong>{formatLocalCents(closedReport.expected_cents)}</strong></div><div><span>{closedReport.difference_cents===0?t('posUi.balanced'):closedReport.difference_cents<0?t('posUi.shortage'):t('posUi.surplus')}</span><strong>{formatLocalCents(Math.abs(closedReport.difference_cents))}</strong></div></div></section>}
      {!cashPage&&<TillNav94 tab={work94} onChange={selectWork94} closeBlocked={cart.length>0} onCloseBlocked={()=>setError(checkoutWords91.finishSaleFirst)} disabled={locked||paying||customerOpen93||club91||!cash}/>}
      {cashPage&&summary95&&<Report95 scope={checkoutScope91(session)} cash={summary95.cash} official={summary95}/>}
      {!cashPage&&['complete','connections'].includes(work94)&&<Desk95 key={work94+domain95} scope={checkoutScope91(session)} token={token} cash={cash?.session_id||''} reference={reference94} total={total} parts={parts94} actor={actor94} initial={work94==='connections'?'devices':domain95} onDirty={setWorkDirty94}/>}
      {!cashPage&&!['sale','complete','connections'].includes(work94)&&<Till94 store={till94} actor={actor94} tab={work94} token={token} reference={reference94} gross={total} products={products} onDirty={setWorkDirty94} onResume={(next,reference)=>{draftID94.current=reference.slice(0,reference.indexOf(":"));setCart(next);setReceived("");setParts94([]);setWork94("sale");setFocused(true);setMessage(tillWords94.resumedHelp);}}/>}
      {!cashPage&&work94==='sale'&&cart.length>0&&<button className="brand-secondary" disabled={locked||paying||!!parts94.length||!till94.ready||till94.busy} onClick={()=>{if(window.confirm(tillWords94.suspendHelp))void till94.mutate(d=>suspend94(d,actor94,cart,reference94)).then(ok=>{if(ok){setCart([]);setReceived("");setWork94("suspended");setFocused(false);}});}}>{tillWords94.suspendSale}</button>}
      {!cashPage&&work94==='sale'&&cart.length>0&&<div className="customer93-summary"><span>{customerWords93.customer}: {customer93.document?'***.***.***-'+customer93.document.slice(-2):customerWords93.noCustomer}</span>{customer93.club!=='skip'&&<span>{customerWords93.clubIntent}</span>}<button className="brand-secondary" disabled={locked||paying} onClick={()=>setCustomerOpen93(true)}>{customerWords93.editCustomer}</button></div>}
      {!cashPage&&work94==='sale' && <div ref={retailRef} className="pos-retail-grid" aria-disabled={!cash}>
        <div className="pos-selling">
          <form className="pos-scan" onSubmit={e=>{e.preventDefault();scan();}}>
            <label className="pos-scan-search">{t('posUi.search')} <kbd>F2</kbd><input ref={scanRef} autoFocus autoComplete="off" value={search} disabled={locked||paying||closing} placeholder={t('posUi.searchPlaceholder')} onChange={e=>{setSearch(e.target.value);setSearchIndex(0);}} onKeyDown={e=>{if(e.key==='ArrowDown'||e.key==='ArrowUp'){e.preventDefault();setSearchIndex(i=>Math.min(Math.max(i+(e.key==='ArrowDown'?1:-1),0),Math.max(0,filtered.length-1)));}if(e.key==='Escape'){setSearch('');setSearchIndex(0);}}}/></label>
            <label className="pos-scan-qty">{t("posUi.quantity")}<input inputMode="decimal" maxLength={32} value={quantity} disabled={locked||paying||closing} onChange={e=>setQuantity(e.target.value)}/></label>
            <button className="brand-small-primary" disabled={locked||!cash||!locationID||!search.trim()||closing||paying}>{t("posUi.add")} ↵</button>
          </form>
          <div className="pos-stock-source"><label>{t("posUi.pickupLocation")}<select value={locationID} disabled={locked||paying} onChange={e=>setLocationID(e.target.value)}><option value="">{t("posUi.selectLocation")}</option>{locations.map(place=><option key={place.id} value={place.id}>{place.name}</option>)}</select></label><span>{t("posUi.locationHelp")}</span></div>
          {!loading&&locations.length===0&&<p className="brand-state">{t('posUi.noLocations')}</p>}
          {quickIDs.length>0&&<section className="pos-quick" aria-label={t('posUi.quickTitle')}><strong>{t('posUi.quickTitle')}</strong><div>{quickIDs.map(id=>{const product=products.find(row=>row.id===id);return product?<button type="button" key={id} className="brand-secondary" disabled={locked||!cash||!locationID||paying||closing} onClick={()=>addProduct(product)}>{product.name} ＋</button>:null;})}</div><small>{t('posUi.quickLoadedOnly')}</small></section>}
          {search.trim()&&<section className="brand-panel pos-search-results" aria-label="Resultados da busca"><p>Busca na página {offset/50+1} · Use ↑ ↓ e Enter para adicionar</p>{filtered.length===0?<p>Produto não encontrado nesta página.</p>:filtered.map((product,i)=><div className="pos-product-row" key={product.id}><button type="button" className={`pos-product ${i===searchIndex?'pos-search-active':''}`} disabled={locked||!cash||!locationID||closing||paying} onClick={()=>addProduct(product)}><span><strong>{product.name}</strong><small>{product.sku} · {product.unit}</small></span><b>{formatLocalCents(product.price_cents)}</b><span aria-hidden="true">＋</span></button><button type="button" className="pos-pin" aria-label={t(quickIDs.includes(product.id)?'posUi.removeQuick':'posUi.addQuick',{name:product.name})} aria-pressed={quickIDs.includes(product.id)} onClick={()=>toggleQuick(product.id)}>{quickIDs.includes(product.id)?'★':'☆'}</button></div>)}</section>}
          <section className="brand-panel pos-basket"><div className="pos-basket-heading"><div><h2>{t("posUi.currentSale")}</h2><p>{t("posUi.priceNotice")}</p></div><span className="pos-pill">{cart.length} linha(s)</span></div>
            <div className="pos-table-scroll"><table className="pos-sale-table"><thead><tr><th scope="col">Produto</th><th scope="col">Quantidade</th><th scope="col">Preço / unidade</th><th scope="col">Subtotal</th><th scope="col"><span className="pos-sr-only">Ações</span></th></tr></thead><tbody>{cart.map((item,index)=><tr className={lastProduct===item.product.id?'pos-last-row':''} key={`${item.product.id}:${item.location_id}`}><td><strong>{item.product.name}</strong><small>{item.product.sku} · {item.product.unit} · {locations.find(p=>p.id===item.location_id)?.name||item.location_id}</small></td><td><CartQuantity item={item} disabled={locked||paying} onCommit={value=>editQuantity(index,value)}/></td><td>{formatLocalCents(item.product.price_cents)}</td><td><strong>{formatLocalCents(lineCents(item.product.price_cents,item.quantity_milli))}</strong></td><td><button type="button" className="pos-remove" disabled={locked||paying} aria-label={`Remover ${item.product.name}`} onClick={()=>{setRemoved(item);setCart(items=>items.filter((_,i)=>i!==index));scanRef.current?.focus();}}>×</button></td></tr>)}</tbody></table>{cart.length===0&&<div className="pos-empty"><span aria-hidden="true">▤</span><h3>Pronto para começar</h3><p>{t('posUi.emptyPrompt')}</p></div>}</div>
            <div className="pos-last-item" aria-live="polite">{lastProduct&&cart.find(i=>i.product.id===lastProduct)?`Último produto: ${cart.find(i=>i.product.id===lastProduct)?.product.name}`:'Aguardando o próximo produto'}{removed&&<button className="brand-secondary" disabled={locked||paying} onClick={undoRemove}>Desfazer remoção de {removed.product.name}</button>}</div>
          </section>
          <details className="pos-catalog-browser"><summary>Consultar catálogo · página {offset/50+1}</summary><div className="pos-product-list">{products.map(product=><button type="button" className="pos-product" key={product.id} disabled={locked||!cash||!locationID||closing||paying} onClick={()=>addProduct(product)}><span><strong>{product.name}</strong><small>{product.sku} · {product.unit}</small></span><b>{formatLocalCents(product.price_cents)}</b><span aria-hidden="true">＋</span></button>)}</div></details>
          <nav className="brand-pagination" aria-label="Páginas de produtos"><button className="brand-secondary" disabled={loading||busy||paying||offset===0} onClick={()=>setOffset(v=>Math.max(0,v-50))}>Anterior</button><span>Página {offset/50+1}</span><button className="brand-secondary" disabled={loading||busy||paying||products.length<50} onClick={()=>setOffset(v=>v+50)}>Próxima</button><button className="brand-secondary" disabled={loading||busy||paying} onClick={()=>{setError('');setReload(v=>v+1);}}>Atualizar catálogo</button></nav>
        </div>
        <aside className="brand-panel pos-summary"><p className="brand-eyebrow">{t("posUi.summary")}</p><h2>{t("posUi.estimatedTotal")}</h2><strong className="pos-grand-total">{formatLocalCents(total)}</strong><dl className="pos-summary-lines"><div><dt>Subtotal</dt><dd>{formatLocalCents(total)}</dd></div><div><dt>Linhas de produtos</dt><dd>{cart.length}</dd></div><div><dt>Unidades (produtos unitários)</dt><dd>{String(cart.filter(i=>i.product.unit==='unit').reduce((n,i)=>n+BigInt(i.quantity_milli),0n)/1000n)}</dd></div></dl><p className="pos-note">Produtos por peso, volume ou comprimento mantêm a quantidade e a unidade de cada linha.</p>
          <div className="pos-status-stack"><div><span>Venda</span><strong>{pending?.kind==='sale'?'Pendente de conferência':cart.length?'Em edição':receipt?'Consulte o registro abaixo':'Não iniciada'}</strong></div><div><span>Pagamento atual</span><strong>{pending?.kind==='sale'?'Resultado não confirmado':busy&&paying?'Conferindo venda':receipt?.status==='committed'&&!cart.length?checkoutWords91.paymentConfirmed:'Não registrado'}</strong></div><div><span>Documento fiscal</span><strong>Não emitido</strong></div></div>
          <div className="pos-primary-actions"><button type="button" className="brand-primary pos-checkout" disabled={locked||!cash||!cart.length||total<=0||closing} onClick={()=>{setReceived('');setPaying(true);}}>{t('posUi.pay')} · F4</button><button type="button" className="brand-secondary pos-discount" disabled={locked||!cart.length} onClick={()=>selectWork94('approvals')}>{t('posUi.discount')} · F8</button></div>
          <button type="button" className="brand-secondary pos-discard" disabled={locked||paying||!cart.length} onClick={discardDraft}>{t('posUi.discard')}</button>
          <p className="pos-note">{t("posUi.cashOnly")}</p>
        </aside>
      </div>}
      {!cashPage&&work94==='sale' && <footer className="pos-statusbar"><span>{cash?t("posUi.open"):t("posUi.closed")}</span><div className="pos-shortcut-items"><span><kbd>F2</kbd> {t('posUi.keySearch')}</span><span><kbd>F4</kbd> {t('posUi.keyPay')}</span><span><kbd>F8</kbd> {t('posUi.keyDiscount')}</span><span><kbd>Esc</kbd> {t('posUi.keyBack')}</span></div><button className="brand-secondary" aria-expanded={help} onClick={()=>setHelp(v=>!v)}>Ajuda</button></footer>}
      {!cashPage&&help&&<section className="brand-panel pos-help"><h2>Operação do caixa</h2><p>Abra seu turno, confira o local de retirada e registre por SKU, código de barras ou nome. A busca consulta a página carregada; navegue pelo catálogo para outros produtos. Altere quantidades diretamente na linha; Enter confirma a correção. Remova e desfaça a última remoção quando necessário.</p><p>A venda só é registrada após confirmação da API. Se houver perda de resposta, use a operação pendente; não abra outra venda para substituir a anterior. O carrinho ainda em edição não é salvo ao recarregar. </p></section>}
      <dialog ref={paymentRef} className="pos-payment-dialog checkout91-dialog" aria-labelledby="pos-payment-title" onCancel={e=>{if(busy){e.preventDefault();return;}setPaying(false);}} onClose={()=>{setPaying(false);scanRef.current?.focus();}}><div className="pos-dialog-heading"><h2 id="pos-payment-title">{checkoutWords91.checkoutTitle}</h2><button type="button" className="brand-secondary" aria-label={t('posUi.backToSale')} disabled={busy} onClick={()=>setPaying(false)}>×</button></div><Payment91 cart={cart} total={total} method={paymentMethod91} setMethod={setPaymentMethod91} fiscal={fiscal91} setFiscal={setFiscal91} document={fiscalDocument91} setDocument={setFiscalDocument91} received={received} setReceived={setReceived} inputRef={receivedRef} disabled={locked||!cart.length} busy={busy} pending={!!pending} error={error} onSubmit={submitSale} paymentTools={<><PaymentParts94 parts={parts94} total={total} received={received} onReceived={setReceived} disabled={locked||!till94.ready||till94.busy} onChange={next=>changeParts94(next)} onClear={()=>changeParts94([],true)} onSave={()=>{void till94.mutate(d=>request94(d,actor94,{kind:'payment',amount:total,reason:tillWords94.payment,reference:reference94,parts:parts94,received:receivedCents91})).then(ok=>{if(ok)setPaymentNotice94('compositionSaved');});}}/>{paymentNotice94&&<p role="status">{tillWords94[paymentNotice94]}</p>}{till94.error&&<p role="alert">{tillWords94[till94.error]||tillWords94.invalid}</p>}{(gatewayGate95||gatewayError95)&&<p role="alert">{gatewayError95?completeWords95.storageError:completeWords95.paymentGate95}</p>}{discountBlocking94&&<p role="alert">{tillWords94.discountBlocking}</p>}</>} hideSimpleCash={parts94.length>0} canSubmit={canCheckout91&&!!cash&&cart.length>0&&!closing} canFiscal={can90(capabilities,'fiscal')} onClub={()=>setClub91(true)} onDetails={details95}/></dialog>
      {!cashPage&&customerOpen93&&<SaleCustomer93 value={customer93} onContinue={value=>{setCustomer93(value);setFiscalDocument91(value.document);setCustomerOpen93(false);if(value.club==='join')setClub91(true);else scanRef.current?.focus();}}/>}
      {!cashPage&&completion91&&<Completion91 receipt={completion91.receipt} names={completion91.names} change={completion91.change} onNext={()=>{setCompletion91(null);setReceived('');scanRef.current?.focus();}}/>}
      {club91&&<Club91 session={session} initialDocument={customer93.document} onClose={()=>setClub91(false)}/>}
      {!cashPage&&work94==='sale'&&!focused&&receipt && <section className="brand-panel pos-receipt"><h2>{t(receipt.status === 'cancelled' ? 'posUi.saleCancelled' : 'posUi.saleConfirmed')}</h2><p>{t('posUi.recordedTotal')}: <strong>{formatLocalCents(receipt.total_cents)}</strong></p><details><summary>{t('posUi.details')}</summary><code>{receipt.sale_id}</code></details></section>}
      {!cashPage&&work94==='sale'&&!focused&&receipt&&<Receipt95 receipt={receipt} names={Object.fromEntries(products.map(p=>[p.id,{name:p.name,unit:p.unit}]))}/>}
      {!cashPage&&work94==='sale'&&!focused&&<SalesHistory token={token} revision={historyRevision} onRead={setReceipt} />}
    </main>
  </div>;
}
