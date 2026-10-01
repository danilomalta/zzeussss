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
export function createLocalClient(fetcher?: typeof fetch): {
  login(identityID: string, password: string): Promise<{ token: string; session: LocalSession }>;
  products(token: string, offset?: number): Promise<LocalProduct[]>;
  logout(token: string): Promise<void>;
};
