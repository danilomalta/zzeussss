import type {Source90} from '../types';
/** The existing restock API reports product balance, not a location/lot ledger. */
export interface StockPosition90{sku:string;location:string;lot:string;qty:number;revision:number}
export async function stockPositions90():Promise<Source90<StockPosition90>>{return {state:'unavailable',rows:[]};}
