import type { Document } from './store.mjs';
import { listDocuments, saveDocument, deleteDocument } from './store.mjs';
/** A company connector is supplied by an authenticated API integration, never a guessed URL. */
export interface CompanySheetStorage {
  kind: 'company';
  list(scope: string): Promise<Document[]>;
  save(scope: string, document: Document, expectedRevision: number): Promise<Document>;
  remove(scope: string, id: string): Promise<void>;
}
export const localSheetStorage = {
  kind: 'local' as const,
  list: listDocuments,
  save: saveDocument,
  remove: deleteDocument,
};
let company: CompanySheetStorage | undefined;
export function configureCompanySheets(adapter: CompanySheetStorage | undefined) {
  company = adapter;
}
export function sheetStorage() {
  return company || localSheetStorage;
}
