import type { IWorkbookData } from '@univerjs/presets';
export interface Block {
  id: string;
  source: string;
  sheetId: string;
  row: number;
  col: number;
  height: number;
  width: number;
  updated: number;
  interval: number;
  detached?: boolean;
  partial?: boolean;
  filters?: { location_id?: string; as_of?: string; q: string; from: string; to: string };
  columns?: string[];
}
export function namespace(session: {
  tenant_id: string;
  store_id: string;
  device_id: string;
  identity_id: string;
}): string;
export function cleanSnapshot(workbook: IWorkbookData, blocks?: Block[]): IWorkbookData;
export function checkSpace(
  workbook: IWorkbookData,
  block: Block,
  matrix: (string | number | boolean)[][],
  previous?: Block
): Block;
export function sourceAllowed(source: string, capabilities: unknown): boolean;
export function tableCSV(matrix: unknown[][]): string;
export function usedMatrix(book: IWorkbookData, id: string): (string | number | boolean)[][];
export function validateImport(
  matrix: unknown[][],
  mapping: string[],
  kind: string
): { line: number; row: unknown[]; errors: string[] }[];
export function lookup(
  cache: Record<string, { columns: string[]; rows: unknown[][] }>,
  source: string,
  field: string,
  key: unknown
): unknown;
export function trackStructure(blocks: Block[], command: unknown): Block[];
export function ensureNoOverlap(block: Block, blocks: Block[]): void;
export function selectionMatrix(
  book: IWorkbookData,
  selection: { sheetId: string; row: number; col: number; height?: number; width?: number }
): (string | number | boolean)[][];
