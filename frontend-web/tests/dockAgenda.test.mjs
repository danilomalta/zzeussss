import test from 'node:test';
import assert from 'node:assert/strict';
import { createDockAgenda } from '../src/core/dockAgenda/source.mjs';
import { dockResource, dockAppointment, dockDetail, dockResult, dockHistory, dockTime, dockWindow, dockUTC, dockCSV, dockCommand } from '../src/core/dockAgenda/model.mjs';
import { dockWords } from '../src/core/dockAgenda/words.mjs';
const at = '2026-10-23T09:00:00Z', window = { from: '2026-10-23T00:00:00Z', to: '2026-10-24T00:00:00Z' }, resource = () => ({ dock_id: 'DOCK', name: 'Doca descartável', enabled: true, revision: 1, created_at: at, updated_at: at }), appointment = () => ({ appointment_id: 'APPOINTMENT', dock_id: 'DOCK', order_id: 'ORDER', supplier_id: 'SUP', starts_at: '2026-10-23T10:00:00Z', ends_at: '2026-10-23T11:00:00Z', status: 'reserved', revision: 1, created_by: 'U', created_at: at, updated_at: at, arrived_at: null, unloading_at: null, completed_at: null, supplier_confirmed: false, stock_received: false });
const response = (v, status = 200) => new Response(JSON.stringify(v), { status });
function fixture(hook) {
    const session = { tenant_id: 'T', store_id: 'S', identity_id: 'U', device_id: 'D', expires_unix: Math.floor(Date.now() / 1000) + 3600 }, capabilities = { ...session, role: 'manager', permissions: ['manage_stock', 'view_orders'], license: { state: 'active', modules: ['core', 'inventory', 'orders', 'logistics'] } }, state = { current: true, posts: 0 }, saved = new Map(), records = new Map(), events = new Map(), db = { dock: resource(), appointment: appointment() }, calls = [], storage = { getItem: k => saved.get(k) ?? null, setItem: (k, v) => saved.set(k, v), removeItem: k => saved.delete(k) }, locks = { request: async (k, o, fn) => fn({ name: k }) };
    let sequence = 0;
    const baseResult = (kind, id, action, snapshot, op) => ({ operation_id: op, entity_kind: kind, entity_id: id, action, revision: snapshot.revision, occurred_at: snapshot.updated_at, repeated: false, [kind === 'dock' ? 'dock' : 'appointment']: structuredClone(snapshot) });
    const record = (kind, id, action, body, snapshot) => { const result = baseResult(kind, id, action, snapshot, body.operation_id), key = kind + '/' + id; records.set(body.operation_id, result); if (!events.has(key))
        events.set(key, []); events.get(key).push({ result, actor_id: 'U', device_id: 'D', request_json: JSON.stringify({ kind, id, action, input: body }) }); return result; };
    record('dock', 'DOCK', 'save_dock', { operation_id: 'BASE_DOCK', dock_id: 'DOCK', expected_revision: 0, name: db.dock.name, enabled: true }, db.dock);
    record('appointment', 'APPOINTMENT', 'reserve', { operation_id: 'BASE_BOOK', appointment_id: 'APPOINTMENT', dock_id: 'DOCK', order_id: 'ORDER', starts_at: db.appointment.starts_at, ends_at: db.appointment.ends_at, reason: 'Planejado' }, db.appointment);
    const detail = () => { const a = db.appointment, measured = '2026-10-23T12:00:00Z', delta = (a, b) => Math.trunc((Date.parse(b) - Date.parse(a)) / 1000); return { appointment: structuredClone(a), measured_at: measured, order_cancelled: false, arrival_delay_seconds: a.arrived_at ? delta(a.starts_at, a.arrived_at) : null, waiting_seconds: a.arrived_at ? delta(a.arrived_at, a.unloading_at || (a.status === 'cancelled' ? a.updated_at : measured)) : null, unloading_seconds: a.unloading_at ? delta(a.unloading_at, a.completed_at || measured) : null }; };
    const fetcher = async (url, init) => {
        calls.push({ url, init });
        const replacement = await hook?.({ url, init, state, db, saved, records, events, session, capabilities });
        if (replacement !== undefined)
            return replacement;
        const u = new URL(url, 'http://discardable.invalid'), path = u.pathname.split('/local/v1/logistics/')[1], offset = Number(u.searchParams.get('offset') || 0), limit = Number(u.searchParams.get('limit') || 50), page = items => ({ items: items.slice(offset, offset + limit), total: items.length, offset, limit });
        if (init.method === 'GET') {
            if (path.startsWith('operations/'))
                return records.has(path.slice(11)) ? response(records.get(path.slice(11))) : response({}, 404);
            if (path.endsWith('/history')) {
                const [base, id] = path.split('/'), key = (base === 'docks' ? 'dock/' : 'appointment/') + decodeURIComponent(id);
                return events.has(key) ? response(page(events.get(key))) : response({}, 404);
            }
            if (path === 'docks')
                return response(page([db.dock]));
            if (path === 'occupancy')
                return response(page([{ dock: db.dock, unloading: db.appointment.status === 'unloading' ? db.appointment : null, waiting_count: db.appointment.status === 'arrived' ? 1 : 0, measured_at: '2026-10-23T12:00:00Z' }]));
            if (path === 'appointments')
                return response(page([db.appointment].filter(a => (!u.searchParams.get('status') || u.searchParams.get('status') === a.status) && (!u.searchParams.get('dock_id') || u.searchParams.get('dock_id') === a.dock_id))));
            if (path.startsWith('appointments/'))
                return response(detail());
            throw Error('unexpected GET');
        }
        state.posts++;
        const body = JSON.parse(init.body);
        if (records.has(body.operation_id))
            return response({ ...records.get(body.operation_id), repeated: true });
        const now = new Date(Date.parse(at) + (++sequence) * 60000).toISOString().replace('.000Z', 'Z');
        let result;
        if (path === 'docks') {
            db.dock = { dock_id: body.dock_id, name: body.name, enabled: body.enabled, revision: body.expected_revision + 1, created_at: body.expected_revision ? db.dock.created_at : now, updated_at: now };
            result = record('dock', db.dock.dock_id, 'save_dock', body, db.dock);
        }
        else if (path === 'appointments') {
            db.appointment = { ...appointment(), appointment_id: body.appointment_id, order_id: body.order_id, dock_id: body.dock_id, starts_at: body.starts_at, ends_at: body.ends_at, created_at: now, updated_at: now };
            result = record('appointment', body.appointment_id, 'reserve', body, db.appointment);
        }
        else {
            const a = structuredClone(db.appointment);
            a.revision++;
            a.updated_at = now;
            if (body.action === 'reschedule') {
                a.dock_id = body.dock_id;
                a.starts_at = body.starts_at;
                a.ends_at = body.ends_at;
            }
            else {
                a.status = { arrive: 'arrived', start: 'unloading', complete: 'completed', cancel: 'cancelled', no_show: 'no_show' }[body.action];
                const timestamp = { arrive: 'arrived_at', start: 'unloading_at', complete: 'completed_at' }[body.action];
                if (timestamp)
                    a[timestamp] = now;
            }
            db.appointment = a;
            result = record('appointment', a.appointment_id, body.action, body, a);
        }
        return response(result);
    };
    const options = { token: 'discardable-only', session, capabilities, isCurrent: () => state.current, storage, locks, fetcher }, source = createDockAgenda(options), dock = (overrides = {}) => source.prepare('dock', 'NEW_DOCK', null, { operation_id: 'NEW_DOCK_OP', dock_id: 'NEW_DOCK', name: 'Novo recurso', enabled: true, expected_revision: 0, ...overrides }), reserve = () => source.prepare('reserve', 'NEW_BOOK', null, { operation_id: 'NEW_BOOK_OP', appointment_id: 'NEW_BOOK', dock_id: 'DOCK', order_id: 'ORDER', starts_at: '2026-10-23T13:00:00Z', ends_at: '2026-10-23T14:00:00Z', reason: 'Planejamento local' }), act = (action = 'arrive', extra = {}) => source.prepare('action', db.appointment.appointment_id, db.appointment, { operation_id: 'ACTION_' + action, expected_revision: db.appointment.revision, action, reason: 'Conferência descartável', ...extra });
    return { source, options, session, capabilities, state, saved, storage, locks, calls, db, records, events, dock, reserve, act, detail };
}
test('authorized historical reads use real logistics endpoints and independent occupancy', async () => { const f = fixture(); assert.equal((await f.source.docks()).items[0].dock_id, 'DOCK'); assert.equal((await f.source.appointments(window)).items.length, 1); assert.equal((await f.source.occupancy()).items[0].unloading, null); assert.equal((await f.source.detail('APPOINTMENT')).appointment.status, 'reserved'); assert.equal((await f.source.history('appointment', 'APPOINTMENT')).items.length, 1); assert.equal(f.state.posts, 0); });
test('save resource verifies original receipt and full history request before clearing', async () => { const f = fixture(), result = await f.source.submit(f.dock()); assert.equal(result.state, 'confirmed'); assert.equal(result.receipt.entity_id, 'NEW_DOCK'); assert.equal(f.source.pending(), null); assert.equal(f.state.posts, 1); assert(f.calls.some(c => c.url.includes('limit=1'))); });
test('resource revision and disabled flag are preserved without forcing replacement', async () => { const f = fixture(), c = f.source.prepare('dock', 'DOCK', f.db.dock, { operation_id: 'EDIT', dock_id: 'DOCK', name: 'Doca revisada', enabled: false, expected_revision: 1 }); const r = await f.source.submit(c); assert.equal(r.receipt.dock.revision, 2); assert.equal(r.receipt.dock.enabled, false); });
test('booking is local reservation only, not supplier acknowledgement, stock or payment', async () => { const f = fixture(), r = await f.source.submit(f.reserve()); assert.equal(r.receipt.appointment.supplier_confirmed, false); assert.equal(r.receipt.appointment.stock_received, false); assert.equal(r.receipt.appointment.order_id, 'ORDER'); });
test('arrival, unloading and completion retain original appointment and server times', async () => { const f = fixture(); for (const action of ['arrive', 'start', 'complete']) {
    const r = await f.source.submit(f.act(action));
    assert.equal(r.state, 'confirmed');
    assert.equal(r.receipt.entity_id, 'APPOINTMENT');
} assert.equal(f.db.appointment.status, 'completed'); assert.equal(f.db.appointment.revision, 4); assert.equal(f.state.posts, 3); assert.equal((await f.source.history('appointment', 'APPOINTMENT')).items.length, 4); assert.equal((await f.source.detail('APPOINTMENT')).unloading_seconds, 60); });
test('reschedule preserves order and supplier while changing only planned dock/window', async () => { const f = fixture(), r = await f.source.submit(f.act('reschedule', { dock_id: 'OTHER_DOCK', starts_at: '2026-10-23T16:00:00Z', ends_at: '2026-10-23T17:00:00Z' })); assert.equal(r.receipt.appointment.status, 'reserved'); assert.equal(r.receipt.appointment.dock_id, 'OTHER_DOCK'); assert.equal(r.receipt.appointment.order_id, 'ORDER'); });
for (const action of ['cancel', 'no_show'])
    test(action + ' keeps history and never fabricates actual unloading', async () => { const f = fixture(), r = await f.source.submit(f.act(action)); assert.equal(r.receipt.appointment.status, action === 'cancel' ? 'cancelled' : 'no_show'); assert.equal(r.receipt.appointment.unloading_at, null); });
