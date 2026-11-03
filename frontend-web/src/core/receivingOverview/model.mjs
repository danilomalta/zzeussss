import { schemas } from './schemas.mjs';
import { dockTime, dockUTC, dockCompareID, dockAppointment, dockStatuses } from '../dockAgenda/model.mjs';
const utf8 = s => new TextEncoder().encode(s).length;
export const readFail = () => { throw Error('receivingOverviewInvalid'); };
function schema(s, v) {
    if (v === null && s.nullable)
        return;
    if (s.$ref)
        return schema(schemas[s.$ref.split('/').pop()], v);
    if (s.allOf) {
        for (const part of s.allOf)
            schema(part, v);
        return;
    }
    if (s.enum && !s.enum.includes(v))
        readFail();
    if (s.type === 'object') {
        if (!v || typeof v !== 'object' || Array.isArray(v) || (s.required || []).some(k => !Object.hasOwn(v, k)) || s.additionalProperties === false && Object.keys(v).some(k => !Object.hasOwn(s.properties, k)))
            readFail();
        for (const [k, value] of Object.entries(v))
            if (s.properties[k])
                schema(s.properties[k], value);
    }
    else if (s.type === 'array') {
        if (!Array.isArray(v))
            readFail();
        for (const item of v)
            schema(s.items, item);
    }
    else if (s.type === 'integer') {
        if (!Number.isSafeInteger(v) || s.minimum !== undefined && v < s.minimum || s.maximum !== undefined && v > s.maximum)
            readFail();
    }
    else if (s.type === 'boolean') {
        if (typeof v !== 'boolean')
            readFail();
    }
    else if (s.type === 'string') {
        if (typeof v !== 'string' || s.minLength !== undefined && v.length < s.minLength || s.maxLength !== undefined && utf8(v) > s.maxLength)
            readFail();
        if (s.format === 'date-time')
            dockTime(v);
    }
}
const MAX = BigInt(Number.MAX_SAFE_INTEGER), units = ['unit', 'kg', 'g', 'liter', 'ml', 'meter'];
export const receivingReadID = s => typeof s === 'string' && !!s && s === s.trim() && utf8(s) <= 128 && !/[\x00\r\n]/.test(s);
export const receivingFlags = ['order_cancelled_no_new_receiving', 'local_receiving_authorization_missing', 'accepted_quantity_pending', 'partial_receiving', 'rejections_recorded_review_agreement', 'receipt_void_recorded_not_refund', 'completed_dock_with_receiving_pending_review', 'cancelled_order_with_active_agenda_review', 'no_local_dock_appointment'];
const count = (value, min = 0, max = Number.MAX_SAFE_INTEGER) => {
    if (!Number.isSafeInteger(value) || value < min || value > max)
        readFail();
    return value;
};
const exact = (value, max = MAX * 1000n) => {
    if (typeof value !== 'string' || value.length > 32 || !/^(0|[1-9]\d*)$/.test(value) || BigInt(value) > max)
        readFail();
    return BigInt(value);
};
function shape(name, v) {
    if (!schemas[name])
        readFail();
    schema(schemas[name], v);
    return v;
}
function flags(values) {
    if (!Array.isArray(values) || values.some(v => !receivingFlags.includes(v)) || new Set(values).size !== values.length)
        readFail();
}
function quantities(row) {
    if (!['order_id', 'supplier_id', 'product_id'].every(k => receivingReadID(row[k])) || !units.includes(row.unit) || !['local_not_sent', 'cancelled'].includes(row.commercial_status) || !['not_authorized', 'authorized', 'partially_received', 'received'].includes(row.receiving_status))
        readFail();
    count(row.planned_milli, 1);
    count(row.accepted_milli);
    count(row.remaining_milli);
    if (BigInt(row.accepted_milli) + BigInt(row.remaining_milli) !== BigInt(row.planned_milli) || row.receiving_status === 'received' && row.remaining_milli !== 0 || row.receiving_status === 'partially_received' && !(row.accepted_milli > 0 && row.remaining_milli > 0) || ['not_authorized', 'authorized'].includes(row.receiving_status) && row.accepted_milli !== 0)
        readFail();
    flags(row.review_flags);
    const expected = [row.commercial_status === 'cancelled', row.commercial_status !== 'cancelled' && row.receiving_status === 'not_authorized', row.commercial_status !== 'cancelled' && row.remaining_milli > 0, row.accepted_milli > 0 && row.remaining_milli > 0];
    for (let i = 0; i < 4; i++)
        if (row.review_flags.includes(receivingFlags[i]) !== expected[i])
            readFail();
    if (row.review_flags.includes(receivingFlags[6]) && row.remaining_milli === 0 || row.review_flags.includes(receivingFlags[7]) && row.commercial_status !== 'cancelled' || row.review_flags.includes(receivingFlags[8]) && [receivingFlags[6], receivingFlags[7]].some(k => row.review_flags.includes(k)))
        readFail();
    return row;
}
export function overviewFilters(input) {
    if (!input || typeof input !== 'object' || Object.keys(input).some(k => !['from', 'to', 'supplier_id', 'product_id', 'state'].includes(k)))
        readFail();
    if (['supplier_id', 'product_id', 'state'].some(k => input[k] !== undefined && typeof input[k] !== 'string'))
        readFail();
    const a = dockTime(input.from, true), b = dockTime(input.to, true);
    if (b <= a || b - a > 366n * 86400n * 1000000000n)
        readFail();
    const result = { from: dockUTC(input.from), to: dockUTC(input.to), supplier_id: input.supplier_id ?? '', product_id: input.product_id ?? '', state: input.state ?? 'pending' };
    if (!['pending', 'all', 'received', 'cancelled', 'review'].includes(result.state) || ['supplier_id', 'product_id'].some(k => typeof result[k] !== 'string' || result[k] && !receivingReadID(result[k])))
        readFail();
    return result;
}
function page(v, offset, total) {
    if (v.offset !== offset || v.limit !== 50 || !Number.isInteger(offset) || offset < 0 || offset > 1000000 || v.appointments && v.appointments.length !== Math.min(50, Math.max(0, total - offset)) || v.items && v.items.length !== Math.min(50, Math.max(0, total - offset)))
        readFail();
}
function expectedFlags(v) {
    const result = [];
    if (v.commercial_status === 'cancelled')
        result.push(receivingFlags[0]);
    if (v.authorization === null && v.commercial_status !== 'cancelled')
        result.push(receivingFlags[1]);
    if (v.remaining_milli > 0 && v.commercial_status !== 'cancelled')
        result.push(receivingFlags[2]);
    if (v.effective_accepted_milli > 0 && v.remaining_milli > 0)
        result.push(receivingFlags[3]);
    if (v.partial_rejected_milli > 0 || v.full_rejection_count > 0)
        result.push(receivingFlags[4]);
    if (v.voided_receipt_count > 0)
        result.push(receivingFlags[5]);
    if (v.agenda_counts.completed > 0 && v.remaining_milli > 0)
        result.push(receivingFlags[6]);
    if (v.commercial_status === 'cancelled' && v.agenda_counts.reserved + v.agenda_counts.arrived + v.agenda_counts.unloading > 0)
        result.push(receivingFlags[7]);
    if (v.total_appointments === 0)
        result.push(receivingFlags[8]);
    return result;
}
export function receivingOverview(v, id, offset = 0) {
    shape('ReceivingOverview', v);
    quantities({ ...v, accepted_milli: v.effective_accepted_milli });
    if (v.order_id !== id || v.supplier_confirmation !== 'not_proven_by_local_records' || v.payment_evaluation !== 'not_evaluated')
        readFail();
    dockTime(v.measured_at);
    count(v.source_records, 0, 1000);
    for (const k of ['effective_delivered_milli', 'partial_rejected_milli', 'receipt_count', 'voided_receipt_count', 'full_rejection_count', 'total_appointments'])
        count(v[k]);
    if (BigInt(v.effective_accepted_milli) + BigInt(v.partial_rejected_milli) !== BigInt(v.effective_delivered_milli) || v.voided_receipt_count > v.receipt_count || v.source_records !== v.receipt_count + v.voided_receipt_count + v.full_rejection_count + v.total_appointments)
        readFail();
    if (v.receipt_count === v.voided_receipt_count && (v.effective_delivered_milli !== 0 || v.effective_accepted_milli !== 0 || v.partial_rejected_milli !== 0))
        readFail();
    exact(v.fully_rejected_milli_exact, BigInt(v.full_rejection_count) * MAX);
    exact(v.voided_accepted_milli_exact, BigInt(v.voided_receipt_count) * MAX);
    if (v.authorization) {
        for (const k of ['operation_id', 'device_id', 'actor_id', 'reference'])
            if (!receivingReadID(v.authorization[k]))
                readFail();
        if (!v.authorization.reason || v.authorization.reason !== v.authorization.reason.trim() || utf8(v.authorization.reason) > 255 || /[\x00\r\n]/.test(v.authorization.reason))
            readFail();
        dockTime(v.authorization.created_at);
        if (v.receiving_status === 'not_authorized')
            readFail();
    }
    else if (v.receiving_status !== 'not_authorized')
        readFail();
    let sum = 0;
    for (const status of dockStatuses)
        sum += count(v.agenda_counts[status]);
    if (sum !== v.total_appointments || JSON.stringify(v.review_flags) !== JSON.stringify(expectedFlags(v)))
        readFail();
    page(v, offset, v.total_appointments);
    let previous;
    const seen = new Set();
    for (const a of v.appointments) {
        dockAppointment(a);
        if (a.order_id !== id || a.supplier_id !== v.supplier_id || seen.has(a.appointment_id) || previous && (dockTime(a.starts_at) < dockTime(previous.starts_at) || dockTime(a.starts_at) === dockTime(previous.starts_at) && dockCompareID(a.appointment_id, previous.appointment_id) <= 0))
            readFail();
        seen.add(a.appointment_id);
        previous = a;
    }
    if (offset === 0 && v.appointments.length === v.total_appointments)
        for (const status of dockStatuses)
            if (v.agenda_counts[status] !== v.appointments.filter(a => a.status === status).length)
                readFail();
    return v;
}
const key = v => JSON.stringify([v.supplier_id, v.product_id, v.unit]);
function match(row, state) { return state === 'all' || state === 'cancelled' && row.commercial_status === 'cancelled' || state === 'pending' && row.commercial_status !== 'cancelled' && row.remaining_milli > 0 || state === 'received' && row.commercial_status !== 'cancelled' && row.remaining_milli === 0 || state === 'review' && row.review_flags.length > 0; }
export function receivingBoard(v, input, offset = 0) {
    shape('ReceivingBoard', v);
    const filters = overviewFilters(input);
    if (['from', 'to', 'supplier_id', 'product_id', 'state'].some(k => v[k] !== filters[k]) || v.basis !== 'order_creation_cohort_local_evidence_not_supplier_confirmation_or_payment')
        readFail();
    dockTime(v.measured_at);
    count(v.cohort_orders, 0, 200);
    count(v.source_evidence, 0, Math.min(2000, v.cohort_orders * 1000));
    count(v.matched_orders, 0, v.cohort_orders);
    page(v, offset, v.matched_orders);
    const groups = new Map();
    let groupOrders = 0, previous;
    for (const g of v.groups) {
        if (!receivingReadID(g.supplier_id) || !receivingReadID(g.product_id) || !units.includes(g.unit) || groups.has(key(g)) || filters.supplier_id && g.supplier_id !== filters.supplier_id || filters.product_id && g.product_id !== filters.product_id)
            readFail();
        count(g.orders, 1, 200);
        const pending = count(g.pending_orders, 0, g.orders), received = count(g.received_orders, 0, g.orders), cancelled = count(g.cancelled_orders, 0, g.orders);
        if (pending + received + cancelled !== g.orders)
            readFail();
        const planned = exact(g.planned_milli_exact, BigInt(g.orders) * MAX), accepted = exact(g.accepted_milli_exact, planned), remaining = exact(g.remaining_milli_exact, planned), pendingMilli = exact(g.pending_milli_exact, remaining);
        if (planned < BigInt(g.orders) || accepted + remaining !== planned || pending === 0 && pendingMilli !== 0n || pending > 0 && pendingMilli < BigInt(pending) || cancelled === 0 && remaining !== pendingMilli)
            readFail();
        if (filters.state === 'pending' && pending !== g.orders || filters.state === 'cancelled' && cancelled !== g.orders || filters.state === 'received' && received !== g.orders)
            readFail();
        if (previous) {
            const compare = dockCompareID(g.supplier_id, previous.supplier_id) || dockCompareID(g.product_id, previous.product_id) || dockCompareID(g.unit, previous.unit);
            if (compare <= 0)
                readFail();
        }
        groups.set(key(g), g);
        groupOrders += g.orders;
        previous = g;
    }
    if (groupOrders !== v.matched_orders)
        readFail();
    const seen = new Set(), partial = new Map();
    previous = undefined;
    for (const row of v.items) {
        quantities(row);
        const at = dockTime(row.created_at);
        if (at < dockTime(v.from) || at >= dockTime(v.to) || seen.has(row.order_id) || !match(row, v.state) || filters.supplier_id && row.supplier_id !== filters.supplier_id || filters.product_id && row.product_id !== filters.product_id || previous && (at < dockTime(previous.created_at) || at === dockTime(previous.created_at) && dockCompareID(row.order_id, previous.order_id) <= 0))
            readFail();
        seen.add(row.order_id);
        const k = key(row);
        if (!groups.has(k))
            readFail();
        const sum = partial.get(k) || { orders: 0, pending_orders: 0, received_orders: 0, cancelled_orders: 0, planned: 0n, accepted: 0n, remaining: 0n, pending: 0n };
        sum.orders++;
        sum.planned += BigInt(row.planned_milli);
        sum.accepted += BigInt(row.accepted_milli);
        sum.remaining += BigInt(row.remaining_milli);
        if (row.commercial_status === 'cancelled')
            sum.cancelled_orders++;
        else if (row.remaining_milli === 0)
            sum.received_orders++;
        else {
            sum.pending_orders++;
            sum.pending += BigInt(row.remaining_milli);
        }
        partial.set(k, sum);
        previous = row;
    }
    for (const [k, sum] of partial) {
        const g = groups.get(k), full = offset === 0 && v.items.length === v.matched_orders;
        for (const field of ['orders', 'pending_orders', 'received_orders', 'cancelled_orders'])
            if (full ? sum[field] !== g[field] : sum[field] > g[field])
                readFail();
        for (const [field, metric] of [['planned', 'planned_milli_exact'], ['accepted', 'accepted_milli_exact'], ['remaining', 'remaining_milli_exact'], ['pending', 'pending_milli_exact']])
            if (full ? sum[field] !== BigInt(g[metric]) : sum[field] > BigInt(g[metric]))
                readFail();
    }
    return v;
}
export function receivingQuantity(value, locale = 'pt-BR') { const n = typeof value === 'number' ? BigInt(count(value)) : exact(value), sep = new Intl.NumberFormat(locale).formatToParts(1.1).find(p => p.type === 'decimal')?.value || '.'; return new Intl.NumberFormat(locale).format(n / 1000n) + sep + String(n % 1000n).padStart(3, '0'); }
export function receivingCSV(rows) {
    return '\uFEFF' + rows.map(row => row.map(value => {
        let s = String(value ?? '');
        if (/^[\s]*[=+\-@]/.test(s))
            s = "'" + s;
        return '"' + s.replaceAll('"', '""') + '"';
    }).join(';')).join('\r\n') + '\r\n';
}
