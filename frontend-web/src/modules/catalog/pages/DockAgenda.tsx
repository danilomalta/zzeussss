import { DraftLeaveDialog } from './CatalogManagement';
import { useEffect, useMemo, useRef, useState, type ReactNode, type DependencyList } from 'react';
import { useTranslation } from 'react-i18next';
import { useLocalSession } from '../../../core/local/useLocalSession';
import { useLocalAccess } from '../../../core/local/LocalAccess';
import { createDockAgenda, type DockSource, type DockOutcome } from '../../../core/dockAgenda/source.mjs';
import { dockActions, dockStatuses, dockLocalInput, dockCSV, dockTime, type DockCommand } from '../../../core/dockAgenda/model.mjs';
import type { Dock, Appointment, Detail, EventPage, ActionInput } from '../../../core/dockAgenda/contracts';
import type { PurchaseOrder } from '../../../core/local/purchaseClient.mjs';
import { dockWords } from '../../../core/dockAgenda/words.mjs';
import './dockAgenda.css';
type Props = {
    order: PurchaseOrder | null;
    onDirty: (dirty: boolean) => void;
    externalPending: boolean;
};
const storage = { getItem: (k: string) => localStorage.getItem(k), setItem: (k: string, v: string) => localStorage.setItem(k, v), removeItem: (k: string) => localStorage.removeItem(k) } as Storage;
export default function DockAgenda(props: Props) {
    const token = useLocalSession(s => s.token), session = useLocalSession(s => s.session), { capabilities } = useLocalAccess(), { i18n } = useTranslation(), locale = i18n.resolvedLanguage || i18n.language, w = dockWords(locale), current = useRef(capabilities);
    current.current = capabilities;
    const source = useMemo(() => { try {
        return token && session && capabilities ? createDockAgenda({ token, session, capabilities, isCurrent: () => { const now = useLocalSession.getState(); return now.token === token && now.session === session && current.current === capabilities; }, storage, locks: navigator.locks }) : null;
    }
    catch {
        return null;
    } }, [token, session, capabilities]);
    return source ? <DockWorkspace key={source.scope + token + JSON.stringify(capabilities)} {...props} source={source} locale={locale}/> : <section className="brand-panel dock-agenda"><h2>{w.title}</h2><p>{w.permission}</p></section>;
}
function useDockRead<T>(read: (signal: AbortSignal) => Promise<T>, deps: DependencyList) {
    const [value, setValue] = useState<T>(), [error, setError] = useState(''), [loading, setLoading] = useState(false);
    useEffect(() => { let live = true; const c = new AbortController(); setValue(undefined); setError(''); setLoading(true); void read(c.signal).then(result => { if (live)
        setValue(result); }).catch(e => { if (live)
        setError(errorKey(e)); }).finally(() => { if (live)
        setLoading(false); }); return () => { live = false; c.abort(); }; }, deps);
    return { value, error, loading };
}
type Form = {
    kind: 'dock' | 'reserve' | 'action';
    id: string;
    before: Dock | Appointment | null;
    name: string;
    enabled: boolean;
    dockID: string;
    from: string;
    to: string;
    reason: string;
    action: string;
    order: PurchaseOrder | null;
};
const blank = (): Form => ({ kind: 'dock', id: crypto.randomUUID(), before: null, name: '', enabled: true, dockID: '', from: '', to: '', reason: '', action: '', order: null });
const localInput = (value: string) => { const d = new Date(value), n = (v: number) => String(v).padStart(2, '0'); return `${d.getFullYear()}-${n(d.getMonth() + 1)}-${n(d.getDate())}T${n(d.getHours())}:${n(d.getMinutes())}:${n(d.getSeconds())}`; };
const today = () => { const d = new Date(); d.setHours(0, 0, 0, 0); return localInput(d.toISOString()); };
function DockWorkspace({ source, locale, order, onDirty, externalPending }: Props & {
    source: DockSource;
    locale: string;
}) {
    const w = dockWords(locale), [refresh, setRefresh] = useState(0), [dockOffset, setDockOffset] = useState(0), [agendaOffset, setAgendaOffset] = useState(0), [occupancyOffset, setOccupancyOffset] = useState(0), [selected, setSelected] = useState(''), [history, setHistory] = useState<{
        kind: 'dock' | 'appointment';
        id: string;
    } | null>(null), [form, setForm] = useState<Form | null>(null), [leaving, setLeaving] = useState<{
        next: Form | null;
    } | null>(null), [review, setReview] = useState<DockCommand | null>(null), [busy, setBusy] = useState(false), [pending, setPending] = useState<DockCommand | null>(null), [storageError, setStorageError] = useState(false), [message, setMessage] = useState(''), [error, setError] = useState(''), [receipt, setReceipt] = useState<DockOutcome>();
    const [draft, setDraft] = useState({ from: today(), to: localInput(new Date(Date.now() + 86400000).toISOString()), dock_id: '', status: '' }), [filters, setFilters] = useState<{
        from: string;
        to: string;
        dock_id: string;
        status: string;
    }>();
    const live = useRef(true), latch = useRef(false), dirty = !!form || !!review || busy || !!pending;
    const docks = useDockRead(s => source.docks(dockOffset, s), [source, dockOffset, refresh]);
    const occupancy = useDockRead(s => source.occupancy(occupancyOffset, s), [source, occupancyOffset, refresh]);
    const agenda = useDockRead(s => filters ? source.appointments(filters, agendaOffset, s) : Promise.resolve(undefined), [source, filters, agendaOffset, refresh]);
    const detail = useDockRead(s => selected ? source.detail(selected, s) : Promise.resolve(undefined), [source, selected, refresh]);
    const readPending = () => { try {
        setPending(source.pending());
        setStorageError(false);
    }
    catch {
        setStorageError(true);
    } };
    useEffect(() => { live.current = true; readPending(); return () => { live.current = false; onDirty(false); }; }, [source]);
    useEffect(() => { onDirty(dirty); }, [dirty, onDirty]);
    useEffect(() => { if (!dirty)
        return; const warn = (e: BeforeUnloadEvent) => { e.preventDefault(); e.returnValue = ''; }; window.addEventListener('beforeunload', warn); return () => window.removeEventListener('beforeunload', warn); }, [dirty]);
    const locked = busy || !!pending || storageError || externalPending || !source.canWrite || !source.canLock;
    const set = (values: Partial<Form>) => setForm(previous => previous ? { ...previous, ...values } : null);
    const start = (next: Form) => { if (locked)
        return; if (form) {
        setLeaving({ next });
        return;
    } setForm(next); setError(''); setMessage(''); };
    function prepare() {
        if (!form)
            return;
        try {
            const operation_id = crypto.randomUUID();
            let command: DockCommand;
            if (form.kind === 'dock')
                command = source.prepare('dock', form.id, form.before, { operation_id, dock_id: form.id, name: form.name.trim(), enabled: form.enabled, expected_revision: form.before?.revision || 0 });
            else if (form.kind === 'reserve') {
                if (!form.order)
                    throw Error('dockInvalid');
                command = source.prepare('reserve', form.id, null, { operation_id, appointment_id: form.id, dock_id: form.dockID, order_id: form.order.id, starts_at: dockLocalInput(form.from), ends_at: dockLocalInput(form.to), reason: form.reason.trim() });
            }
            else
                command = source.prepare('action', form.id, form.before, { operation_id, expected_revision: form.before!.revision, action: form.action as ActionInput['action'], reason: form.reason.trim(), ...(form.action === 'reschedule' ? { dock_id: form.dockID, starts_at: dockLocalInput(form.from), ends_at: dockLocalInput(form.to) } : {}) });
            setReview(command);
            setError('');
        }
        catch (e) {
            setError(errorKey(e));
        }
    }
    async function run(fn: () => Promise<DockOutcome | null>) { if (latch.current)
        return; latch.current = true; setBusy(true); setError(''); setMessage(''); try {
        const result = await fn();
        source.assertCurrent();
        if (!live.current)
            return;
        if (result) {
            setMessage(result.state);
            if (result.state === 'confirmed') {
                setReceipt(result);
                setReview(null);
                setForm(null);
                setRefresh(n => n + 1);
                if (result.receipt?.appointment)
                    setSelected(result.receipt.entity_id);
            }
        }
    }
    catch (e) {
        if (live.current)
            setError(errorKey(e));
    }
    finally {
        latch.current = false;
        if (live.current) {
            readPending();
            setBusy(false);
        }
    } }
    function query() { try {
        setFilters({ from: dockLocalInput(draft.from), to: dockLocalInput(draft.to), dock_id: draft.dock_id, status: draft.status });
        setAgendaOffset(0);
        setError('');
    }
    catch (e) {
        setError(errorKey(e));
    } }
    function exportPage() { try {
        source.assertCurrent();
        if (!agenda.value || !filters)
            return;
        const url = URL.createObjectURL(new Blob([dockCSV(agenda.value, filters)], { type: 'text/csv;charset=utf-8' })), link = document.createElement('a');
        link.href = url;
        link.download = 'Titan_Docas_Pagina.csv';
        link.click();
        setTimeout(() => URL.revokeObjectURL(url), 1000);
    }
    catch (e) {
        setError(errorKey(e));
    } }
    const date = (v: string | null) => v ? new Intl.DateTimeFormat(locale, { dateStyle: 'short', timeStyle: 'medium' }).format(new Date(v)) : w.unknown;
    return <><DraftLeaveDialog open={!!leaving} locale={locale} onCancel={() => setLeaving(null)} onLeave={() => { setForm(leaving?.next || null); setLeaving(null); setError(''); setMessage(''); }}/><section className="brand-panel dock-agenda" aria-label={w.title}><header><h2>{w.title}</h2><button className="brand-secondary" disabled={busy} onClick={() => setRefresh(n => n + 1)}>{w.refresh}</button></header><p>{w.help}</p>{!source.canManage && <p>{w.resourceRole}</p>}{!source.canWrite && <p>{w.readOnly}</p>}{!source.canLock && <p role="alert">{w.locks}</p>}{storageError && <p role="alert">{w.storage}</p>}{error && !review && <p role="alert">{w[error] || w.unavailable}</p>}{message && <p role="status">{w[message]}</p>}
 {pending && <section aria-label={w.pending}><h3>{w.pending}</h3><p>{w.operation}: {pending.body.operation_id} · {pending.entity_id}</p>{pending.uncertain && <p>{w.uncertain}</p>}<div className="dock-actions"><button disabled={busy || !source.canLock} onClick={() => void run(() => source.recover())}>{w.lookup}</button><button disabled={busy || !source.canLock || !source.canWrite || pending.kind === 'dock' && !source.canManage || externalPending} onClick={() => void run(() => source.recover(true))}>{w.retry}</button><button disabled={busy || !source.canLock || pending.uncertain} onClick={() => void run(() => source.recover(false, true))}>{w.discard}</button></div></section>}
 <section><h3>{w.resource}</h3><ReadState state={docks} w={w}/><button className="brand-secondary" disabled={locked || !source.canManage} onClick={() => start(blank())}>{w.newDock}</button>{docks.value && <><div className="dock-scroll" tabIndex={0}><table><thead><tr><th>{w.dock}</th><th>{w.name}</th><th>{w.enabled}</th><th>{w.revision}</th><th>{w.details}</th></tr></thead><tbody>{docks.value.items.map(row => <tr key={row.dock_id}><td>{row.dock_id}</td><td>{row.name}</td><td><span>{row.enabled ? w.yes : w.no}</span></td><td>{row.revision}</td><td><button disabled={locked || !source.canManage} onClick={() => start({ ...blank(), id: row.dock_id, before: row, name: row.name, enabled: row.enabled })}>{w.editDock}</button><button onClick={() => setHistory({ kind: 'dock', id: row.dock_id })}>{w.history}</button></td></tr>)}</tbody></table></div><Pages offset={dockOffset} total={docks.value.total} loading={docks.loading} set={setDockOffset} w={w}/></>}
 <p>{order ? `${w.order}: ${order.id} · ${w.supplier}: ${order.supplier_name}` : w.chooseOrder}</p><button className="brand-primary" disabled={locked || !order || order.status === 'cancelled'} onClick={() => start({ ...blank(), kind: 'reserve', order, from: localInput(new Date().toISOString()), to: localInput(new Date(Date.now() + 3600000).toISOString()) })}>{w.reserve}</button></section>
 {form && <section className="dock-form" aria-label={w.review}><h3>{form.kind === 'dock' ? w.editDock : form.kind === 'reserve' ? w.reserve : w[form.action]}</h3><button disabled={busy || !!review} onClick={() => setLeaving({ next: null })}>{w.close}</button><form onSubmit={e => { e.preventDefault(); prepare(); }}>
 {form.kind === 'dock' ? <><label>{w.name}<input required maxLength={120} disabled={locked || !!review} value={form.name} onChange={e => set({ name: e.target.value })}/></label><label className="dock-check"><input type="checkbox" disabled={locked || !!review} checked={form.enabled} onChange={e => set({ enabled: e.target.checked })}/>{w.enabled}</label></> : <><p>{w.order}: {form.order?.id || (form.before as Appointment)?.order_id}</p>{(form.kind === 'reserve' || form.action === 'reschedule') && <><p>{w.windowHelp} · {Intl.DateTimeFormat().resolvedOptions().timeZone}</p><label>{w.dock}<select required disabled={locked || !!review} value={form.dockID} onChange={e => set({ dockID: e.target.value })}><option value="">{w.choose}</option>{docks.value?.items.filter(row => row.enabled).map(row => <option key={row.dock_id} value={row.dock_id}>{row.name} · {row.dock_id}</option>)}</select></label><p>{w.pageHelp}</p><label>{w.from}<input required type="datetime-local" step="1" disabled={locked || !!review} value={form.from} onChange={e => set({ from: e.target.value })}/></label><label>{w.to}<input required type="datetime-local" step="1" disabled={locked || !!review} value={form.to} onChange={e => set({ to: e.target.value })}/></label></>}<label>{w.reason}<textarea required maxLength={255} disabled={locked || !!review} value={form.reason} onChange={e => set({ reason: e.target.value })}/></label></>}
 <button className="brand-primary" disabled={locked || !!review}>{w.review}</button></form></section>}
 <section><h3>{w.appointments}</h3><p>{w.windowHelp} · {Intl.DateTimeFormat().resolvedOptions().timeZone}</p><form className="dock-filters" onSubmit={e => { e.preventDefault(); query(); }}><label>{w.from}<input required type="datetime-local" step="1" value={draft.from} onChange={e => setDraft({ ...draft, from: e.target.value })}/></label><label>{w.to}<input required type="datetime-local" step="1" value={draft.to} onChange={e => setDraft({ ...draft, to: e.target.value })}/></label><label>{w.dock}<select value={draft.dock_id} onChange={e => setDraft({ ...draft, dock_id: e.target.value })}><option value="">{w.all}</option>{docks.value?.items.map(row => <option key={row.dock_id} value={row.dock_id}>{row.name}</option>)}</select></label><label>{w.status}<select value={draft.status} onChange={e => setDraft({ ...draft, status: e.target.value })}><option value="">{w.all}</option>{dockStatuses.map(s => <option key={s} value={s}>{w[s]}</option>)}</select></label><button>{w.query}</button></form><ReadState state={agenda} w={w}/>{agenda.value && <><p>{w.pageHelp}</p>{filters && <p>{date(filters.from)} — {date(filters.to)} · {filters.dock_id || w.all} · {w[filters.status] || w.all}</p>}<button onClick={exportPage}>{w.export}</button><div className="dock-scroll" tabIndex={0}><table><thead><tr><th>{w.dock}</th><th>{w.order}</th><th>{w.from}</th><th>{w.to}</th><th>{w.status}</th><th>{w.details}</th></tr></thead><tbody>{agenda.value.items.map(row => <tr key={row.appointment_id}><td>{row.dock_id}</td><td>{row.order_id}</td><td>{date(row.starts_at)}</td><td>{date(row.ends_at)}</td><td>{w[row.status]}</td><td><button onClick={() => setSelected(row.appointment_id)}>{w.details}</button></td></tr>)}</tbody></table></div><Pages offset={agendaOffset} total={agenda.value.total} loading={agenda.loading} set={setAgendaOffset} w={w}/></>}
 </section>
 {selected && <section><h3>{w.details} · {selected}</h3><ReadState state={detail} w={w}/>{detail.value && <><AppointmentDetails value={detail.value} w={w} date={date}/>{detail.value.appointment.status === 'reserved' && <p>{w.noShowHelp}</p>}<div className="dock-actions">{dockActions[detail.value.appointment.status].map(action => <button key={action} title={action === 'no_show' ? w.noShowHelp : undefined} disabled={locked || !!form || action === 'no_show' && dockTime(detail.value!.measured_at) < dockTime(detail.value!.appointment.ends_at) || detail.value!.order_cancelled && ['arrive', 'start', 'reschedule'].includes(action)} onClick={() => { const a = detail.value!.appointment; start({ ...blank(), kind: 'action', id: a.appointment_id, before: a, action, dockID: a.dock_id, from: localInput(a.starts_at), to: localInput(a.ends_at) }); }}>{w[action]}</button>)}<button onClick={() => setHistory({ kind: 'appointment', id: selected })}>{w.history}</button></div></>}</section>}
 <section><h3>{w.occupancy}</h3><p>{w.occupancyHelp}</p><ReadState state={occupancy} w={w}/>{occupancy.value && <><ul>{occupancy.value.items.map(row => <li key={row.dock.dock_id}><strong>{row.dock.name}</strong> · {row.unloading ? `${w.unloading}: ${row.unloading.order_id}` : w.free} · {w.waiting}: {row.waiting_count} · {w.measured}: {date(row.measured_at)}{row.unloading && <button onClick={() => setSelected(row.unloading!.appointment_id)}>{w.details}</button>}</li>)}</ul><Pages offset={occupancyOffset} total={occupancy.value.total} loading={occupancy.loading} set={setOccupancyOffset} w={w}/></>}</section>
 {receipt?.receipt && <section aria-label={w.receipt}><h3>{w.receipt}</h3><p>{w.operation}: {receipt.receipt.operation_id} · {w.revision}: {receipt.receipt.revision} · {w.at}: {date(receipt.receipt.occurred_at)}</p><p>{w[receipt.receipt.action]} · {receipt.receipt.entity_id}</p></section>}
 {history && <DockHistory key={history.kind + history.id} source={source} kind={history.kind} id={history.id} locale={locale} refresh={refresh} close={() => setHistory(null)}/>}
 <DockDialog open={!!review} close={() => { if (!busy)
        setReview(null); }} label={w.review}>{review && <><h3>{w.review}</h3><p>{w.reviewHelp}</p><p><strong>{review.kind === 'dock' ? w.save_dock : review.kind === 'reserve' ? w.reserve : w[(review.body as ActionInput).action]}</strong></p><p>{w.operation}: {review.body.operation_id} · {review.entity_id}</p>{'starts_at' in review.body && <p>UTC · {review.body.starts_at} — {review.body.ends_at}</p>}<p>{w.revision}: {'expected_revision' in review.body ? review.body.expected_revision : 0} → {'expected_revision' in review.body ? review.body.expected_revision + 1 : 1}</p>{form && <dl>{(form.kind === 'dock' ? [[w.name, form.name], [w.enabled, form.enabled ? w.yes : w.no]] : [[w.order, form.order?.id || (form.before as Appointment)?.order_id], [w.dock, form.dockID], [w.from, form.from], [w.to, form.to], [w.reason, form.reason]]).map(([label, value]) => <div key={label}><dt>{label}</dt><dd>{value}</dd></div>)}</dl>}{error && <p role="alert">{w[error] || w.unavailable}</p>}<div className="dock-actions"><button autoFocus disabled={busy} onClick={() => setReview(null)}>{w.back}</button><button disabled={locked} onClick={() => void run(() => source.submit(review))}>{busy ? w.loading : w.confirm}</button></div></>}</DockDialog>
 </section></>;
}
function ReadState({ state, w }: {
    state: {
        loading: boolean;
        error: string;
        value?: {
            items?: unknown[];
        } | Detail;
    };
    w: Record<string, string>;
}) { return <>{state.loading && <p role="status">{w.loading}</p>}{state.error && <p role="alert">{w[state.error] || w.unavailable}</p>}{state.value && 'items' in state.value && !state.value.items?.length && <p>{w.empty}</p>}</>; }
function Pages({ offset, total, loading, set, w }: {
    offset: number;
    total: number;
    loading: boolean;
    set: (offset: number) => void;
    w: Record<string, string>;
}) { return <nav className="dock-actions" aria-label={w.page}><button disabled={loading || offset === 0} onClick={() => set(Math.max(0, offset - 50))}>{w.previous}</button><span>{w.page} {offset / 50 + 1} · {total}</span><button disabled={loading || offset + 50 >= total || offset + 50 > 1000000} onClick={() => set(offset + 50)}>{w.next}</button></nav>; }
function AppointmentDetails({ value, w, date }: {
    value: Detail;
    w: Record<string, string>;
    date: (at: string | null) => string;
}) { const a = value.appointment; return <><p>{w.status}: {w[a.status]} · {w.revision}: {a.revision}</p><p>{w.order}: {a.order_id} · {w.supplier}: {a.supplier_id} · {w.dock}: {a.dock_id}</p><p>{date(a.starts_at)} — {date(a.ends_at)}</p>{value.order_cancelled && <p role="alert">{w.orderCancelled}</p>}<dl>{([[w.arrivalDelay, value.arrival_delay_seconds], [w.waitingTime, value.waiting_seconds], [w.unloadingTime, value.unloading_seconds]] as const).map(([label, v]) => <div key={label}><dt>{label}</dt><dd>{v === null ? w.unknown : v}</dd></div>)}</dl><p>{w.measured}: {date(value.measured_at)}</p></>; }
function DockDialog({ open, label, close, children }: {
    open: boolean;
    label: string;
    close: () => void;
    children: ReactNode;
}) { const dialog = useRef<HTMLDialogElement>(null); useEffect(() => { if (!open)
    return; const active = document.activeElement as HTMLElement; dialog.current?.showModal(); return () => { dialog.current?.close(); active?.focus(); }; }, [open]); return <dialog ref={dialog} className="dock-dialog" aria-label={label} onCancel={e => { e.preventDefault(); close(); }}>{children}</dialog>; }
