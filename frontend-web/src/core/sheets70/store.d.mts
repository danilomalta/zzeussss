import type { IWorkbookData } from '@univerjs/presets';
import type { Block } from './model.mjs';
export interface Document {
  id: string;
  name: string;
  workbook: IWorkbookData;
  blocks: Block[];
  activity: { at: number; action: string }[];
  revision: number;
  updated: number;
  versions: { at: number; workbook: IWorkbookData; blocks: Block[] }[];
}
export function listDocuments(scope: string): Promise<Document[]>;
export function saveDocument(
  scope: string,
  document: Document,
  expectedRevision: number
): Promise<Document>;
export function deleteDocument(scope: string, id: string): Promise<void>;
