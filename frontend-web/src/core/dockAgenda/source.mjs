import { LocalAPIError } from '../local/localClient.mjs';
import { contextOf, contextKey } from '../context74/model.mjs';
import { dockID, dockFail, dockEqual, dockPage, dockDetail, dockCommand, dockReceipt, dockResult, dockHistory, dockWindow, dockStatuses, dockTime, dockCompareID } from './model.mjs';
export function createDockAgenda({ token, session, capabilities, isCurrent, storage, locks, fetcher = globalThis.fetch }) {
    const scope = contextKey(contextOf(session, capabilities)), key = 'titan-dock-agenda:v1:' + scope;
    const read = () => ['manage_stock', 'view_orders'].every(p => capabilities.permissions?.includes(p));
    const write = () => read() && capabilities.license?.state === 'active' && ['core', 'inventory', 'orders', 'logistics'].every(m => capabilities.license.modules.includes(m));
    const manager = () => ['owner', 'manager'].includes(capabilities.role);
    function authorize(mutating = false, kind) { let valid = false; try {
        valid = typeof token === 'string' && !!token && !/[\r\n]/.test(token) && isCurrent() && contextKey(contextOf(session, capabilities)) === scope;
    }
    catch { } if (!valid)
        throw new LocalAPIError(401, 'dock'); if (!read() || mutating && (!write() || kind === 'dock' && !manager()))
        throw new LocalAPIError(403, 'dock'); }
    authorize();
    const id = v => { if (!dockID(v))
        dockFail(); return encodeURIComponent(v); };
    const pagination = (offset, limit = 50) => { if (!Number.isInteger(offset) || offset < 0 || offset > 1000000 || ![1, 50].includes(limit))
        dockFail(); return new URLSearchParams({ limit: String(limit), offset: String(offset) }); };
    async function request(path, method = 'GET', body, signal, kind) { authorize(method === 'POST', kind); const ctrl = new AbortController(), abort = () => ctrl.abort(); if (signal?.aborted)
        abort(); signal?.addEventListener('abort', abort, { once: true }); const timer = setTimeout(abort, 15000); try {
        ctrl.signal.throwIfAborted();
        const response = await fetcher('/local/v1/logistics/' + path, { method, credentials: 'omit', cache: 'no-store', signal: ctrl.signal, headers: { Authorization: 'Bearer ' + token, ...(body ? { 'Content-Type': 'application/json' } : {}) }, ...(body ? { body: JSON.stringify(body) } : {}) });
        authorize();
        ctrl.signal.throwIfAborted();
        const raw = await response.text();
        authorize();
        ctrl.signal.throwIfAborted();
        if (raw.length > 2000000)
            dockFail();
        let result;
        try {
            result = raw ? JSON.parse(raw) : null;
        }
        catch {
            if (response.ok)
                dockFail();
        }
        if (!response.ok) {
            const error = new LocalAPIError(response.status, 'dock');
            if (['agenda_slot_occupied', 'agenda_order_unavailable', 'agenda_conflict'].includes(result?.code))
                error.code = result.code;
            throw error;
        }
        return result;
    }
    finally {
        clearTimeout(timer);
        signal?.removeEventListener('abort', abort);
    } }
    const history = async (kind, entity, offset = 0, signal, limit = 50) => { if (!['dock', 'appointment'].includes(kind))
        dockFail(); return dockHistory(await request((kind === 'dock' ? 'docks/' : 'appointments/') + id(entity) + '/history?' + pagination(offset, limit), 'GET', undefined, signal), kind, entity, offset, limit); };
    const detail = async (entity, signal) => dockDetail(await request('appointments/' + id(entity), 'GET', undefined, signal), entity);
    function pending() { authorize(); try {
        const raw = storage.getItem(key);
        if (raw === null)
            return null;
        if (raw.length > 20000)
            dockFail();
        return dockCommand(JSON.parse(raw), scope);
    }
    catch {
        throw Error('dockStorage');
    } }
    function save(command) { authorize(); const raw = JSON.stringify(dockCommand(command, scope)); try {
        storage.setItem(key, raw);
        if (storage.getItem(key) !== raw)
            throw Error();
    }
    catch {
        throw Error('dockStorage');
    } }
    function clear(command) { authorize(); if (!dockEqual(pending(), command))
        throw Error('dockStorage'); try {
        storage.removeItem(key);
        if (storage.getItem(key) !== null)
            throw Error();
    }
    catch {
        throw Error('dockStorage');
    } }
    async function locked(fn) { if (!locks?.request)
        throw Error('dockLocks'); return locks.request(key, { ifAvailable: true }, async (lock) => { authorize(); if (!lock)
        throw Error('dockLocked'); return fn(); }); }
    async function receipt(command) { let original; try {
        original = await request('operations/' + id(command.body.operation_id));
    }
    catch (error) {
        if (error instanceof LocalAPIError && error.status === 404)
            error.code = 'dockOriginalMissing';
        throw error;
    } const result = dockReceipt(original, command, session.identity_id), offset = result.revision - 1; if (offset > 1000000)
        throw Error('dockHistoryLimit'); const events = await history(result.entity_kind, result.entity_id, offset, undefined, 1), event = events.items[0]; if (!event || event.actor_id !== session.identity_id || event.device_id !== session.device_id || !dockEqual(event.result, result) || !dockEqual(JSON.parse(event.request_json), { kind: result.entity_kind, id: command.entity_id, action: result.action, input: command.body }))
        dockFail(); return result; }
    async function preflight(command) { authorize(true, command.kind); if (command.kind === 'action') {
        if (!dockEqual((await detail(command.entity_id)).appointment, command.before))
            throw new LocalAPIError(409, 'dock');
    }
    else if (command.kind === 'dock' && command.before) {
        const r = await history('dock', command.entity_id, command.before.revision - 1, undefined, 1);
        if (r.total !== command.before.revision || !dockEqual(r.items[0]?.result.dock, command.before))
            throw new LocalAPIError(409, 'dock');
    } authorize(true, command.kind); }
    async function resolve(send, discard) {
        let command = pending();
        if (!command)
            return null;
        let proof;
        try {
            proof = await receipt(command);
        }
        catch (error) {
            if (!(error instanceof LocalAPIError) || error.status !== 404 || error.code !== 'dockOriginalMissing')
                throw error;
        }
        if (proof) {
            clear(command);
            return { state: 'confirmed', command, receipt: proof };
        }
        if (discard) {
            if (command.uncertain)
                throw Error('dockUncertain');
            clear(command);
            return { state: 'discarded', command };
        }
        if (!send)
            return { state: 'missing', command };
        await preflight(command);
        const wasUncertain = command.uncertain;
        command = { ...command, uncertain: true };
        save(command);
        authorize(true, command.kind);
        const path = command.kind === 'dock' ? 'docks' : command.kind === 'reserve' ? 'appointments' : 'appointments/' + id(command.entity_id) + '/actions';
        try {
            const result = await request(path, 'POST', command.body, undefined, command.kind);
            dockResult(result);
            dockReceipt({ ...result, repeated: false }, command, session.identity_id);
        }
        catch (error) {
            if (!wasUncertain && error instanceof LocalAPIError && [400, 403, 404, 409, 413, 422].includes(error.status)) {
                authorize();
                save({ ...command, uncertain: false });
            }
            throw error;
        }
        proof = await receipt(command);
        clear(command);
        return { state: 'confirmed', command, receipt: proof };
    }
    return { scope, get canWrite() { return !!write(); }, get canManage() { return manager(); }, canLock: !!locks?.request, assertCurrent() { authorize(); }, pending,
        async docks(offset = 0, signal) { return dockPage('DockPage', await request('docks?' + pagination(offset), 'GET', undefined, signal), offset); },
        async occupancy(offset = 0, signal) { return dockPage('OccupancyPage', await request('occupancy?' + pagination(offset), 'GET', undefined, signal), offset); },
        async appointments(filters, offset = 0, signal) { if (!filters || typeof filters !== 'object' || Object.keys(filters).some(k => !['from', 'to', 'dock_id', 'status'].includes(k)) || ['from', 'to'].some(k => typeof filters[k] !== 'string') || ['dock_id', 'status'].some(k => filters[k] !== undefined && typeof filters[k] !== 'string'))
            dockFail(); const window = dockWindow(filters.from, filters.to, true), query = pagination(offset); query.set('from', window.from); query.set('to', window.to); if (filters.dock_id)
            query.set('dock_id', decodeURIComponent(id(filters.dock_id))); if (filters.status) {
            if (!dockStatuses.includes(filters.status))
                dockFail();
            query.set('status', filters.status);
        } const page = dockPage('AppointmentPage', await request('appointments?' + query, 'GET', undefined, signal), offset); let previous; for (const row of page.items) {
            if (dockTime(row.ends_at) <= dockTime(window.from) || dockTime(row.starts_at) >= dockTime(window.to) || filters.dock_id && row.dock_id !== filters.dock_id || filters.status && row.status !== filters.status || previous && (dockTime(row.starts_at) < dockTime(previous.starts_at) || dockTime(row.starts_at) === dockTime(previous.starts_at) && dockCompareID(row.appointment_id, previous.appointment_id) <= 0))
                dockFail();
            previous = row;
        } return page; },
        detail, history,
        prepare(kind, entity, before, body) { authorize(true, kind); return dockCommand(structuredClone({ version: 1, scope, kind, entity_id: entity, before, body, uncertain: false }), scope); },
        async submit(value) { authorize(true, value?.kind); const command = dockCommand(structuredClone(value), scope); if (command.uncertain)
            dockFail(); return locked(async () => { authorize(true, command.kind); if (pending())
            throw Error('dockPending'); save(command); return resolve(true, false); }); },
        async recover(send = false, discard = false) { if (typeof send !== 'boolean' || typeof discard !== 'boolean' || send && discard)
            dockFail(); return locked(() => resolve(send, discard)); }
    };
}
