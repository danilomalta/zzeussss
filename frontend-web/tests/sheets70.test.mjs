import test from 'node:test';
import assert from 'node:assert/strict';
import {
  namespace,
  cleanSnapshot,
  checkSpace,
  sourceAllowed,
  tableCSV,
  usedMatrix,
  validateImport,
  lookup,
} from '../src/core/sheets70/model.mjs';
const book = () => ({
  id: 'b',
  name: 'T',
  sheetOrder: ['s'],
  sheets: {
    s: {
      id: 's',
      cellData: {
        0: { 0: { v: 'sku' }, 1: { v: 'secret' } },
        1: { 0: { v: 'a' }, 1: { v: 12 } },
        2: { 4: { f: '=B2', v: 12 } },
      },
    },
  },
});
const block = {
  id: 'link',
  source: 'catalog',
  sheetId: 's',
  row: 0,
  col: 0,
  height: 2,
  width: 2,
  updated: 1,
  interval: 0,
};
test('storage namespace contains company and operator, never a token', () => {
  assert.notEqual(
    namespace({ tenant_id: 'a', identity_id: 'u' }),
    namespace({ tenant_id: 'b', identity_id: 'u' })
  );
  assert.throws(() => namespace({}));
});
test('redaction removes linked values and calculated formula caches, preserving manual data and formula', () => {
  const b = book();
  b.sheets.s.cellData[3] = { 0: { v: 'manual' } };
  const safe = cleanSnapshot(b, [block]);
  assert.deepEqual(safe.sheets.s.cellData[0], {});
  assert.equal(safe.sheets.s.cellData[2][4].f, '=B2');
  assert.equal(safe.sheets.s.cellData[2][4].v, undefined);
  assert.equal(safe.sheets.s.cellData[3][0].v, 'manual');
  assert.equal(b.sheets.s.cellData[0][1].v, 'secret');
});
test('growth never overwrites independent populated cells and accepts only previous owned rectangle', () => {
  assert.equal(
    checkSpace(
      book(),
      block,
      [
        ['a', 'b'],
        ['c', 'd'],
      ],
      block
    ).height,
    2
  );
  assert.throws(() => checkSpace(book(), { ...block, row: 2, col: 3 }, [['x', 'y']]));
  assert.throws(() => checkSpace(book(), { ...block, row: 9999 }, [['x'], ['y']]));
});
test('sources require both licensed module and permission, owner does not bypass it', () => {
  assert.equal(
    sourceAllowed('staff', {
      role: 'owner',
      permissions: ['manage_staff'],
      license: { state: 'not_installed', modules: ['staff'] },
    }),
    false
  );
  assert.equal(
    sourceAllowed('staff', {
      role: 'owner',
      permissions: [],
      license: { state: 'active', modules: ['staff'] },
    }),
    false
  );
  assert.equal(
    sourceAllowed('staff', {
      permissions: ['manage_staff'],
      license: { state: 'active', modules: ['staff'] },
    }),
    true
  );
});
test('sparse cells keep their positions in export preview', () => {
  const matrix = usedMatrix(book(), 's');
  assert.equal(matrix[2][4], 12);
  assert.equal(matrix[2][3], '');
});
test('import preview rejects negative amounts, missing mappings and owner elevation without writes', () => {
  assert.deepEqual(
    validateImport(
      [
        ['sku', 'price'],
        ['A', -1],
      ],
      ['sku', 'price'],
      'prices'
    )[0].errors,
    ['price']
  );
  assert.deepEqual(
    validateImport(
      [
        ['name', 'role'],
        ['A', 'owner'],
      ],
      ['name', 'role'],
      'employees'
    )[0].errors,
    ['role']
  );
  assert.equal(
    validateImport(
      [
        ['sku', 'price'],
        ['A', '12.50'],
      ],
      ['sku', 'price'],
      'prices'
    )[0].errors.length,
    0
  );
});
test('lookup returns unavailable for unloaded sources and no fabricated zero', () => {
  assert.equal(lookup({}, 'catalog', 'price', 'a'), '#N/A');
  assert.equal(
    lookup({ catalog: { columns: ['sku', 'price'], rows: [['a', 12]] } }, 'catalog', 'price', 'a'),
    12
  );
});
test('CSV neutralizes spreadsheet injection', () =>
  assert.equal(tableCSV([['=1+1', 'normal']]), '"\'=1+1","normal"'));
import { trackStructure, ensureNoOverlap } from '../src/core/sheets70/model.mjs';
import { readFileSync } from 'node:fs';
test('native structural mutations shift linked ranges, including undo inverse mutations', () => {
  const shifted = trackStructure([block], {
    id: 'sheet.mutation.insert-row',
    params: { subUnitId: 's', range: { startRow: 0, endRow: 1 } },
  });
  assert.equal(shifted[0].row, 2);
  assert.equal(
    trackStructure(shifted, {
      id: 'sheet.mutation.remove-rows',
      params: { subUnitId: 's', range: { startRow: 0, endRow: 1 } },
    })[0].row,
    0
  );
  assert.equal(
    trackStructure([block], {
      id: 'sheet.mutation.remove-rows',
      params: { subUnitId: 's', range: { startRow: 0, endRow: 1 } },
    }).length,
    0
  );
});
test('growth rejects a separate linked block even when its cells are empty', () =>
  assert.throws(() =>
    ensureNoOverlap({ ...block, width: 5 }, [{ ...block, id: 'other', col: 4 }])
  ));
test('five complete dictionaries share exactly the same keys', () => {
  const languages = ['pt-BR', 'en', 'es', 'fr', 'zh-CN'];
  let expected;
  for (const language of languages) {
    const dictionary = JSON.parse(
      readFileSync(new URL(`../src/i18n/sheets70/${language}.json`, import.meta.url), 'utf8')
    );
    assert(Object.values(dictionary).every((value) => typeof value === 'string'));
    const keys = Object.keys(dictionary).sort();
    if (expected) assert.deepEqual(keys, expected);
    else expected = keys;
  }
});
import { templateBook } from '../src/core/sheets70/templates.mjs';
import { selectionMatrix } from '../src/core/sheets70/model.mjs';
test('eight templates isolate workbooks and contain actual spreadsheet formulas', () => {
  for (const kind of [
    'catalog',
    'stock',
    'cashflow',
    'pricing',
    'schedule',
    'closing',
    'payable',
    'receivable',
  ]) {
    const source = book(),
      made = templateBook(source, kind, (key) => key);
    assert.notEqual(made.id, source.id);
    assert.equal(made.sheetOrder.length, 1);
    assert(
      Object.values(made.sheets[made.sheetOrder[0]].cellData[1]).some((cell) =>
        cell.f?.startsWith('=')
      )
    );
    assert.equal(source.sheets.s.cellData[0][1].v, 'secret');
  }
});
test('selection copying preserves offsets and enforces a bounded region', () => {
  assert.deepEqual(selectionMatrix(book(), { sheetId: 's', row: 1, col: 1, height: 1, width: 1 }), [
    [12],
  ]);
  assert.throws(() =>
    selectionMatrix(book(), { sheetId: 's', row: 0, col: 0, height: 501, width: 1 })
  );
});
