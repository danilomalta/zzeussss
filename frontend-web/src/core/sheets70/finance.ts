import type { Adapter } from './contracts';
export const finance: Adapter = async () => {
  if (import.meta.env.VITE_SHEETS_MOCKS !== 'true') throw Error('unavailable');
  return {
    columns: ['date', 'description', 'amount'],
    rows: [['2026-01-01', 'Demo', 100]],
    partial: false,
    mock: true,
    at: Date.now(),
  };
};
