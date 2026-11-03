import { schemas } from './schemas.mjs';
export const dockFail = () => { throw Error('dockInvalid'); };
export const dockStatuses = ['reserved', 'arrived', 'unloading', 'completed', 'cancelled', 'no_show'];
export const dockActions = { reserved: ['reschedule', 'arrive', 'cancel', 'no_show'], arrived: ['start', 'cancel'], unloading: ['complete'], completed: [], cancelled: [], no_show: [] };
const utf8 = s => new TextEncoder().encode(s).length;
export const dockText = (s, max) => typeof s === 'string' && s === s.trim() && s.length > 0 && utf8(s) <= max && !/[\x00-\x1f\x7f]/.test(s);
export const dockID = s => dockText(s, 128) && !s.includes(' ');
export function dockCompareID(a, b) { const x = new TextEncoder().encode(a), y = new TextEncoder().encode(b); for (let i = 0; i < Math.min(x.length, y.length); i++)
    if (x[i] !== y[i])
        return x[i] - y[i]; return x.length - y.length; }
export function dockSnapshot(value) { return JSON.stringify(Array.isArray(value) ? value.map(v => JSON.parse(dockSnapshot(v))) : value && typeof value === 'object' ? Object.fromEntries(Object.keys(value).sort().map(k => [k, JSON.parse(dockSnapshot(value[k]))])) : value); }
export const dockEqual = (a, b) => dockSnapshot(a) === dockSnapshot(b);
export function dockTime(value, whole = false) { if (typeof value !== 'string' || !/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d{1,9})?(?:Z|[+-]\d{2}:\d{2})$/.test(value))
    dockFail(); const [year, month, day, hour, minute, second] = value.slice(0, 19).split(/[-T:]/).map(Number), d = new Date(Date.UTC(year, month - 1, day)); if (year < 2000 || year > 2200 || month < 1 || month > 12 || d.getUTCFullYear() !== year || d.getUTCMonth() !== month - 1 || d.getUTCDate() !== day || hour > 23 || minute > 59 || second > 59 || !Number.isFinite(Date.parse(value)) || whole && /\.\d*[1-9]/.test(value))
    dockFail(); return BigInt(Date.parse(value.replace(/\.\d+/, ''))) * 1000000n + BigInt((value.match(/\.(\d+)/)?.[1] || '').padEnd(9, '0')); }
export function dockUTC(value) { dockTime(value, true); return new Date(value).toISOString().replace('.000Z', 'Z'); }
export function dockLocalInput(value) { if (typeof value !== 'string' || !/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}(?::\d{2})?$/.test(value))
    dockFail(); const d = new Date(value); if (!Number.isFinite(d.valueOf()) || [d.getFullYear(), d.getMonth() + 1, d.getDate(), d.getHours(), d.getMinutes()].join(',') !== value.slice(0, 16).split(/[-T:]/).map(Number).join(','))
    dockFail(); return dockUTC(d.toISOString()); }
export function dockWindow(from, to, days = false) { const a = dockTime(from, true), b = dockTime(to, true), delta = b - a; if (delta < (days ? 1n : 300n) * 1000000000n || delta > BigInt(days ? 31 * 86400 : 8 * 3600) * 1000000000n)
    dockFail(); return { from: dockUTC(from), to: dockUTC(to) }; }
