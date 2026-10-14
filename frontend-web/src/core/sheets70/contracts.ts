import type { LocalCapabilities } from '../local/localClient.mjs';
export type Cell = string | number | boolean;
export interface Filters {
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
  token: string;
  capabilities: LocalCapabilities;
  signal: AbortSignal;
}
export type Adapter = (context: Context, filters: Filters) => Promise<DataSet>;