test('lost response leaves uncertain operation recoverable by original receipt without POST', async () => { let lost = false; const f = fixture(({ init, url }) => { if (lost && url.includes('/operations/'))
    return; }); const real = f.options.fetcher; f.options.fetcher = async (...args) => { const r = await real(...args); if (args[1].method === 'POST' && !lost) {
    lost = true;
    throw Error('lost response');
} return r; }; const source = createDockAgenda(f.options), c = f.dock(); await assert.rejects(source.submit(c), /lost response/); assert.equal(source.pending().uncertain, true); const r = await source.recover(); assert.equal(r.receipt.operation_id, c.body.operation_id); assert.equal(f.state.posts, 1); assert.equal(source.pending(), null); });
test('receipt absence alone never triggers automatic resend or uncertain discard', async () => { const f = fixture(({ init }) => { if (init.method === 'POST')
    throw Error('transport'); }); await assert.rejects(f.source.submit(f.dock())); assert.equal(f.source.pending().uncertain, true); assert.equal((await f.source.recover()).state, 'missing'); await assert.rejects(f.source.recover(false, true), /dockUncertain/); assert.equal(f.source.pending().uncertain, true); });
test('explicit retry reuses exact operation ID and payload', async () => { let first = true; const f = fixture(({ init }) => { if (init.method === 'POST' && first) {
    first = false;
    throw Error('transport');
} }), c = f.reserve(); await assert.rejects(f.source.submit(c)); const r = await f.source.recover(true); assert.equal(r.receipt.operation_id, c.body.operation_id); const bodies = f.calls.filter(c => c.init.method === 'POST').map(c => c.init.body); assert.equal(bodies.length, 2); assert.equal(bodies[0], bodies[1]); });
for (const field of ['tenant_id', 'store_id', 'identity_id', 'device_id'])
    test('old receipt lookup cannot send after ' + field + ' changes', async () => { const f = fixture(({ url, session, capabilities }) => { if (url.includes('/operations/')) {
        session[field] = 'OTHER';
        capabilities[field] = 'OTHER';
    } }); await assert.rejects(f.source.submit(f.dock()), e => e.status === 401); assert.equal(f.state.posts, 0); assert.equal(f.saved.size, 1); });
