export function productionResponsibles(value){
 const id=s=>typeof s==='string'&&s===s.trim()&&s.length>0&&new TextEncoder().encode(s).length<=128&&!/[\x00\r\n]/.test(s);
 if(!value||!Array.isArray(value.items)||value.items.length>100||new Set(value.items.map(row=>row?.identity_id)).size!==value.items.length||!value.items.every(row=>row&&id(row.identity_id)&&typeof row.name==='string'&&row.name.trim()&&typeof row.status==='string'&&['owner','manager','employee','cashier','stock','production','accountant','supplier'].includes(row.role)))throw Error('productionInvalid');
 return {items:value.items.filter(row=>row.status==='active').map(row=>({id:row.identity_id,name:row.name})),limited:value.items.length===100};
}
