import type { LocalCapabilities } from '../local/localClient.mjs';
export type Cell = string | number | boolean;
export interface Filters {
  location_id?: string;
  as_of?: string;
  q: string;
  from: string;
  to: string;
}
export interface DataSet {
  columns: string[];
  rows: Cell[][];
  partial: boolean;
  mock: boolean;
  at: number;
}
export interface Context {
  session: import('../local/localClient.mjs').LocalSession;
  isCurrent: () => boolean;
  locale?: string;
  token: string;
  capabilities: LocalCapabilities;
  signal: AbortSignal;
}
export type Adapter = (context: Context, filters: Filters) => Promise<DataSet>;
