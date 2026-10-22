import type {ClosingDraft92,ClosingSale92} from './closing92.mjs';
import type {Review97} from './closingReview97.mjs';
import type {CountSummary107} from './count107.mjs';
export interface FinalReview108 {counted:CountSummary107;review:Review97;gate:string|null;exceptions:boolean;exceptionApproved:boolean;signature:string;}
export function finalStamp108(d:ClosingDraft92,sales:ClosingSale92[],complete:boolean):string;
export function finalReview108(d:ClosingDraft92,sales:ClosingSale92[],complete:boolean,manager:boolean):FinalReview108;
export function confirmFinal108(d:ClosingDraft92,sales:ClosingSale92[],complete:boolean,manager:boolean,signature:string,ack:boolean):number;
