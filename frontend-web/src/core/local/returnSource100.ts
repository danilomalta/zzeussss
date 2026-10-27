import {createContext} from 'react';import type {Receipt96} from './checkout96.mjs';
// Optional authorized adapter. Production falls back to existing checkout/local clients.
// No endpoint is invented; the demonstration overrides this context explicitly.
export interface ReturnSource100{receipt:(token:string,id:string,signal:AbortSignal)=>Promise<Receipt96>}
export const ReturnSourceContext100=createContext<ReturnSource100|null>(null);