function schema(s, v) { if (v === null && s.nullable)
    return; if (s.$ref)
    return schema(schemas[s.$ref.split('/').pop()], v); if (s.allOf) {
    for (const part of s.allOf)
        schema(part, v);
    return;
} if (s.enum && !s.enum.includes(v))
    dockFail(); if (s.type === 'object') {
    if (!v || typeof v !== 'object' || Array.isArray(v) || (s.required || []).some(k => !Object.hasOwn(v, k)) || s.additionalProperties === false && Object.keys(v).some(k => !Object.hasOwn(s.properties, k)))
        dockFail();
    for (const [k, value] of Object.entries(v))
        if (s.properties[k])
            schema(s.properties[k], value);
}
else if (s.type === 'array') {
    if (!Array.isArray(v))
        dockFail();
    for (const item of v)
        schema(s.items, item);
}
else if (s.type === 'integer') {
    if (!Number.isSafeInteger(v) || s.minimum !== undefined && v < s.minimum || s.maximum !== undefined && v > s.maximum)
        dockFail();
}
else if (s.type === 'boolean') {
    if (typeof v !== 'boolean')
        dockFail();
}
else if (s.type === 'string') {
    if (typeof v !== 'string' || s.minLength !== undefined && v.length < s.minLength || s.maxLength !== undefined && utf8(v) > s.maxLength)
        dockFail();
    if (s.format === 'date-time')
        dockTime(v);
} }
export function dockValidate(name, v) { if (!schemas[name])
    dockFail(); schema(schemas[name], v); return v; }
export function dockResource(v) { dockValidate('Dock', v); if (!dockID(v.dock_id) || !dockText(v.name, 120) || v.revision < 1 || dockTime(v.updated_at) < dockTime(v.created_at))
    dockFail(); return v; }
export function dockAppointment(v) { dockValidate('Appointment', v); if (!['appointment_id', 'dock_id', 'order_id', 'supplier_id', 'created_by'].every(k => dockID(v[k])) || v.revision < 1 || !dockStatuses.includes(v.status) || v.supplier_confirmed !== false || v.stock_received !== false)
    dockFail(); dockWindow(v.starts_at, v.ends_at); const created = dockTime(v.created_at), updated = dockTime(v.updated_at); if (updated < created)
    dockFail(); let previous = created; for (const key of ['arrived_at', 'unloading_at', 'completed_at'])
    if (v[key] !== null) {
        const at = dockTime(v[key]);
        if (at < previous || at > updated)
            dockFail();
        previous = at;
    } const flags = [v.arrived_at !== null, v.unloading_at !== null, v.completed_at !== null], wanted = { reserved: [false, false, false], no_show: [false, false, false], arrived: [true, false, false], unloading: [true, true, false], completed: [true, true, true] }; if (v.status === 'cancelled' ? flags[1] || flags[2] : !dockEqual(flags, wanted[v.status]))
    dockFail(); return v; }
export function dockResult(v) { dockValidate('Result', v); if (!dockID(v.operation_id) || !dockID(v.entity_id) || v.revision < 1 || !['dock', 'appointment'].includes(v.entity_kind))
    dockFail(); dockTime(v.occurred_at); if (v.entity_kind === 'dock') {
    dockResource(v.dock);
    if (v.appointment != null || v.action !== 'save_dock' || v.dock.dock_id !== v.entity_id || v.dock.revision !== v.revision || v.dock.updated_at !== v.occurred_at)
        dockFail();
}
else {
    dockAppointment(v.appointment);
    if (v.dock != null || !['reserve', 'reschedule', 'arrive', 'start', 'complete', 'cancel', 'no_show'].includes(v.action) || v.appointment.appointment_id !== v.entity_id || v.appointment.revision !== v.revision || v.appointment.updated_at !== v.occurred_at)
        dockFail();
} return v; }
export function dockPage(name, v, offset, limit = 50) { dockValidate(name, v); if (v.offset !== offset || v.limit !== limit || v.items.length !== Math.min(limit, Math.max(0, v.total - offset)))
    dockFail(); const ids = new Set(); for (const row of v.items) {
    if (name === 'DockPage')
        dockResource(row);
    else if (name === 'AppointmentPage')
        dockAppointment(row);
    else if (name === 'OccupancyPage') {
        dockResource(row.dock);
        dockTime(row.measured_at);
        if (row.waiting_count < 0)
            dockFail();
        if (row.unloading) {
            dockAppointment(row.unloading);
            if (row.unloading.status !== 'unloading' || row.unloading.dock_id !== row.dock.dock_id)
                dockFail();
        }
    }
    const id = name === 'DockPage' ? row.dock_id : name === 'AppointmentPage' ? row.appointment_id : row.dock?.dock_id;
    if (id) {
        if (ids.has(id))
            dockFail();
        ids.add(id);
    }
} return v; }
export function dockDetail(v, id) { dockValidate('Detail', v); const a = dockAppointment(v.appointment), at = dockTime(v.measured_at); if (a.appointment_id !== id)
    dockFail(); const delay = a.arrived_at === null ? null : Number((dockTime(a.arrived_at) - dockTime(a.starts_at)) / 1000000000n), wait = a.arrived_at === null ? null : Number(((a.unloading_at ? dockTime(a.unloading_at) : a.status === 'cancelled' ? dockTime(a.updated_at) : at) - dockTime(a.arrived_at)) / 1000000000n), unloading = a.unloading_at === null ? null : Number(((a.completed_at ? dockTime(a.completed_at) : at) - dockTime(a.unloading_at)) / 1000000000n); if (v.arrival_delay_seconds !== delay || v.waiting_seconds !== wait || v.unloading_seconds !== unloading || wait !== null && wait < 0 || unloading !== null && unloading < 0)
    dockFail(); return v; }
