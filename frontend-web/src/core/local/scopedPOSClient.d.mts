import type {createLocalClient,LocalSession,LocalCapabilities} from './localClient.mjs';
export type ScopedPOSClient=Pick<ReturnType<typeof createLocalClient>,'products'|'locations'|'history'|'currentCash'|'receipt'|'staff'|'quoteSale'|'completeSale'|'openCash'|'closeCash'>&{assertCurrent(token?:string):void};
export function createScopedPOSClient(client:ReturnType<typeof createLocalClient>,options:{token:string;session:LocalSession;getCurrent:()=>{token:string|null;session:LocalSession|null;capabilities:LocalCapabilities|null}}):ScopedPOSClient;