function DockHistory({ source, kind, id, locale, refresh, close }: {
    source: DockSource;
    kind: 'dock' | 'appointment';
    id: string;
    locale: string;
    refresh: number;
    close: () => void;
}) { const w = dockWords(locale), [offset, setOffset] = useState(0), read = useDockRead<EventPage>(s => source.history(kind, id, offset, s), [source, kind, id, offset, refresh]); return <DockDialog open label={w.history} close={close}><header><h3>{w.history} · {id}</h3><button autoFocus onClick={close}>{w.back}</button></header><p>{w.historyHelp}</p><ReadState state={read} w={w}/>{read.value && <><ol>{read.value.items.map(event => { const r = event.result, request = JSON.parse(event.request_json); return <li key={r.revision}><p>{w[r.action]} · {w.revision}: {r.revision} · {w.at}: {new Intl.DateTimeFormat(locale, { dateStyle: 'short', timeStyle: 'medium' }).format(new Date(r.occurred_at))}</p><p>{w.actor}: {event.actor_id} · {w.device}: {event.device_id}</p><p>{w.operation}: {r.operation_id}</p>{request.input.reason && <p>{w.reason}: {request.input.reason}</p>}</li>; })}</ol><Pages offset={offset} total={read.value.total} loading={read.loading} set={setOffset} w={w}/></>}</DockDialog>; }
function errorKey(error: unknown) { const e = error as {
    status?: number;
    code?: string;
    message?: string;
}; if (e.code === 'agenda_slot_occupied')
    return 'slot'; if (e.code === 'agenda_order_unavailable')
    return 'orderUnavailable'; if (e.status === 401 || e.status === 403)
    return 'denied'; if (e.status === 409)
    return 'conflict'; return ({ dockInvalid: 'invalid', dockStorage: 'storage', dockLocks: 'locks', dockLocked: 'locks', dockPending: 'locks', dockUncertain: 'uncertain', dockHistoryLimit: 'historyLimit' } as Record<string, string>)[e.message || ''] || 'unavailable'; }
