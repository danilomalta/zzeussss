export interface LocalSession {
  tenant_id: string;
  store_id: string;
  identity_id: string;
  device_id: string;
  expires_unix: number;
}
export interface LocalProduct {
  id: string;
  sku: string;
  name: string;
  unit: string;
  price_cents: number;
  barcode?: string;
  cost_cents?: number;
}
export class LocalAPIError extends Error {
  status: number;
  constructor(status: number, message: string);
}
export function formatLocalCents(value: number): string;
export interface LocalLocation { id: string; kind: string; name: string; }
export interface CashSession { session_id: string; opened_at: string; }
export interface CashOpenInput { session_id: string; opening_cents: number; }
export interface CashCloseInput { session_id: string; operation_id: string; declared_cents: number; }
export interface CashOpenResult { session_id: string; repeated: boolean; }
export interface CashCloseResult extends CashOpenResult { expected_cents: number; difference_cents: number; }
export interface SaleInput {
  operation_id: string; sale_id: string; cash_session_id: string;
  items: { product_id: string; location_id: string; quantity_milli: number }[];
  payments: { method: 'cash'; amount_cents: number }[];
}
export interface SaleResult { sale_id: string; total_cents: number; repeated: boolean; }
export interface SaleReceipt {
  sale_id: string; operation_id: string; cash_session_id: string; status: 'committed' | 'cancelled';
  total_cents: number; fiscal_authorized: false;
  items: { item_id: string; product_id: string; location_id: string; quantity_milli: number; unit_price_cents: number; total_cents: number }[];
  payments: { payment_id: string; method: string; status: string; amount_cents: number }[];
}
export function createLocalClient(fetcher?: typeof fetch): {
  login(identityID: string, password: string): Promise<{ token: string; session: LocalSession }>;
  products(token: string, offset?: number): Promise<LocalProduct[]>;
  locations(token: string): Promise<LocalLocation[]>;
  currentCash(token: string): Promise<CashSession | null>;
  openCash(token: string, input: CashOpenInput): Promise<CashOpenResult>;
  closeCash(token: string, input: CashCloseInput): Promise<CashCloseResult>;
  completeSale(token: string, input: SaleInput): Promise<SaleResult>;
  receipt(token: string, saleID: string): Promise<SaleReceipt>;
  logout(token: string): Promise<void>;
};
