import type {Lot,Analysis} from './engine.mjs';
export function tableCSV(rows:{row:Lot;a:Analysis}[],words:Record<string,string>,locale?:string):string;
