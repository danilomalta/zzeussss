import { LocalAPIError } from '../local/localClient.mjs';
import { contextOf, contextKey } from '../context74/model.mjs';
import { receivingReadID, overviewFilters, receivingOverview, receivingBoard, readFail } from './model.mjs';
// Historical read adapter: no command, commercial acceptance, payment or stock mutation.
export function createReceivingOverview({ token, session, capabilities, isCurrent, fetcher = globalThis.fetch }) {
    const scope = contextKey(contextOf(session, capabilities)), running = new Map();
    function authorize() {
        let current = false;
        try {
            current = typeof token === 'string' && !!token && !/[\r\n]/.test(token) && isCurrent() && contextKey(contextOf(session, capabilities)) === scope;
        }
        catch { }
        if (!current)
            throw new LocalAPIError(401, 'receiving-overview');
        if (!['view_orders', 'manage_stock'].every(p => capabilities.permissions?.includes(p)))
            throw new LocalAPIError(403, 'receiving-overview');
    }
    authorize();
    const page = offset => {
        if (!Number.isInteger(offset) || offset < 0 || offset > 1000000)
            readFail();
        return new URLSearchParams({ limit: '50', offset: String(offset) });
    };
    async function get(path, channel, signal) {
        authorize();
        running.get(channel)?.abort();
        const ctrl = new AbortController();
        running.set(channel, ctrl);
        const abort = () => ctrl.abort();
        if (signal?.aborted)
            abort();
        signal?.addEventListener('abort', abort, { once: true });
        const timer = setTimeout(abort, 15000);
        try {
            ctrl.signal.throwIfAborted();
            const response = await fetcher('/local/v1/' + path, { method: 'GET', credentials: 'omit', cache: 'no-store', headers: { Authorization: 'Bearer ' + token }, signal: ctrl.signal });
            authorize();
            ctrl.signal.throwIfAborted();
            if (running.get(channel) !== ctrl)
                throw Error('receivingReadSuperseded');
            const raw = await response.text();
            authorize();
            ctrl.signal.throwIfAborted();
            if (running.get(channel) !== ctrl)
                throw Error('receivingReadSuperseded');
            if (raw.length > 2000000)
                readFail();
            if (!response.ok)
                throw new LocalAPIError(response.status, 'receiving-overview');
            try {
                return JSON.parse(raw);
            }
            catch {
                readFail();
            }
        }
        finally {
            clearTimeout(timer);
            signal?.removeEventListener('abort', abort);
            if (running.get(channel) === ctrl)
                running.delete(channel);
        }
    }
    return { scope, assertCurrent() { authorize(); }, dispose() {
            for (const c of running.values())
                c.abort();
            running.clear();
        },
        async overview(order, offset = 0, signal) {
            if (!receivingReadID(order))
                readFail();
            const result = await get('purchase-orders/' + encodeURIComponent(order) + '/receiving-overview?' + page(offset), 'overview', signal);
            return receivingOverview(result, order, offset);
        },
        async board(input, offset = 0, signal) {
            const filters = overviewFilters(input), query = page(offset);
            for (const [key, value] of Object.entries(filters))
                if (value)
                    query.set(key, value);
            const result = await get('management/purchases/receiving-board?' + query, 'board', signal);
            return receivingBoard(result, filters, offset);
        }
    };
}
