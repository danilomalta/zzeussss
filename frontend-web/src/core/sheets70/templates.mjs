const models = {
  catalog: {
    columns: ['sku', 'product', 'unit', 'cost', 'margin', 'price'],
    rows: [['DEMO-1', 'Demo', 'unit', 10, 0.3, { f: '=D2/(1-E2)' }]],
  },
  stock: {
    columns: ['sku', 'product', 'incoming', 'outgoing', 'quantity'],
    rows: [['DEMO-1', 'Demo', 20, 5, { f: '=C2-D2' }]],
  },
  cashflow: {
    columns: ['date', 'description', 'incoming', 'outgoing', 'balance'],
    rows: [
      ['2026-01-01', 'Demo', 100, 30, { f: '=C2-D2' }],
      ['2026-01-02', 'Demo', 50, 10, { f: '=E2+C3-D3' }],
    ],
  },
  pricing: {
    columns: ['product', 'cost', 'margin', 'price', 'profit'],
    rows: [['Demo', 10, 0.3, { f: '=B2/(1-C2)' }, { f: '=D2-B2' }]],
  },
  schedule: {
    columns: ['person', 'date', 'start', 'end', 'hours'],
    rows: [['Demo', '2026-01-01', 9 / 24, 17 / 24, { f: '=(D2-C2)*24' }]],
  },
  closing: {
    columns: ['date', 'method', 'expected', 'counted', 'difference'],
    rows: [['2026-01-01', 'Demo', 100, 95, { f: '=D2-C2' }]],
  },
  payable: {
    columns: ['supplier', 'due', 'amount', 'paid', 'balance'],
    rows: [['Demo', '2026-01-31', 100, 20, { f: '=C2-D2' }]],
  },
  receivable: {
    columns: ['person', 'due', 'amount', 'received', 'balance'],
    rows: [['Demo', '2026-01-31', 100, 20, { f: '=C2-D2' }]],
  },
};
export function templateBook(base, kind, label) {
  const model = models[kind];
  if (!model) throw Error('template');
  const book = structuredClone(base),
    id = crypto.randomUUID(),
    cellData = { 0: Object.fromEntries(model.columns.map((key, col) => [col, { v: label(key) }])) };
  model.rows.forEach((row, r) => {
    cellData[r + 1] = Object.fromEntries(
      row.map((value, c) => [c, typeof value === 'object' ? value : { v: value }])
    );
  });
  book.id = crypto.randomUUID();
  book.name = label(kind);
  book.styles = {};
  book.resources = [];
  book.sheetOrder = [id];
  book.sheets = { [id]: { id, name: label(kind), rowCount: 1000, columnCount: 50, cellData } };
  return book;
}