export function dockInput(kind, body) { dockValidate({ dock: 'DockInput', reserve: 'BookInput', action: 'ActionInput' }[kind], body); if (!dockID(body.operation_id))
    dockFail(); for (const key of ['dock_id', 'appointment_id', 'order_id'])
    if (body[key] !== undefined && !dockID(body[key]))
        dockFail(); if (kind === 'dock') {
    if (!dockText(body.name, 120))
        dockFail();
}
else {
    if (!dockText(body.reason, 255))
        dockFail();
    if (kind === 'reserve' || body.action === 'reschedule') {
        dockWindow(body.starts_at, body.ends_at);
        if (body.starts_at !== dockUTC(body.starts_at) || body.ends_at !== dockUTC(body.ends_at) || !dockID(body.dock_id))
            dockFail();
    }
    else if (['dock_id', 'starts_at', 'ends_at'].some(k => Object.hasOwn(body, k)))
        dockFail();
} return body; }
export function dockCommand(v, scope) { const keys = ['version', 'scope', 'kind', 'entity_id', 'before', 'body', 'uncertain']; if (!v || typeof v !== 'object' || Object.keys(v).length !== keys.length || keys.some(k => !Object.hasOwn(v, k)) || v.version !== 1 || v.scope !== scope || typeof v.uncertain !== 'boolean' || !dockID(v.entity_id) || !['dock', 'reserve', 'action'].includes(v.kind))
    dockFail(); dockInput(v.kind, v.body); if (v.kind === 'dock') {
    if (v.body.dock_id !== v.entity_id)
        dockFail();
    if (v.before === null) {
        if (v.body.expected_revision !== 0)
            dockFail();
    }
    else {
        dockResource(v.before);
        if (v.before.dock_id !== v.entity_id || v.before.revision !== v.body.expected_revision)
            dockFail();
    }
}
else if (v.kind === 'reserve') {
    if (v.before !== null || v.body.appointment_id !== v.entity_id)
        dockFail();
}
else {
    dockAppointment(v.before);
    if (v.before.appointment_id !== v.entity_id || v.body.expected_revision !== v.before.revision || !dockActions[v.before.status].includes(v.body.action))
        dockFail();
} return v; }
export function dockReceipt(v, command, actor) { dockResult(v); const { kind, body, before, entity_id } = command, action = kind === 'dock' ? 'save_dock' : kind === 'reserve' ? 'reserve' : body.action, expected = kind === 'reserve' ? 1 : body.expected_revision + 1; if (v.operation_id !== body.operation_id || v.entity_id !== entity_id || v.entity_kind !== (kind === 'dock' ? 'dock' : 'appointment') || v.action !== action || v.revision !== expected || v.repeated !== false)
    dockFail(); if (kind === 'dock') {
    if (v.dock.name !== body.name || v.dock.enabled !== body.enabled || before && v.dock.created_at !== before.created_at)
        dockFail();
}
else {
    const a = v.appointment;
    if (kind === 'reserve') {
        if (a.status !== 'reserved' || a.created_by !== actor || a.order_id !== body.order_id || a.dock_id !== body.dock_id || a.starts_at !== body.starts_at || a.ends_at !== body.ends_at)
            dockFail();
    }
    else {
        const status = { reschedule: 'reserved', arrive: 'arrived', start: 'unloading', complete: 'completed', cancel: 'cancelled', no_show: 'no_show' }[action];
        if (a.status !== status || ['order_id', 'supplier_id', 'created_by', 'created_at'].some(k => a[k] !== before[k]))
            dockFail();
        if (action === 'reschedule') {
            if (['dock_id', 'starts_at', 'ends_at'].some(k => a[k] !== body[k]))
                dockFail();
        }
        else if (['dock_id', 'starts_at', 'ends_at'].some(k => a[k] !== before[k]))
            dockFail();
        for (const key of ['arrived_at', 'unloading_at', 'completed_at'])
            if (before[key] !== null && a[key] !== before[key])
                dockFail();
    }
} return v; }
export function dockHistory(v, kind, id, offset, limit = 50) { dockPage('EventPage', v, offset, limit); for (const [i, event] of v.items.entries()) {
    const r = dockResult(event.result);
    if (r.entity_kind !== kind || r.entity_id !== id || r.revision !== offset + i + 1 || r.repeated || !dockID(event.actor_id) || !dockID(event.device_id))
        dockFail();
    let request;
    try {
        request = JSON.parse(event.request_json);
    }
    catch {
        dockFail();
    }
    if (!request || Object.keys(request).length !== 4 || request.kind !== kind || request.id !== id || request.action !== r.action)
        dockFail();
    dockInput(r.action === 'save_dock' ? 'dock' : r.action === 'reserve' ? 'reserve' : 'action', request.input);
    if (r.action === 'save_dock') {
        if (request.input.dock_id !== id || request.input.name !== r.dock.name || request.input.enabled !== r.dock.enabled)
            dockFail();
    }
    else if (r.action === 'reserve') {
        if (request.input.appointment_id !== id || ['dock_id', 'order_id', 'starts_at', 'ends_at'].some(k => request.input[k] !== r.appointment[k]) || event.actor_id !== r.appointment.created_by)
            dockFail();
    }
    else {
        if (request.input.action !== r.action || r.appointment.status !== ({ reschedule: 'reserved', arrive: 'arrived', start: 'unloading', complete: 'completed', cancel: 'cancelled', no_show: 'no_show' }[r.action]) || r.action === 'reschedule' && ['dock_id', 'starts_at', 'ends_at'].some(k => request.input[k] !== r.appointment[k]))
            dockFail();
    }
    if (request.input.operation_id !== r.operation_id || (r.action === 'reserve' ? r.revision !== 1 : request.input.expected_revision !== r.revision - 1))
        dockFail();
} return v; }
export function dockCSV(page, window) { dockPage('AppointmentPage', page, page.offset); const range = dockWindow(window.from, window.to, true), columns = ['appointment_id', 'dock_id', 'order_id', 'supplier_id', 'starts_at', 'ends_at', 'status', 'revision', 'arrived_at', 'unloading_at', 'completed_at', 'supplier_confirmed', 'stock_received']; const rows = [['query_from', 'query_to', ...columns], ...page.items.map(row => [range.from, range.to, ...columns.map(k => row[k] ?? '')])]; return '\uFEFF' + rows.map(row => row.map(value => { let s = String(value); if (/^[\s]*[=+\-@]/.test(s))
    s = "'" + s; return '"' + s.replaceAll('"', '""') + '"'; }).join(';')).join('\r\n') + '\r\n'; }
