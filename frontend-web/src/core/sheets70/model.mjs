import { allowedAreas } from '../local/accessModel.mjs';
export function namespace(session) {
  if (!session?.tenant_id || !session?.identity_id) throw Error('session');
  return JSON.stringify([
    session.tenant_id,
    session.store_id,
    session.device_id,
    session.identity_id,
  ]);
}
export function cleanSnapshot(workbook, blocks = []) {
  const copy = structuredClone(workbook);
  for (const block of blocks) {
    const data = copy.sheets[block.sheetId]?.cellData;
    if (!data) continue;
    for (let r = block.row; r < block.row + block.height; r++)
      for (let c = block.col; c < block.col + block.width; c++) if (data[r]) delete data[r][c];
  }
  if (blocks.length)
    for (const sheet of Object.values(copy.sheets))
      for (const row of Object.values(sheet.cellData || {}))
        for (const cell of Object.values(row))
          if (cell.f) {
            delete cell.v;
            delete cell.p;
          }
  return copy;
}
export function checkSpace(workbook, block, matrix, previous) {
  if (!matrix.length || !matrix[0].length) throw Error('empty');
  const height = matrix.length,
    width = matrix[0].length;
  if (
    matrix.some((row) => row.length !== width) ||
    block.row + height > 10000 ||
    block.col + width > 200
  )
    throw Error('limit');
  const data = workbook.sheets[block.sheetId]?.cellData || {};
  for (let r = block.row; r < block.row + height; r++)
    for (let c = block.col; c < block.col + width; c++) {
      const own =
        previous &&
        r >= previous.row &&
        r < previous.row + previous.height &&
        c >= previous.col &&
        c < previous.col + previous.width;
      const cell = data[r]?.[c];
      if (!own && cell && (cell.v !== undefined || cell.f || cell.p)) throw Error('conflict');
    }
  return { ...block, height, width };
}
export function sourceAllowed(source, capabilities) {
  if (!capabilities) return false;
  const area = {
    catalog: 'catalog',
    sales: 'pos',
    staff: 'staff',
    purchases: 'orders',
    stock: 'stock',
    finance: 'accounting',
  }[source];
  return allowedAreas(capabilities).includes(area);
}
export function tableCSV(matrix) {
  return matrix
    .map((row) =>
      row
        .map((v) => {
          let s = String(v ?? '');
          if (/^[=+@-]/.test(s)) s = "'" + s;
          return '"' + s.replaceAll('"', '""') + '"';
        })
        .join(',')
    )
    .join('\r\n');
}
export function usedMatrix(workbook, sheetId) {
  const data = workbook.sheets[sheetId]?.cellData || {};
  const coords = Object.keys(data).map(Number);
  if (!coords.length) return [];
  const height = Math.min(501, Math.max(...coords) + 1),
    width = Math.min(
      200,
      Math.max(0, ...Object.values(data).flatMap((row) => Object.keys(row).map(Number))) + 1
    );
  return Array.from({ length: height }, (_, r) =>
    Array.from({ length: width }, (_, c) => data[r]?.[c]?.v ?? '')
  );
}
export function validateImport(matrix, mapping, kind) {
  const needed =
    {
      prices: ['sku', 'price'],
      inventory: ['sku', 'quantity'],
      products: ['sku', 'name', 'price'],
      employees: ['name', 'role'],
    }[kind] || [];
  const indexes = Object.fromEntries(mapping.map((key, i) => [key, i]));
  return matrix.slice(1).map((row, i) => {
    const errors = [];
    for (const key of needed) {
      const index = indexes[key];
      if (index === undefined || row[index] === '' || row[index] === undefined) errors.push(key);
    }
    for (const key of ['price', 'quantity'])
      if (needed.includes(key) && indexes[key] !== undefined) {
        const value = String(row[indexes[key]]);
        const pattern = key === 'price' ? /^\d+(?:[.,]\d{1,2})?$/ : /^\d+(?:[.,]\d{1,3})?$/;
        if (!pattern.test(value)) errors.push(key);
      }
    if (kind === 'employees' && row[indexes.role] === 'owner') errors.push('role');
    return { line: i + 2, row, errors: [...new Set(errors)] };
  });
}
export function lookup(cache, source, field, key) {
  const aliases = { vendas: 'sales', catalogo: 'catalog', estoque: 'stock' };
  const data = cache[aliases[source] || source];
  if (!data) return '#N/A';
  const index = data.columns.indexOf(field);
  if (index < 0) return '#N/A';
  const row = data.rows.find((r) => String(r[0]) === String(key));
  return row?.[index] ?? '#N/A';
}
export function trackStructure(blocks, command) {
  const p = command.params;
  if (!p?.subUnitId) return blocks;
  if (command.id === 'sheet.mutation.remove-sheet')
    return blocks.filter((b) => b.sheetId !== p.subUnitId);
  const match = /^sheet\.mutation\.(insert-row|insert-col|remove-rows|remove-col)$/.exec(
    command.id
  );
  if (!match || !p.range) return blocks;
  const vertical = match[1].includes('row'),
    insert = match[1].startsWith('insert'),
    start = vertical ? p.range.startRow : p.range.startColumn,
    end = vertical ? p.range.endRow : p.range.endColumn,
    count = end - start + 1;
  if (!Number.isInteger(start) || !Number.isInteger(count) || count < 1) return blocks;
  return blocks
    .map((b) => {
      if (b.sheetId !== p.subUnitId) return b;
      const pos = vertical ? b.row : b.col,
        size = vertical ? b.height : b.width;
      let next = pos,
        length = size;
      if (insert) {
        if (start <= pos) next += count;
        else if (start < pos + size) length += count;
      } else {
        const overlap = Math.max(0, Math.min(pos + size, end + 1) - Math.max(pos, start));
        length -= overlap;
        if (end < pos) next -= count;
        else if (start < pos) next = start;
      }
      return { ...b, [vertical ? 'row' : 'col']: next, [vertical ? 'height' : 'width']: length };
    })
    .filter((b) => b.height > 0 && b.width > 0);
}
export function ensureNoOverlap(block, blocks) {
  for (const other of blocks)
    if (
      other.id !== block.id &&
      other.sheetId === block.sheetId &&
      block.row < other.row + other.height &&
      block.row + block.height > other.row &&
      block.col < other.col + other.width &&
      block.col + block.width > other.col
    )
      throw Error('conflict');
}
export function selectionMatrix(book, selection) {
  const height = selection.height || 1,
    width = selection.width || 1;
  if (height > 500 || width > 200 || height * width > 100000) throw Error('limit');
  const cells = book.sheets[selection.sheetId]?.cellData || {};
  return Array.from({ length: height }, (_, r) =>
    Array.from({ length: width }, (_, c) => cells[r + selection.row]?.[c + selection.col]?.v ?? '')
  );
}
