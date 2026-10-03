import test from 'node:test';
import assert from 'node:assert/strict';
import { createLocalClient, LocalAPIError } from '../src/core/local/localClient.mjs';
import { parseMoney, parseQuantity, lineCents, cartTotal, makeSale } from '../src/core/local/posModel.mjs';
import { createPOSOperations } from '../src/core/local/posOperations.mjs';

const scope = { tenant_id: 'tenant', store_id: 'store', device_id: 'device', identity_id: 'actor' };
const json = (body, status = 200) => new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });
const lock = async (_key, action) => action();
function memoryStorage() {
  const values = new Map();
  return { getItem: (key) => values.get(key) ?? null, setItem: (key, value) => values.set(key, value), removeItem: (key) => values.delete(key), values };
}
const input = { operation_id: 'op1', sale_id: 'sale1', cash_session_id: 'cash1',
  items: [{ product_id: 'p1', location_id: 'shelf1', quantity_milli: 1000 }], payments: [{ method: 'cash', amount_cents: 899 }] };
function receipt(sale = input) {
  return { sale_id: sale.sale_id, operation_id: sale.operation_id, cash_session_id: sale.cash_session_id,
    total_cents: sale.payments[0].amount_cents, status: 'committed', fiscal_authorized: false,
    items: sale.items.map((item, index) => ({ ...item, item_id: `item${index}`, unit_price_cents: 899, total_cents: 899 })),
    payments: [{ payment_id: 'pay1', method: 'cash', status: 'confirmed', amount_cents: sale.payments[0].amount_cents }] };
}