test('logout during response body blocks sending and preserves pending operation', async () => { const f = fixture(({ url, state }) => url.includes('/operations/') ? { ok: false, status: 404, text: async () => { state.current = false; return '{}'; } } : undefined); await assert.rejects(f.source.submit(f.dock()), e => e.status === 401); assert.equal(f.state.posts, 0); assert.equal(f.saved.size, 1); });
test('session changes after POST cannot downgrade or erase uncertainty', async () => { const f = fixture(({ init, state }) => { if (init.method === 'POST') {
    state.current = false;
    return response({}, 403);
} }); await assert.rejects(f.source.submit(f.dock())); assert.equal([...f.saved.values()].map(JSON.parse)[0].uncertain, true); });
for (const change of [f => f.capabilities.permissions = [], f => f.capabilities.license.state = 'expired', f => f.capabilities.license.modules = ['core', 'orders', 'inventory']])
    test('late revocation blocks writes ' + String(change), async () => { let f; f = fixture(({ url }) => { if (url.includes('/operations/'))
        change(f); }); await assert.rejects(f.source.submit(f.reserve())); assert.equal(f.state.posts, 0); });
test('worker may read resources but not create or edit a resource', async () => { const f = fixture(); f.capabilities.role = 'employee'; await f.source.docks(); assert.throws(() => f.dock(), e => e.status === 403); assert.equal(f.state.posts, 0); });
test('expired license and unavailable locks allow authorized historical reads', async () => { const f = fixture(); f.capabilities.license.state = 'expired'; const source = createDockAgenda({ ...f.options, locks: undefined }); await source.docks(); await source.history('appointment', 'APPOINTMENT'); assert.equal(source.canWrite, false); assert.equal(source.canLock, false); });
test('storage failure prevents all network calls; malformed pending is preserved', async () => { const f = fixture(); f.storage.setItem = () => { }; await assert.rejects(f.source.submit(f.dock()), /dockStorage/); assert.equal(f.calls.length, 0); f.saved.set('titan-dock-agenda:v1:' + f.source.scope, 'invalid'); await assert.rejects(f.source.submit(f.dock()), /dockStorage/); assert.equal(f.saved.size, 1); });
test('existing queue or held cross-tab lock prevents a second operation', async () => { const f = fixture(); f.saved.set('titan-dock-agenda:v1:' + f.source.scope, JSON.stringify(f.dock())); await assert.rejects(f.source.submit(f.reserve()), /dockPending/); const g = fixture(); g.locks.request = async (k, o, fn) => fn(null); await assert.rejects(g.source.submit(g.dock()), /dockLocked/); assert.equal(g.state.posts, 0); });
test('stale appointment state/revision is compared before POST', async () => { const f = fixture(), c = f.act(); f.db.appointment.revision = 2; await assert.rejects(f.source.submit(c), e => e.status === 409); assert.equal(f.state.posts, 0); assert.equal(f.source.pending().uncertain, false); });
test('explicit slot conflict remains visible and cannot be force-retried automatically', async () => { const f = fixture(({ init }) => init.method === 'POST' ? response({ code: 'agenda_slot_occupied' }, 409) : undefined); await assert.rejects(f.source.submit(f.reserve()), e => e.code === 'agenda_slot_occupied'); assert.equal(f.source.pending().uncertain, false); assert.equal((await f.source.recover()).state, 'missing'); assert.equal((await f.source.recover(false, true)).state, 'discarded'); });
test('old uncertain attempt is not downgraded by later explicit rejection', async () => { let n = 0; const f = fixture(({ init }) => { if (init.method === 'POST') {
    if (++n === 1)
        throw Error('transport');
    return response({}, 409);
} }); await assert.rejects(f.source.submit(f.reserve())); await assert.rejects(f.source.recover(true)); assert.equal(f.source.pending().uncertain, true); });
test('missing history for an existing original receipt is not mistaken for an absent operation', async () => { const f = fixture(({ url }) => url.includes('/history') ? response({}, 404) : undefined), c = f.dock(); await assert.rejects(f.source.submit(c), e => e.status === 404); assert.equal(f.state.posts, 1); await assert.rejects(f.source.recover(true), e => e.status === 404); assert.equal(f.state.posts, 1); assert.equal(f.source.pending().uncertain, true); });
test('different request JSON or original actor/device cannot confirm the receipt', async () => { for (const field of ['request_json', 'actor_id', 'device_id']) {
    const f = fixture(({ url, events }) => { if (url.includes('NEW_DOCK/history')) {
        const e = events.get('dock/NEW_DOCK')[0];
        e[field] = field === 'request_json' ? JSON.stringify({ kind: 'dock', id: 'NEW_DOCK', action: 'save_dock', input: { operation_id: 'NEW_DOCK_OP', dock_id: 'NEW_DOCK', name: 'Outra', enabled: true, expected_revision: 0 } }) : 'OTHER';
    } });
    await assert.rejects(f.source.submit(f.dock()));
    assert.equal(f.source.pending().uncertain, true);
} });
test('later current state does not replace the original historical operation result', async () => { const f = fixture(); await f.source.submit(f.act('arrive')); const original = structuredClone(f.records.get('ACTION_arrive')); await f.source.submit(f.act('start')); const command = f.source.prepare('action', 'APPOINTMENT', appointment(), { operation_id: 'ACTION_arrive', expected_revision: 1, action: 'arrive', reason: 'Conferência descartável' }); f.saved.set('titan-dock-agenda:v1:' + f.source.scope, JSON.stringify({ ...command, uncertain: true })); const r = await f.source.recover(); assert.deepEqual(r.receipt, original); assert.equal(f.state.posts, 2); });
test('another context cannot see or delete original pending data', () => { const f = fixture(), c = f.reserve(); f.saved.set('titan-dock-agenda:v1:' + f.source.scope, JSON.stringify(c)); const session = { ...f.session, store_id: 'OTHER' }, capabilities = { ...f.capabilities, store_id: 'OTHER' }, other = createDockAgenda({ ...f.options, session, capabilities }); assert.equal(other.pending(), null); assert.equal(f.saved.size, 1); });
test('date/window validation rejects invalid calendars, fractions and unsafe ranges', () => { for (const value of ['2026-02-30T10:00:00Z', '2026-10-23T24:00:00Z', '2026-10-23', '1999-12-31T10:00:00Z'])
    assert.throws(() => dockTime(value)); assert.equal(dockUTC('2026-10-23T10:00:00-03:00'), '2026-10-23T13:00:00Z'); assert.throws(() => dockWindow('2026-10-23T10:00:00Z', '2026-10-23T10:04:59Z')); assert.throws(() => dockWindow('2026-10-23T10:00:00.1Z', '2026-10-23T11:00:00Z')); assert.throws(() => dockWindow('2026-10-01T00:00:00Z', '2026-11-02T00:00:00Z', true)); assert.equal(dockTime('2026-10-23T10:00:00.123456789Z') - dockTime('2026-10-23T10:00:00Z'), 123456789n); });
