import type { Adapter } from './contracts';
export const stock: Adapter = async () => {
  if (import.meta.env.VITE_SHEETS_MOCKS !== 'true') throw Error('unavailable');
  return {
    columns: ['sku', 'product', 'quantity'],
    rows: [['DEMO-1', 'Demo', 12]],
    partial: false,
    mock: true,
    at: Date.now(),
  };
};
