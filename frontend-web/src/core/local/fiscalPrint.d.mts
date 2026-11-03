export function printFiscalReceipt(options:{load:()=>Promise<string>;signal?:AbortSignal;open?:()=>Window|null}):Promise<Window>;