test('UTF8 limits, unrelated action fields, illegal transitions and corrupt scope are rejected', () => { const f = fixture(); assert.throws(() => f.dock({ name: 'á'.repeat(61) })); assert.throws(() => f.act('start')); assert.throws(() => f.act('arrive', { starts_at: '2026-10-23T10:00:00Z' })); assert.throws(() => dockCommand({ ...f.reserve(), scope: 'FOREIGN' }, f.source.scope)); });
test('inconsistent schema/state/metrics never become operational success', () => { assert.throws(() => dockResource({ ...resource(), revision: 0 })); assert.throws(() => dockAppointment({ ...appointment(), supplier_confirmed: true })); assert.throws(() => dockAppointment({ ...appointment(), status: 'completed' })); const f = fixture(); assert.throws(() => dockDetail({ ...f.detail(), waiting_seconds: 1 }, 'APPOINTMENT')); assert.throws(() => dockResult({ ...f.records.get('BASE_BOOK'), entity_id: 'OTHER' })); });
test('CSV is explicitly one validated page and protects formula prefixes', () => { const a = { ...appointment(), order_id: '=1+1' }, csv = dockCSV({ items: [a], total: 1, offset: 0, limit: 50 }, window); assert(csv.includes("'=1+1")); assert(csv.includes('query_from')); });
test('five dictionaries have identical populated keys and translated actions/statuses', () => { const base = Object.keys(dockWords('pt-BR')).sort(); for (const locale of ['en', 'es', 'fr', 'zh-CN']) {
    const words = dockWords(locale);
    assert.deepEqual(Object.keys(words).sort(), base);
    for (const value of Object.values(words))
        assert(value.trim());
} });
for (const field of ['operation_id', 'entity_id', 'revision', 'repeated'])
    test('incompatible original receipt ' + field + ' cannot confirm or erase the queue', async () => { const f = fixture(({ url, records }) => { if (url.includes('operations/NEW_DOCK_OP') && records.has('NEW_DOCK_OP')) {
        const result = structuredClone(records.get('NEW_DOCK_OP'));
        result[field] = field === 'revision' ? 9 : field === 'repeated' ? true : 'OTHER';
        return response(result);
    } }); await assert.rejects(f.source.submit(f.dock())); assert.equal(f.source.pending().uncertain, true); assert.equal(f.state.posts, 1); });
