export interface SheetCell {value:string;bold:boolean;italic:boolean;align:'left'|'center'|'right';format:'general'|'number'|'currency'|'percent';fill:'none'|'muted';}
export interface TitanSheet {id:string;name:string;rows:number;cols:number;cells:Record<string,SheetCell>;freeze:boolean;}
export interface TitanWorkbook {schema:1;title:string;sheets:TitanSheet[];}
export const MAX_ROWS:number,MAX_COLS:number,MAX_SHEETS:number;
export const defaultCell:Readonly<SheetCell>;
export function cellKey(row:number,col:number):string;
export function createSheet(id:string,name:string):TitanSheet;
export function createWorkbook(id:string,title:string,name:string):TitanWorkbook;
export function evaluateSheet(sheet:TitanSheet):Record<string,string|number|boolean>;
export function parseWorkbook(text:string):TitanWorkbook;
export function csvText(matrix:(string|number|boolean)[][],delimiter?:string):string;
export function parseDelimited(text:string):string[][];
export function sortSheet(sheet:TitanSheet,col:number,descending?:boolean):TitanSheet;
