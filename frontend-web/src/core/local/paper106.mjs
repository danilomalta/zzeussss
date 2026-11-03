import {parsePaper92} from './closing92.mjs';
import {parseMoney} from './posModel.mjs';
export function validDate106(value){
 if(typeof value!=='string')return false;
 const m=/^(\d{4})-(\d{2})-(\d{2})T([01]\d|2[0-3]):([0-5]\d)$/.exec(value);
 if(!m||+m[1]<2000||+m[1]>2199)return false;
 const d=new Date(Date.UTC(+m[1],+m[2]-1,+m[3]));
 return d.getUTCFullYear()===+m[1]&&d.getUTCMonth()===+m[2]-1&&d.getUTCDate()===+m[3];
}
export function emptyPaper106(){return {amount:'',method:'',reference:'',at:''};}
export function readPaper106(text){
 if(typeof text!=='string'||!text.trim()||text.length>12000)throw Error('invalidPaper106');
 const p=parsePaper92(text),clean=text.normalize('NFD').replace(/[\u0300-\u036f]/g,'').toUpperCase();
 const refs=[...clean.matchAll(/\b(?:NSU|AUTORIZACAO|COD(?:IGO)?\s*AUT)\s*[:#=]?\s*([A-Z0-9-]{3,40})(?![A-Z0-9-])/g)].map(m=>m[1]);
 const dates=[...clean.matchAll(/\b\d{2}\/\d{2}\/\d{4}\b/g)].map(m=>m[0]);
 const times=[...clean.matchAll(/\b(?:[01]\d|2[0-3]):[0-5]\d(?::[0-5]\d)?\b/g)].map(m=>m[0].slice(0,5));
 const amounts=[...clean.matchAll(/(?:VALOR(?:\s+TOTAL|\s+DA\s+VENDA)?|TOTAL|R\$)\s*[:=]?\s*(?:R\$\s*)?([\d.]+[,\.]\d{2})(?!\d)/g)];let amount='';
 if(amounts.length===1){const v=amounts[0][1],raw=v.includes(',')?v.replaceAll('.','').replace(',','.'):v;try{const cents=BigInt(parseMoney(raw));if(cents>0n)amount=`${cents/100n}.${String(cents%100n).padStart(2,'0')}`;}catch{/* Leave invalid value blank. */}}
 return {amount,method:p.method,reference:new Set(refs).size===1?refs[0]:'',at:validDate106(p.at)&&new Set(dates).size===1&&new Set(times).size===1?p.at:''};
}
export function checkedPaper106(p){
 if(!p||!['debit','credit','pix'].includes(p.method)||typeof p.amount!=='string'||typeof p.reference!=='string'||!/^[A-Za-z0-9-]{3,40}$/.test(p.reference)||!validDate106(p.at))throw Error('invalidPaper106');
 const cents=parseMoney(p.amount);if(cents<=0)throw Error('invalidPaper106');
 const n=BigInt(cents);
 return {amount:`${n/100n}.${String(n%100n).padStart(2,'0')}`,method:p.method,reference:p.reference.toUpperCase(),at:p.at};
}
export function paperText106(p){const v=checkedPaper106(p),[y,m,d]=v.at.slice(0,10).split('-');return `VALOR: ${v.amount}\n${{debit:'DEBITO',credit:'CREDITO',pix:'PIX'}[v.method]}\nNSU: ${v.reference}\n${d}/${m}/${y} ${v.at.slice(11)}`;}
