export function tableCSV(rows,words,locale='pt-BR'){
 if(!Array.isArray(rows)||rows.length>500)throw Error('invalid');
 const number=n=>new Intl.NumberFormat(locale,{useGrouping:false,maximumFractionDigits:3}).format(n);
 const cell=value=>'"'+String(value??'').replace(/^([\s]*[=+@-])/u,"'$1").replace(/"/g,'""')+'"';
 const headers=[words.name,words.unit,words.available,words.losses,words.cost,words.price,words.expiry,words.coverage,words.suggestion,words.reorderQty,words.monitor];
 const table=rows.map(({row,a})=>[row.name,row.unit==='unit'?words.unitLabel:row.unit==='litre'?words.litre:'kg',number(a.stock/1000),number(row.loss_milli/1000),number(row.cost_cents/100),number(row.price_cents/100),row.expiry||words.unknown,a.coverage===null?words.unknown:number(a.coverage),a.suggested===null?words.unknown:number(a.suggested/100),number(a.reorder/1000),a.expired?words.expired:a.near?words.near:a.slow?words.slow:words.healthy]);
 return '\uFEFF'+[headers,...table].map(line=>line.map(cell).join(';')).join('\r\n')+'\r\n';
}
