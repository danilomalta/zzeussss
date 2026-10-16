import type {CheckoutAdapter96} from './checkout96.mjs';
import {createContext} from 'react';import type {Domain95,Input95,Reply95} from './desk95.mjs';
export interface Gateway95{checkout96?:CheckoutAdapter96;mode:'live'|'demo';features:readonly Domain95[];command:(token:string,input:Input95,signal:AbortSignal)=>Promise<Reply95>}
// Production has no guessed URLs and no successful fallback. Bind verified contracts here.
export const disconnected95:Gateway95={mode:'live',features:[],async command(){throw Error('unavailable');}};
export const GatewayContext95=createContext<Gateway95>(disconnected95);
