import type { ReceivingOverview, ReceivingBoard } from './contracts';
export interface ReceivingFilters {
    from: string;
    to: string;
    supplier_id?: string;
    product_id?: string;
    state?: 'pending' | 'all' | 'received' | 'cancelled' | 'review';
}
export function readFail(): never;
export function receivingReadID(value: unknown): boolean;
export const receivingFlags: string[];
export function overviewFilters(value: ReceivingFilters): Required<ReceivingFilters>;
export function receivingOverview(value: unknown, id: string, offset?: number): ReceivingOverview;
export function receivingBoard(value: unknown, filters: ReceivingFilters, offset?: number): ReceivingBoard;
export function receivingQuantity(value: number | string, locale?: string): string;
export function receivingCSV(rows: unknown[][]): string;
