import type {Adapter,Filters} from './contracts';
export function sheetFinanceQuery(filters:Filters):{from:string;to:string;as_of:string};
export const sheetFinanceSnapshot:Adapter;
