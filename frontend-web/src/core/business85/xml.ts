import {decimal85} from './rules.mjs';
import type {PurchaseLine85} from './types';
export interface XMLPreview85{supplier:string;taxId:string;number:string;reference:string;items:PurchaseLine85[]}
/** Parses local XML as inert data. No signature validation or fiscal acknowledgement. */
export function parseXML85(text:string):XMLPreview85{
 if(text.length>1000000||/<!DOCTYPE|<!ENTITY/i.test(text))throw Error('xmlInvalid');
 const doc=new DOMParser().parseFromString(text,'application/xml');if(doc.querySelector('parsererror'))throw Error('xmlInvalid');
 const find=(node:Document|Element,name:string)=>Array.from(node.getElementsByTagName('*')).find(e=>e.localName===name),value=(node:Document|Element,name:string)=>find(node,name)?.textContent?.trim()||'';
 const invoice=find(doc,'infNFe'),issuer=invoice&&find(invoice,'emit');if(!invoice||!issuer)throw Error('xmlInvalid');
 const lines=Array.from(invoice.getElementsByTagName('*')).filter(e=>e.localName==='det');if(!lines.length||lines.length>200)throw Error('xmlInvalid');
 const fixed=(s:string,scale:number)=>decimal85(s.includes('.')?s.replace(/0+$/,'').replace(/\.$/, ''):s,scale);
 const items=lines.map((node,index)=>{const product=find(node,'prod');if(!product)throw Error('xmlInvalid');const unit=value(product,'uCom').toLowerCase(),quantity=fixed(value(product,'qCom'),1000),price=fixed(value(product,'vUnCom'),100);if(!quantity)throw Error('xmlInvalid');return {id:'XML-'+index,sku:value(product,'cProd'),name:value(product,'xProd'),unit:({un:'unit',und:'unit',l:'litre'} as Record<string,string>)[unit]||unit,quantity,price,received:0};});
 if(items.some(i=>!i.sku||!i.name||!['unit','kg','g','litre'].includes(i.unit)))throw Error('xmlUnits');
 return {supplier:value(issuer,'xNome'),taxId:value(issuer,'CNPJ'),number:value(invoice,'nNF'),reference:invoice.getAttribute('Id')?.replace(/^NFe/,'')||'',items};
}