test('confirmed operation with storage removal failure remains recoverable without another POST', async () => { const f = fixture(); f.storage.removeItem = () => { }; await assert.rejects(f.source.submit(f.dock()), /dockStorage/); assert.equal(f.state.posts, 1); assert.equal(f.source.pending().uncertain, true); f.storage.removeItem = k => f.saved.delete(k); assert.equal((await f.source.recover()).state, 'confirmed'); assert.equal(f.state.posts, 1); });
test('expired contract can recover an original confirmed operation with same IDs but cannot resend', async () => { let first = true; const f = fixture(), send = f.options.fetcher; const source = createDockAgenda({ ...f.options, fetcher: async (...args) => { const r = await send(...args); if (args[1].method === 'POST' && first) {
        first = false;
        throw Error('lost');
    } return r; } }); await assert.rejects(source.submit(f.dock())); f.capabilities.license.state = 'expired'; const result = await source.recover(); assert.equal(result.receipt.operation_id, 'NEW_DOCK_OP'); assert.equal(f.state.posts, 1); });
test('actual occupancy retains a late unloading from a prior planned day', async () => { const f = fixture(); f.db.appointment = { ...appointment(), starts_at: '2026-10-22T08:00:00Z', ends_at: '2026-10-22T09:00:00Z', created_at: '2026-10-22T07:00:00Z', arrived_at: '2026-10-22T08:00:00Z', unloading_at: '2026-10-22T08:10:00Z', updated_at: '2026-10-22T08:10:00Z', status: 'unloading', revision: 3 }; const page = await f.source.occupancy(); assert.equal(page.items[0].unloading.status, 'unloading'); assert.equal(page.items[0].unloading.ends_at, '2026-10-22T09:00:00Z'); assert.equal(f.state.posts, 0); });
test('cancelled arrival freezes waiting at cancellation and absent timestamps remain unknown', () => { const a = { ...appointment(), status: 'cancelled', arrived_at: '2026-10-23T09:05:00Z', updated_at: '2026-10-23T09:10:00Z', revision: 3 }; const v = { appointment: a, measured_at: '2026-10-23T12:00:00Z', order_cancelled: true, arrival_delay_seconds: -3300, waiting_seconds: 300, unloading_seconds: null }; assert.equal(dockDetail(v, 'APPOINTMENT').waiting_seconds, 300); assert.throws(() => dockDetail({ ...v, waiting_seconds: 10500 }, 'APPOINTMENT')); });
test('valid history accepts another authorized actor but still checks event revision continuity', async () => { const f = fixture(); f.events.get('dock/DOCK')[0].actor_id = 'OTHER'; assert.equal((await f.source.history('dock', 'DOCK')).items[0].actor_id, 'OTHER'); f.events.get('dock/DOCK')[0].result.revision = 3; await assert.rejects(f.source.history('dock', 'DOCK')); });
test('appointment query validates filter names, page offset and maximum window before network', async () => { const f = fixture(); for (const filters of [null, { ...window, typo: 'silent' }, { ...window, status: 'unknown' }, { ...window, dock_id: null }, { ...window, to: '2026-12-23T00:00:00Z' }])
    await assert.rejects(f.source.appointments(filters)); await assert.rejects(f.source.appointments(window, 1000001)); assert.equal(f.calls.length, 0); });
