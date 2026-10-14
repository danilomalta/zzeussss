export function analysisMatrix76(rows,words){
 if(!Array.isArray(rows)||rows.length>500)throw Error('invalid');
 const headers=[words.name,words.unit,words.available,words.losses,words.cost,words.price,words.expiry,words.coverage,words.suggestion,words.reorderQty,words.monitor];
 const table=rows.map(({row,a})=>[row.name,row.unit==='unit'?words.unitLabel:row.unit==='litre'?words.litre:'kg',a.stock/1000,row.loss_milli/1000,row.cost_cents/100,row.price_cents/100,row.expiry||words.unknown,a.coverage===null?words.unknown:a.coverage,a.suggested===null?words.unknown:a.suggested/100,a.reorder/1000,a.expired?words.expired:a.near?words.near:a.slow?words.slow:words.healthy]);
 return [headers,...table];
}
export function tableCSV(rows,words,locale='pt-BR'){
 const format=new Intl.NumberFormat(locale,{useGrouping:false,maximumFractionDigits:3});
 const cell=value=>'"'+String(typeof value==='number'?format.format(value):value??'').replace(/^([\s]*[=+@-])/u,"'$1").replace(/"/g,'""')+'"';
 return '\uFEFF'+analysisMatrix76(rows,words).map(line=>line.map(cell).join(';')).join('\r\n')+'\r\n';
}
