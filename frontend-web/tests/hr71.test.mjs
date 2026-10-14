import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import {
  canView,
  sourceAccess,
  visibleAreas,
  areaState,
  filterPeople,
  initials,
  csv,
  currency,
} from '../src/core/hr71/policy.mjs';
const caps = (role = 'owner', permissions = ['manage_staff'], modules = ['staff']) => ({
  role,
  permissions,
  license: { state: 'active', modules },
});
test('RH follows existing permissions and licensed module, not only owner role', () => {
  assert.equal(canView(caps()), true);
  assert.equal(canView(caps('employee', [])), false);
  assert.equal(canView(caps('owner', [], ['staff'])), false);
  assert.equal(canView(caps('manager', ['manage_staff'], [])), false);
});
test('management has operational areas but no salary or benefits access inferred from manage_staff', () => {
  const areas = visibleAreas(caps('manager'));
  assert(!areas.some((a) => a.sensitive));
  assert.equal(sourceAccess('payroll', caps('manager')), 'forbidden');
  assert.equal(sourceAccess('finance', caps('employee', [])), 'forbidden');
  assert(visibleAreas(caps()).some((a) => a.id === 'payroll'));
});
test('only real staff route is active; unfinished workflows are not clickable destinations', () => {
  for (const area of visibleAreas(caps()))
    assert.equal(areaState(area, caps()), area.id === 'people' ? 'ready' : 'unavailable');
});
test('unlicensed finance never returns a fabricated zero', () =>
  assert.equal(sourceAccess('finance', caps()), 'not_licensed'));
test('search, filters, metric IDs and ordering combine without mutating roster', () => {
  const people = [
    { id: 'b', name: 'Zoe', department: 'Ops', jobTitle: 'Cashier', status: 'active' },
    { id: 'a', name: 'Alice', department: 'Sales', jobTitle: 'Cashier', status: 'active' },
  ];
  assert.equal(filterPeople(people, { jobTitle: 'Cashier', ids: ['a'] })[0].name, 'Alice');
  assert.equal(filterPeople(people, { query: 'ops' }).length, 1);
  assert.equal(filterPeople(people, { status: 'revoked' }).length, 0);
  assert.equal(filterPeople(people, {})[0].name, 'Alice');
  assert.equal(people[0].name, 'Zoe');
});
test('avatar supports Unicode and never needs a UUID to identify a person', () => {
  assert.equal(initials('  Ana Maria Lima '), 'AM');
  assert.equal(initials('王 明'), '王明');
  assert.equal(initials(''), '');
});
test('CSV quotes and neutralizes spreadsheet injection without logging personal data', () => {
  assert.equal(
    csv([['=HYPERLINK("x")', 'Ana\nMaria']]),
    '\uFEFF"\'=HYPERLINK(""x"")","Ana\nMaria"'
  );
});
test('currency uses exact cents and refuses invalid totals', () => {
  assert(currency(1050, 'pt-BR').includes('10,50'));
  assert.equal(currency(undefined, 'pt-BR'), '—');
  assert.equal(currency(-1, 'pt-BR'), '—');
});
test('all five dictionaries have identical complete keys and no missing visible strings', () => {
  let keys;
  for (const lang of ['pt-BR', 'en', 'es', 'fr', 'zh-CN']) {
    const d = JSON.parse(
      readFileSync(new URL(`../src/i18n/hr71/${lang}.json`, import.meta.url), 'utf8')
    );
    assert(
      Object.entries(d)
        .filter(([k]) => k !== '_review')
        .every(([, v]) => typeof v === 'string' && v.length)
    );
    const own = Object.keys(d).sort();
    if (keys) assert.deepEqual(own, keys);
    else keys = own;
  }
});
test('unsupported sources have no guessed fetch URL or mutation', () => {
  for (const name of [
    'attendance',
    'schedules',
    'leave',
    'documents',
    'payroll',
    'finance',
    'alerts',
  ]) {
    const source = readFileSync(new URL(`../src/core/hr71/${name}.ts`, import.meta.url), 'utf8');
    assert(!source.includes('fetch('));
    assert(!source.includes('/local/v1/'));
  }
});
