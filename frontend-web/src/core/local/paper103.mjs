import {parsePaper92} from './closing92.mjs';
// Each camera read starts from a blank form. Missing fields never inherit another receipt.
export function readPaper103(text){const p=parsePaper92(text);return {amount:p.amount||'',reference:p.reference||'',method:p.method||'',at:p.at||''};}
