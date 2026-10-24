import {retainDisposable} from '../../../core/local/retainDisposable.mjs';
import { useEffect, useMemo, useRef, useState, type DependencyList } from 'react';
import { useTranslation } from 'react-i18next';
import { useLocalSession } from '../../../core/local/useLocalSession';
import { useLocalAccess } from '../../../core/local/LocalAccess';
import { createReceivingOverview, type ReceivingOverviewSource } from '../../../core/receivingOverview/source.mjs';
import { overviewFilters, receivingQuantity, receivingCSV, type ReceivingFilters } from '../../../core/receivingOverview/model.mjs';
import type { ReceivingBoard, ReceivingOverview as Overview } from '../../../core/receivingOverview/contracts';
import { overviewWords } from '../../../core/receivingOverview/words.mjs';
import { dockLocalInput } from '../../../core/dockAgenda/model.mjs';
import type { PurchaseOrder } from '../../../core/local/purchaseClient.mjs';
import './receivingOverview.css';
type Props = {
    order: PurchaseOrder | null;
    refreshVersion?: number;
};
export default function ReceivingOverview(props: Props) {
    const token = useLocalSession(s => s.token), session = useLocalSession(s => s.session), { capabilities } = useLocalAccess(), { i18n } = useTranslation(), locale = i18n.resolvedLanguage || i18n.language, w = overviewWords(locale), latest = useRef(capabilities);
    latest.current = capabilities;
    const source = useMemo(() => {
        try {
            return token && session && capabilities ? createReceivingOverview({ token, session, capabilities, isCurrent: () => { const current = useLocalSession.getState(); return current.token === token && current.session === session && latest.current === capabilities; } }) : null;
        }
        catch {
            return null;
        }
    }, [token, session, capabilities]);
    return source ? <ReceivingViews key={source.scope + token + JSON.stringify(capabilities)} {...props} source={source} locale={locale}/> : <section className="brand-panel receiving-overview"><h2>{w.title}</h2><p>{w.permission}</p></section>;
}
function useRead<T>(read: (signal: AbortSignal) => Promise<T>, signature: string, deps: DependencyList) {
    const [result, setResult] = useState<{
        signature: string;
        value?: T;
        error: string;
        loading: boolean;
    }>({ signature: '', error: '', loading: true });
    useEffect(() => {
        let live = true;
        const ctrl = new AbortController();
        setResult({ signature, error: '', loading: true });
        void read(ctrl.signal).then(value => {
            if (live)
                setResult({ signature, value, error: '', loading: false });
        }).catch(error => {
            if (live)
                setResult({ signature, error: errorKey(error), loading: false });
        });
        return () => { live = false; ctrl.abort(); };
    }, deps);
    return result.signature === signature ? result : { signature, error: '', loading: true, value: undefined };
}
const inputTime = (value: Date) => { const n = (v: number) => String(v).padStart(2, '0'); return `${value.getFullYear()}-${n(value.getMonth() + 1)}-${n(value.getDate())}T${n(value.getHours())}:${n(value.getMinutes())}:00`; };
const start = () => { const date = new Date(); date.setHours(0, 0, 0, 0); return inputTime(date); };
export function ReceivingViews({ order, source, locale, refreshVersion=0 }: Props & {
    source: ReceivingOverviewSource;
    locale: string;
}) {
    const w = overviewWords(locale), [draft, setDraft] = useState({ from: start(), to: inputTime(new Date(Date.now() + 86400000)), supplier_id: '', product_id: '', state: 'pending' as NonNullable<ReceivingFilters['state']> }), [filters, setFilters] = useState<Required<ReceivingFilters>>(), [offset, setOffset] = useState(0), [overviewOffset, setOverviewOffset] = useState(0), [selected, setSelected] = useState(order?.id || ''), [refresh, setRefresh] = useState(0), [error, setError] = useState('');
    useEffect(() => retainDisposable(source), [source]);
    useEffect(() => { setSelected(order?.id || ''); setOverviewOffset(0); }, [order?.id]);
    const boardKey = JSON.stringify([filters, offset, refresh, refreshVersion]), detailKey = JSON.stringify([selected, overviewOffset, refresh, refreshVersion]), board = useRead(s => filters ? source.board(filters, offset, s) : Promise.resolve(undefined), boardKey, [source, boardKey]), detail = useRead(s => selected ? source.overview(selected, overviewOffset, s) : Promise.resolve(undefined), detailKey, [source, detailKey]);
    const date = (v: string) => new Intl.DateTimeFormat(locale, { dateStyle: 'short', timeStyle: 'medium' }).format(new Date(v)), quantity = (v: number | string) => receivingQuantity(v, locale);
    function query() {
        try {
            const next = overviewFilters({ ...draft, from: dockLocalInput(draft.from), to: dockLocalInput(draft.to) });
            setFilters(next);
            setOffset(0);
            setError('');
        }
        catch (e) {
            setError(errorKey(e));
        }
    }
    function download(name: string, rows: unknown[][]) {
        try {
            source.assertCurrent();
            const url = URL.createObjectURL(new Blob([receivingCSV(rows)], { type: 'text/csv;charset=utf-8' })), link = document.createElement('a');
            link.href = url;
            link.download = name;
            link.click();
            setTimeout(() => URL.revokeObjectURL(url), 1000);
        }
        catch (e) {
            setError(errorKey(e));
        }
    }
    function exportBoard(value: ReceivingBoard) {
        const rows: unknown[][] = [['record_type', 'from', 'to', 'state', 'supplier_id', 'product_id', 'unit', 'order_id', 'orders', 'planned_milli_exact', 'accepted_milli_exact', 'remaining_milli_exact', 'pending_milli_exact', 'commercial_status', 'receiving_status', 'review_flags', 'measured_at']];
        for (const g of value.groups)
            rows.push(['group', value.from, value.to, value.state, g.supplier_id, g.product_id, g.unit, '', g.orders, g.planned_milli_exact, g.accepted_milli_exact, g.remaining_milli_exact, g.pending_milli_exact, '', '', '', value.measured_at]);
        for (const row of value.items)
            rows.push(['order', value.from, value.to, value.state, row.supplier_id, row.product_id, row.unit, row.order_id, '', String(row.planned_milli), String(row.accepted_milli), String(row.remaining_milli), '', row.commercial_status, row.receiving_status, row.review_flags.join(' | '), value.measured_at]);
        download('Titan_Recebimentos_Grupos_e_Pagina.csv', rows);
    }
    function exportOverview(value: Overview) {
        const rows: unknown[][] = [['record_type', 'order_id', 'supplier_id', 'product_id', 'unit', 'planned_milli', 'effective_delivered_milli', 'effective_accepted_milli', 'partial_rejected_milli', 'remaining_milli', 'fully_rejected_milli_exact', 'voided_accepted_milli_exact', 'commercial_status', 'receiving_status', 'review_flags', 'appointment_id', 'dock_id', 'agenda_status', 'starts_at', 'ends_at', 'supplier_confirmation', 'payment_evaluation', 'measured_at']];
        rows.push(['summary', value.order_id, value.supplier_id, value.product_id, value.unit, value.planned_milli, value.effective_delivered_milli, value.effective_accepted_milli, value.partial_rejected_milli, value.remaining_milli, value.fully_rejected_milli_exact, value.voided_accepted_milli_exact, value.commercial_status, value.receiving_status, value.review_flags.join(' | '), '', '', '', '', '', value.supplier_confirmation, value.payment_evaluation, value.measured_at]);
        for (const a of value.appointments)
            rows.push(['appointment', value.order_id, value.supplier_id, '', '', '', '', '', '', '', '', '', '', '', '', a.appointment_id, a.dock_id, a.status, a.starts_at, a.ends_at, value.supplier_confirmation, value.payment_evaluation, value.measured_at]);
        download('Titan_Conferencia_Recebimento_Pagina.csv', rows);
    }
    const currentBoard = board.value, currentDetail = detail.value?.order_id === selected ? detail.value : undefined;
    return <section className="brand-panel receiving-overview" aria-label={w.title}><header><h2>{w.title}</h2><button onClick={() => setRefresh(n => n + 1)}>{w.refresh}</button></header><p>{w.help}</p>{error && <p role="alert">{w[error] || w.unavailable}</p>}
 <section><h3>{w.board}</h3><p>{w.scopeHelp}</p><p>{w.limitsHelp}</p><form className="receiving-overview-filters" onSubmit={e => { e.preventDefault(); query(); }}><label>{w.from}<input required type="datetime-local" step="1" value={draft.from} onChange={e => setDraft({ ...draft, from: e.target.value })}/></label><label>{w.toExclusive}<input required type="datetime-local" step="1" value={draft.to} onChange={e => setDraft({ ...draft, to: e.target.value })}/></label><label>{w.supplierID}<input maxLength={128} value={draft.supplier_id} onChange={e => setDraft({ ...draft, supplier_id: e.target.value })}/></label><label>{w.productID}<input maxLength={128} value={draft.product_id} onChange={e => setDraft({ ...draft, product_id: e.target.value })}/></label><label>{w.status}<select value={draft.state} onChange={e => setDraft({ ...draft, state: e.target.value as typeof draft.state })}>{(['pending', 'all', 'received', 'cancelled', 'review'] as const).map(state => <option key={state} value={state}>{w[state]}</option>)}</select></label><button>{w.query}</button><button type="button" disabled={!order} onClick={() => setDraft({ ...draft, supplier_id: order?.supplier_id || '', product_id: order?.items[0]?.product_id || '' })}>{w.useSelected}</button></form><p>{Intl.DateTimeFormat().resolvedOptions().timeZone}</p><ReadStatus state={board} w={w}/>
 {currentBoard && <><p>{date(currentBoard.from)} — {date(currentBoard.to)} · {w[currentBoard.state]} · {w.measured}: {date(currentBoard.measured_at)}</p><p>{w.cohort}: {currentBoard.cohort_orders} · {w.evidence}: {currentBoard.source_evidence} · {w.matched}: {currentBoard.matched_orders}</p><h4>{w.groups}</h4><p>{w.groupsHelp}</p><p>{w.cancelledHelp}</p><div className="receiving-overview-scroll" tabIndex={0}><table><thead><tr><th>{w.supplier}</th><th>{w.product}</th><th>{w.unit}</th><th>{w.matched}</th><th>{w.pending}</th><th>{w.received}</th><th>{w.cancelled}</th><th>{w.planned}</th><th>{w.accepted}</th><th>{w.remaining}</th><th>{w.pendingQuantity}</th></tr></thead><tbody>{currentBoard.groups.map(g => <tr key={JSON.stringify([g.supplier_id, g.product_id, g.unit])}><td>{g.supplier_id}</td><td>{g.product_id}</td><td>{w['unit_' + g.unit]}</td><td>{g.orders}</td><td>{g.pending_orders}</td><td>{g.received_orders}</td><td>{g.cancelled_orders}</td><td>{quantity(g.planned_milli_exact)}</td><td>{quantity(g.accepted_milli_exact)}</td><td>{quantity(g.remaining_milli_exact)}</td><td>{quantity(g.pending_milli_exact)}</td></tr>)}</tbody></table></div>
 <h4>{w.ordersPage}</h4><p>{w.pageHelp}</p>{!currentBoard.items.length && <p>{w.empty}</p>}<div className="receiving-overview-scroll" tabIndex={0}><table><thead><tr><th>{w.order}</th><th>{w.supplier}</th><th>{w.product}</th><th>{w.unit}</th><th>{w.at}</th><th>{w.commercial}</th><th>{w.receiving}</th><th>{w.planned}</th><th>{w.accepted}</th><th>{w.remaining}</th><th>{w.flags}</th><th>{w.inspect}</th></tr></thead><tbody>{currentBoard.items.map(row => <tr key={row.order_id}><td>{row.order_id}</td><td>{row.supplier_id}</td><td>{row.product_id}</td><td>{w['unit_' + row.unit]}</td><td>{date(row.created_at)}</td><td>{w[row.commercial_status]}</td><td>{w[row.receiving_status]}</td><td>{quantity(row.planned_milli)}</td><td>{quantity(row.accepted_milli)}</td><td>{quantity(row.remaining_milli)}</td><td><Flags values={row.review_flags} w={w}/></td><td><button onClick={() => { setSelected(row.order_id); setOverviewOffset(0); }}>{w.inspect}</button></td></tr>)}</tbody></table></div><Pages offset={offset} total={currentBoard.matched_orders} set={setOffset} loading={board.loading} w={w}/><button onClick={() => exportBoard(currentBoard)}>{w.exportBoard}</button><p>{w.exportHelp}</p></>}
 </section>
 <section><h3>{w.overview}{selected ? ' · ' + selected : ''}</h3><p>{w.navigationHelp}</p>{!selected ? <p>{w.chooseOrder}</p> : <><ReadStatus state={detail} w={w}/>{currentDetail && <><p>{w.supplier}: {currentDetail.supplier_id} · {w.product}: {currentDetail.product_id} · {w.unit}: {w['unit_' + currentDetail.unit]}</p><p>{w.commercial}: {w[currentDetail.commercial_status]} · {w.receiving}: {w[currentDetail.receiving_status]}</p><p>{w.measured}: {date(currentDetail.measured_at)}</p><p>{w.physicalHelp}</p><dl>{([[w.planned, currentDetail.planned_milli], [w.delivered, currentDetail.effective_delivered_milli], [w.accepted, currentDetail.effective_accepted_milli], [w.partialRejected, currentDetail.partial_rejected_milli], [w.remaining, currentDetail.remaining_milli], [w.fullyRejected, currentDetail.fully_rejected_milli_exact], [w.voided, currentDetail.voided_accepted_milli_exact]] as const).map(([label, value]) => <div key={label}><dt>{label}</dt><dd>{quantity(value)}</dd></div>)}</dl><p>{w.receiptCount}: {currentDetail.receipt_count} · {w.voidCount}: {currentDetail.voided_receipt_count} · {w.rejectionCount}: {currentDetail.full_rejection_count} · {w.evidence}: {currentDetail.source_records}</p><h4>{w.authorization}</h4>{currentDetail.authorization ? <><p>{w.actor}: {currentDetail.authorization.actor_id} · {w.device}: {currentDetail.authorization.device_id} · {w.at}: {date(currentDetail.authorization.created_at)}</p><p>{w.operation}: {currentDetail.authorization.operation_id} · {w.reference}: {currentDetail.authorization.reference}</p><p>{w.reason}: {currentDetail.authorization.reason}</p></> : <p>{w.noAuthorization}</p>}<h4>{w.flags}</h4><Flags values={currentDetail.review_flags} w={w}/><h4>{w.appointments}</h4><p>{Object.entries(currentDetail.agenda_counts).map(([status, total]) => `${w[status]}: ${total}`).join(' · ')}</p><p>{w.pageHelp}</p>{!currentDetail.appointments.length && <p>{w.empty}</p>}<div className="receiving-overview-scroll" tabIndex={0}><table><thead><tr><th>{w.dock}</th><th>{w.from}</th><th>{w.to}</th><th>{w.status}</th></tr></thead><tbody>{currentDetail.appointments.map(a => <tr key={a.appointment_id}><td>{a.dock_id} · {a.appointment_id}</td><td>{date(a.starts_at)}</td><td>{date(a.ends_at)}</td><td>{w[a.status]}</td></tr>)}</tbody></table></div><Pages offset={overviewOffset} total={currentDetail.total_appointments} loading={detail.loading} set={setOverviewOffset} w={w}/><button onClick={() => exportOverview(currentDetail)}>{w.exportOverview}</button><p>{w.exportHelp}</p></>}</>}
 </section></section>;
}
function Flags({ values, w }: {
    values: string[];
    w: Record<string, string>;
}) { return values.length ? <ul>{values.map(flag => <li key={flag}>{w[flag]}</li>)}</ul> : <p>{w.noneFlags}</p>; }
function ReadStatus({ state, w }: {
    state: {
        loading: boolean;
        error: string;
    };
    w: Record<string, string>;
}) { return <>{state.loading && <p role="status">{w.loading}</p>}{state.error && <p role="alert">{w[state.error] || w.unavailable}</p>}</>; }
function Pages({ offset, total, loading, set, w }: {
    offset: number;
    total: number;
    loading: boolean;
    set: (value: number) => void;
    w: Record<string, string>;
}) { return <nav className="receiving-overview-actions" aria-label={w.page}><button disabled={loading || offset === 0} onClick={() => set(Math.max(0, offset - 50))}>{w.previous}</button><span>{w.page} {offset / 50 + 1} · {total}</span><button disabled={loading || offset + 50 >= total || offset + 50 > 1000000} onClick={() => set(offset + 50)}>{w.next}</button></nav>; }
function errorKey(error: unknown) {
    const e = error as {
        status?: number;
        message?: string;
    };
    if (e.status === 401 || e.status === 403)
        return 'denied';
    if (e.status === 422)
        return 'sourceLimit';
    if (e.status === 409)
        return 'conflict';
    return ['receivingOverviewInvalid', 'dockInvalid'].includes(e.message || '') ? 'invalid' : 'unavailable';
}
