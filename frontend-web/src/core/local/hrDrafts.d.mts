export const hrSections:string[];
export const hrEditableSections:string[];
export interface HRDraftInput {section:string;subject:string;person:string;notes:string;date:string;endDate:string;email:string;stage:string;amount:string;startTime:string;endTime:string;}
export interface HRDraft extends HRDraftInput {id:string;}
export function validateHRDraft(value:unknown):boolean;
export function createHRDraft(input:HRDraftInput,id:string):HRDraft;
