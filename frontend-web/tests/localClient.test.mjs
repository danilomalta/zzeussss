import test from 'node:test';
import assert from 'node:assert/strict';
import { createLocalClient, LocalAPIError, formatLocalCents } from '../src/core/local/localClient.mjs';

const expires = Math.floor(Date.now() / 1000) + 3600;
const session = { tenant_id: 'empresa-teste', store_id: 'loja-teste', identity_id: 'operador-teste', device_id: 'aparelho-teste', expires_unix: expires };
const json = (body, status = 200) => new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });

test('local login uses identity_id, then confirms context with its opaque token', async () => {
  const calls = [];
  const client = createLocalClient(async (url, options) => {
    calls.push({ url, options });
    return url.endsWith('/login') ? json({ token: 'token-de-teste', expires_unix: expires }) : json(session);
  });
  const result = await client.login('operador-teste', 'senha-ficticia-de-teste');
  assert.deepEqual(result.session, session);
  assert.deepEqual(JSON.parse(calls[0].options.body), { identity_id: 'operador-teste', password: 'senha-ficticia-de-teste' });
  assert.equal(calls[0].url, '/local/v1/login');
  assert.equal(calls[1].url, '/local/v1/me');
  assert.equal(calls[1].options.headers.Authorization, 'Bearer token-de-teste');
  assert.equal(calls[0].options.credentials, 'omit');
});
test('failed login does not request products or session; errors contain no submitted password', async () => {
  let calls = 0;
  const client = createLocalClient(async () => { calls++; return json({}, 401); });
  await assert.rejects(client.login('operador-teste', 'senha-ficticia'), (error) => error instanceof LocalAPIError && error.status === 401 && !error.message.includes('senha-ficticia'));
  assert.equal(calls, 1);
});
test('foreign identity in me is rejected and new server session is revoked best effort', async () => {
  const paths = [];
  const client = createLocalClient(async (url) => {
    paths.push(url);
    if (url.endsWith('/login')) return json({ token: 'token-de-teste', expires_unix: expires });
    if (url.endsWith('/me')) return json({ ...session, identity_id: 'outra-identidade' });
    return new Response(null, { status: 204 });
  });
  await assert.rejects(client.login('operador-teste', 'senha-ficticia'), LocalAPIError);
  assert.deepEqual(paths, ['/local/v1/login', '/local/v1/me', '/local/v1/logout']);
});
test('catalog reads actual response, preserves empty state and paginates', async () => {
  const row = { id: 'p1', sku: 'SKU', name: 'Produto de teste', unit: 'unit', price_cents: 899 };
  const client = createLocalClient(async (url, options) => {
    assert.equal(options.headers.Authorization, 'Bearer token-de-teste');
    return json({ items: url.endsWith('offset=50') ? null : [row] });
  });
  assert.deepEqual(await client.products('token-de-teste'), [row]);
  assert.deepEqual(await client.products('token-de-teste', 50), []);
});
test('catalog rejects unauthorized, unavailable, HTML and unsafe money instead of inventing rows', async () => {
  for (const status of [401, 403, 503]) {
    const client = createLocalClient(async () => json({}, status));
    await assert.rejects(client.products('token-de-teste'), (error) => error.status === status);
  }
  const html = createLocalClient(async () => new Response('<html>index</html>'));
  await assert.rejects(html.products('token-de-teste'), LocalAPIError);
  const unsafe = createLocalClient(async () => json({ items: [{ id: 'p1', sku: 'S', name: 'P', unit: 'unit', price_cents: 9007199254740992 }] }));
  await assert.rejects(unsafe.products('token-de-teste'), LocalAPIError);
});
test('logout calls the server and reports unavailable transport', async () => {
  const client = createLocalClient(async (url, options) => {
    assert.equal(url, '/local/v1/logout'); assert.equal(options.method, 'POST');
    return new Response(null, { status: 204 });
  });
  await client.logout('token-de-teste');
  const offline = createLocalClient(async () => { throw new Error('internal transport details'); });
  await assert.rejects(offline.logout('token-de-teste'), (error) => error.status === 0 && !error.message.includes('internal transport details'));
});
test('cent formatting remains exact at the JavaScript safe integer boundary', () => {
  assert.equal(formatLocalCents(0), 'R$ 0,00');
  assert.equal(formatLocalCents(899), 'R$ 8,99');
  assert.equal(formatLocalCents(Number.MAX_SAFE_INTEGER), 'R$ 90.071.992.547.409,91');
  assert.throws(() => formatLocalCents(1.5), LocalAPIError);
});