test('decimal parsing and weighted rounding remain exact without floating money', () => {
  assert.equal(parseMoney('12,34'), 1234);
  assert.equal(parseMoney('12.3'), 1230);
  assert.equal(parseQuantity('0,125', 'kg'), 125);
  assert.equal(parseQuantity('2', 'unit'), 2000);
  assert.equal(lineCents(199, 500), 100);
  assert.equal(lineCents(Number.MAX_SAFE_INTEGER, 1000), Number.MAX_SAFE_INTEGER);
  for (const value of ['-1', '1e2', 'NaN', '1.234', '1.000,00', 'Infinity', '']) assert.throws(() => parseMoney(value));
  assert.throws(() => parseQuantity('1,5', 'unit'));
  assert.throws(() => parseQuantity('0', 'kg'));
  assert.throws(() => lineCents(Number.MAX_SAFE_INTEGER, 2000));
});
test('sale input uses exact totals but never sends prices, company, actor or cash tender', () => {
  const cart = [{ product: { id: 'p1', price_cents: 199, name: 'Test', unit: 'kg' }, location_id: 'shelf1', quantity_milli: 500 }];
  let id = 0;
  const sale = makeSale(cart, 'cash1', () => `id${++id}`);
  assert.equal(cartTotal(cart), 100);
  assert.deepEqual(sale, { operation_id: 'id1', sale_id: 'id2', cash_session_id: 'cash1',
    items: [{ product_id: 'p1', location_id: 'shelf1', quantity_milli: 500 }], payments: [{ method: 'cash', amount_cents: 100 }] });
  assert.throws(() => makeSale([], 'cash1', () => 'id'));
});
test('client matches Go location JSON and preserves blind current cash', async () => {
  const client = createLocalClient(async (url) => json(url.endsWith('/locations') ?
    { items: [{ ID: 'shelf1', kind: 'shelf', name: 'Shelf' }, { ID: 'r1', kind: 'receiving', name: 'Dock' }] } : { session: { session_id: 'cash1', opened_at: 'now' } }));
  assert.deepEqual(await client.locations('token'), [{ id: 'shelf1', kind: 'shelf', name: 'Shelf' }, { id: 'r1', kind: 'receiving', name: 'Dock' }]);
  assert.deepEqual(await client.currentCash('token'), { session_id: 'cash1', opened_at: 'now' });
  assert.equal(await createLocalClient(async () => json({ session: null })).currentCash('token'), null);
  await assert.rejects(createLocalClient(async () => json({})).currentCash('token'), LocalAPIError);
});
test('client validates requests before any HTTP write', async () => {
  let calls = 0;
  const client = createLocalClient(async () => { calls++; throw new Error(); });
  await assert.rejects(client.openCash('token', { session_id: 'cash1', opening_cents: 0.1 }));
  await assert.rejects(client.closeCash('token', { session_id: 'cash1', operation_id: 'op', declared_cents: -1 }));
  await assert.rejects(client.completeSale('token', { ...input, payments: [{ method: 'card', amount_cents: 899 }] }));
  await assert.rejects(client.completeSale('', input));
  assert.equal(calls, 0);
});
test('client rejects foreign IDs, unsafe money, mismatched closure and fabricated fiscal receipts', async () => {
  const client = createLocalClient(async () => json({ sale_id: 'foreign', total_cents: 899, repeated: false }));
  await assert.rejects(client.completeSale('token', input));
  await assert.rejects(createLocalClient(async () => json({ session_id: 'cash1', repeated: false, expected_cents: 1, difference_cents: 5 }))
    .closeCash('token', { session_id: 'cash1', operation_id: 'op', declared_cents: 1 }));
  for (const changes of [{ fiscal_authorized: true }, { total_cents: 9007199254740992 }, { total_cents: 900 }, { items: [] }, { sale_id: 'foreign' }]) {
    await assert.rejects(createLocalClient(async () => json({ ...receipt(), ...changes })).receipt('token', 'sale1'));
  }
});
test('full HTTP contract opens cash, saves sale, reads actual receipt and closes blindly', async () => {
  const calls = [];
  const client = createLocalClient(async (url, options) => {
    assert.equal(options.headers.Authorization, 'Bearer token');
    calls.push([url, options.method, options.body && JSON.parse(options.body)]);
    if (url.endsWith('/cash/open')) return json({ session_id: 'cash1', repeated: false }, 201);
    if (url.endsWith('/cash/close')) return json({ session_id: 'cash1', expected_cents: 999, difference_cents: -9, repeated: false });
    if (url.endsWith('/sales')) return json({ sale_id: 'sale1', total_cents: 899, repeated: false }, 201);
    if (url.endsWith('/sales/sale1')) return json(receipt());
    throw new Error('unexpected path');
  });
  const storage = memoryStorage();
  const operations = createPOSOperations(client, storage, scope, lock);
  assert.equal((await operations.start('token', 'open', { session_id: 'cash1', opening_cents: 100 })).kind, 'open');
  assert.deepEqual((await operations.start('token', 'sale', input)).result, receipt());
  assert.equal((await operations.start('token', 'close', { session_id: 'cash1', operation_id: 'close1', declared_cents: 990 })).result.difference_cents, -9);
  assert.deepEqual(calls.map((call) => call.slice(0, 2)), [['/local/v1/cash/open', 'POST'], ['/local/v1/sales', 'POST'], ['/local/v1/sales/sale1', 'GET'], ['/local/v1/cash/close', 'POST']]);
  assert.equal(operations.pending(), null);
});
test('lost sale reply survives reload and consults receipt instead of selling again', async () => {
  let posts = 0, saved = false;
  const client = createLocalClient(async (url) => {
    if (url.endsWith('/sales')) { posts++; saved = true; throw new Error('lost reply'); }
    return saved ? json(receipt()) : json({}, 404);
  });
  const storage = memoryStorage();
  const first = createPOSOperations(client, storage, scope, lock);
  await assert.rejects(first.start('token', 'sale', input));
  assert.equal(first.pending().uncertain, true);
  const persisted = [...storage.values.values()].join('');
  assert.ok(!persisted.includes('token') && !persisted.includes('password'));
  const restarted = createPOSOperations(client, storage, scope, lock);
  const result = await restarted.retry('new-session-token');
  assert.equal(result.result.sale_id, 'sale1');
  assert.equal(posts, 1);
  assert.equal(restarted.pending(), null);
});
test('lost request is retried with byte-identical IDs and payload after a missing receipt', async () => {
  const payloads = [];
  let saved = false;
  const client = createLocalClient(async (url, options) => {
    if (url.endsWith('/sales')) {
      payloads.push(options.body);
      if (payloads.length === 1) throw new Error('lost before server');
      saved = true; return json({ sale_id: 'sale1', total_cents: 899, repeated: false });
    }
    return saved ? json(receipt()) : json({}, 404);
  });
  const operations = createPOSOperations(client, memoryStorage(), scope, lock);
  await assert.rejects(operations.start('token', 'sale', input));
  await operations.retry('token');
  assert.deepEqual(payloads, [JSON.stringify(input), JSON.stringify(input)]);
});
test('success followed by receipt failure remains uncertain and cannot be discarded', async () => {
  const client = createLocalClient(async (url) => url.endsWith('/sales') ? json({ sale_id: 'sale1', total_cents: 899, repeated: false }) : json({}, 403));
  const operations = createPOSOperations(client, memoryStorage(), scope, lock);
  await assert.rejects(operations.start('token', 'sale', input));
  assert.equal(operations.pending().uncertain, true);
  assert.equal(operations.canDiscard(), false);
  await assert.rejects(operations.discardRejected('token'));
});
test('definite refusal permits editing only after verifying that no receipt exists', async () => {
  const client = createLocalClient(async (url) => json({}, url.endsWith('/sales') ? 409 : 404));
  const operations = createPOSOperations(client, memoryStorage(), scope, lock);
  await assert.rejects(operations.start('token', 'sale', input));
  assert.equal(operations.canDiscard(), true);
  await operations.discardRejected('token');
  assert.equal(operations.pending(), null);
});
test('later refusal cannot erase a previous uncertain submission', async () => {
  let first = true;
  const client = createLocalClient(async (url) => {
    if (!url.endsWith('/sales')) return json({}, 404);
    if (first) { first = false; throw new Error('network failure'); }
    return json({}, 403);
  });
  const operations = createPOSOperations(client, memoryStorage(), scope, lock);
  await assert.rejects(operations.start('token', 'sale', input));
  await assert.rejects(operations.retry('token'));
  assert.equal(operations.canDiscard(), false);
  await assert.rejects(operations.start('token', 'sale', { ...input, sale_id: 'new' }));
});
test('unknown query failure never triggers another POST', async () => {
  let posts = 0;
  const client = createLocalClient(async (url) => {
    if (url.endsWith('/sales')) { posts++; throw new Error('lost reply'); }
    return json({}, 500);
  });
  const operations = createPOSOperations(client, memoryStorage(), scope, lock);
  await assert.rejects(operations.start('token', 'sale', input));
  await assert.rejects(operations.retry('token'));
  assert.equal(posts, 1);
});
test('foreign receipt or differing items cannot confirm or clear the pending sale', async () => {
  let queries = 0;
  const client = createLocalClient(async (url) => {
    if (url.endsWith('/sales')) throw new Error('lost reply');
    queries++;
    return json({ ...receipt(), operation_id: 'foreign' });
  });
  const operations = createPOSOperations(client, memoryStorage(), scope, lock);
  await assert.rejects(operations.start('token', 'sale', input));
  await assert.rejects(operations.retry('token'));
  assert.equal(queries, 1);
  assert.ok(operations.pending());
});
test('cancelled durable sale is reported and never resubmitted', async () => {
  let posts = 0;
  const client = createLocalClient(async (url) => {
    if (url.endsWith('/sales')) { posts++; throw new Error('lost reply'); }
    return json({ ...receipt(), status: 'cancelled' });
  });
  const operations = createPOSOperations(client, memoryStorage(), scope, lock);
  await assert.rejects(operations.start('token', 'sale', input));
  assert.equal((await operations.retry('token')).result.status, 'cancelled');
  assert.equal(posts, 1);
});
test('opening and closing retry their original identifiers after response loss', async () => {
  for (const [kind, body, response] of [
    ['open', { session_id: 'cash1', opening_cents: 100 }, { session_id: 'cash1', repeated: true }],
    ['close', { session_id: 'cash1', operation_id: 'close1', declared_cents: 0 }, { session_id: 'cash1', repeated: true, expected_cents: 0, difference_cents: 0 }],
  ]) {
    const bodies = [];
    const client = createLocalClient(async (_url, options) => { bodies.push(options.body); if (bodies.length === 1) throw new Error(); return json(response); });
    const operations = createPOSOperations(client, memoryStorage(), scope, lock);
    await assert.rejects(operations.start('token', kind, body));
    await operations.retry('token');
    assert.deepEqual(bodies, [JSON.stringify(body), JSON.stringify(body)]);
  }
});
test('failed browser persistence prevents sending and corrupt pending state blocks mutations', async () => {
  let posts = 0;
  const client = createLocalClient(async () => { posts++; throw new Error(); });
  const broken = { getItem: () => null, setItem: () => { throw new Error(); }, removeItem: () => {} };
  await assert.rejects(createPOSOperations(client, broken, scope, lock).start('token', 'sale', input));
  const corrupted = { ...broken, getItem: () => '{invalid' };
  await assert.rejects(createPOSOperations(client, corrupted, scope, lock).start('token', 'sale', input));
  assert.equal(posts, 0);
});
test('company, store, device and operator have distinct browser pending state', async () => {
  const client = createLocalClient(async () => { throw new Error(); });
  const storage = memoryStorage();
  const operations = createPOSOperations(client, storage, scope, lock);
  await assert.rejects(operations.start('token', 'sale', input));
  for (const field of Object.keys(scope)) assert.equal(createPOSOperations(client, storage, { ...scope, [field]: 'other' }, lock).pending(), null);
});
test('double click and concurrent tabs cannot produce independent pending submissions', async () => {
  let finish;
  let posts = 0, locked = false;
  const tabLock = async (_key, action) => {
    if (locked) throw new Error('other tab');
    locked = true; try { return await action(); } finally { locked = false; }
  };
  const storage = memoryStorage();
  const client = createLocalClient(async (url) => {
    if (!url.endsWith('/sales')) return json(receipt());
    posts++;
    await new Promise((resolve) => { finish = resolve; });
    return json({ sale_id: 'sale1', total_cents: 899, repeated: false });
  });
  const first = createPOSOperations(client, storage, scope, tabLock);
  const second = createPOSOperations(client, storage, scope, tabLock);
  const running = first.start('token', 'sale', input);
  await assert.rejects(first.start('token', 'sale', input));
  await assert.rejects(second.start('token', 'sale', input));
  finish(); await running;
  assert.equal(posts, 1);
});