test('history rejects another resource ID hidden inside an otherwise matching envelope', async () => { const f = fixture(); const row = f.events.get('dock/DOCK')[0], request = JSON.parse(row.request_json); request.input.dock_id = 'OTHER'; row.request_json = JSON.stringify(request); await assert.rejects(f.source.history('dock', 'DOCK')); });
test('appointment ordering follows server UTF8 binary IDs for equal timestamps', async () => { const rows = ['\uE000', '😀'].map((id, i) => ({ ...appointment(), appointment_id: id, order_id: 'ORDER' + i })), f = fixture(({ url }) => new URL(url, 'http://preview.invalid').pathname.endsWith('/appointments') ? response({ items: rows, total: 2, limit: 50, offset: 0 }) : undefined); assert.equal((await f.source.appointments(window)).items[1].appointment_id, '😀'); });

test('two appointments on one dock are distinct records on the same page',async()=>{const rows=[appointment(),{...appointment(),appointment_id:'SECOND',order_id:'SECOND_ORDER',starts_at:'2026-10-23T11:00:00Z',ends_at:'2026-10-23T12:00:00Z'}],f=fixture(({url})=>new URL(url,'http://preview.invalid').pathname.endsWith('/appointments')?response({items:rows,total:2,limit:50,offset:0}):undefined);assert.equal((await f.source.appointments(window)).items.length,2);rows[1].appointment_id=rows[0].appointment_id;await assert.rejects(f.source.appointments(window),/dockInvalid/);});
