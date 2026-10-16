export type Domain95='payments'|'credit'|'fiscal'|'loyalty'|'discount'|'refund'|'devices'|'pairing'|'report';
export interface Intent95{id:string;domain:Domain95;action:string;amount:number;state:string;reference:string;source:string;at:string;mode:'live'|'demo'}
export interface Input95{domain:Domain95;action:string;source?:string;operation?:string;sale?:string;amount?:number;method?:string;customer?:string;cpf?:string;name?:string;email?:string;consent?:boolean;points?:number;reason?:string;installments?:{due:string;amount:number}[];device?:string;provider?:string;paper?:string;orientation?:string;username?:string;password?:string;cash?:string;scenario?:string;part?:string;gross?:number;code?:string;paperFields?:{id:string;reference:string;amount:number;method:string;at:string}}
export interface Reply95{operation:string;domain:Domain95;state:string;reference:string;amount?:number;expires?:string;qr?:string;copy?:string;message?:string;refundedAmount?:number;member?:{id:string;name:string;points:number};credit?:{customer:string;limit:number;used:number};document?:{number:string;accessKey:string;kind:string;pdf?:string};devices?:{id:string;name:string;kind:string;state:string}[];pair?:{code:string;device:string;cash:string;lastSeen:string;reviewed:number};report?:{opening:number;cashSales:number;withdrawals:number;deposits:number;refunds:number;expected:number;counted:number;difference:number;debit:number;credit:number;pix:number;pending:number;reviewer:string}}
export const domains95:Domain95[];export const states95:Record<Domain95,string[]>;
export function validateReply95(r:unknown,intent:Intent95):Reply95;
export function safeIntent95(input:Input95,mode:'live'|'demo'):Intent95;
export function validateRecords95(r:unknown,mode:'live'|'demo'):Intent95[];
export function creditSchedule95(amount:number,count:number,start:string):{due:string;amount:number}[];
export function proposed95(gross:number,discount:number):{gross:number;discount:number;net:number};
export function remainingSeconds95(expires:string,now?:number):number;
