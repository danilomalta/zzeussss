import type { createLocalClient, LocalSession, SaleInput, CashOpenInput, CashCloseInput, SaleReceipt, CashOpenResult, CashCloseResult } from './localClient.mjs';
export type PendingOperation =
  { version: 1; kind: 'sale'; input: SaleInput; uncertain: boolean } |
  { version: 1; kind: 'open'; input: CashOpenInput; uncertain: boolean } |
  { version: 1; kind: 'close'; input: CashCloseInput; uncertain: boolean };
export type OperationOutcome = { kind: 'sale'; result: SaleReceipt } | { kind: 'open'; result: CashOpenResult } | { kind: 'close'; result: CashCloseResult };
export function createPOSOperations(client: ReturnType<typeof createLocalClient>, storage: Pick<Storage, 'getItem' | 'setItem' | 'removeItem'>,
  scope: LocalSession, lock: <T>(key: string, action: () => Promise<T>) => Promise<T>): {
    pending(): PendingOperation | null;
    canDiscard(): boolean;
    start(token: string, kind: PendingOperation['kind'], input: SaleInput | CashOpenInput | CashCloseInput): Promise<OperationOutcome>;
    retry(token: string): Promise<OperationOutcome>;
    discardRejected(token: string): Promise<void>;
  };
