import {money,xmlPreflight} from './engine.mjs';
export interface Invoice{key:string;number:string;issued:string;issuer:string;recipient:string;total:number;protocol:string;items:{code:string;name:string;ncm:string;cfop:string;cst:string;quantity:string;total:number}[];raw:string}
export function readInvoice(raw:string):Invoice{
 const doc=new DOMParser().parseFromString(xmlPreflight(raw),'application/xml');
 if(doc.getElementsByTagName('parsererror').length)throw Error('invalid');
 const all=(node:Document|Element,name:string)=>Array.from(node.getElementsByTagNameNS('*',name));
 const node=all(doc,'infNFe');if(node.length!==1)throw Error('invalid');const inf=node[0],key=(inf.getAttribute('Id')||'').replace(/^NFe/,'');
 if(!/^\d{44}$/.test(key))throw Error('invalid');
 const val=(parent:Document|Element,name:string)=>all(parent,name)[0]?.textContent?.trim()||'';
 const emit=all(inf,'emit')[0],dest=all(inf,'dest')[0],ide=all(inf,'ide')[0],tot=all(inf,'ICMSTot')[0];if(!emit||!ide||!tot)throw Error('invalid');
 if(!['55','65'].includes(val(ide,'mod')))throw Error('invalid');
 const details=all(inf,'det');if(details.length>1000)throw Error('invalid');
 const items=details.map(det=>{const prod=all(det,'prod')[0];if(!prod)throw Error('invalid');return {code:val(prod,'cProd'),name:val(prod,'xProd'),ncm:val(prod,'NCM'),cfop:val(prod,'CFOP'),cst:val(det,'CST')||val(det,'CSOSN'),quantity:val(prod,'qCom'),total:money(val(prod,'vProd'))};});
 const protocol=all(doc,'infProt')[0];const status=protocol?val(protocol,'cStat'):'';
 return {key,number:val(ide,'nNF'),issued:val(ide,'dhEmi')||val(ide,'dEmi'),issuer:val(emit,'xNome'),recipient:dest?val(dest,'CNPJ')||val(dest,'CPF'):'',total:money(val(tot,'vNF')),protocol:['100','150'].includes(status)?val(protocol!,'nProt'):'',items,raw};
}
