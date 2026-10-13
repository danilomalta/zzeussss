// Summary of one explicitly bounded server page, never the company's full ledger.
export function summarizeOwnerSales(rows){
 if(!Array.isArray(rows)||rows.length>20)throw new Error('Invalid sales page');
 const ids=new Set(),days=new Map();let total=0,completed=0,cancelled=0;
 for(const row of rows){
  if(!row||typeof row.sale_id!=='string'||!row.sale_id||ids.has(row.sale_id)||!['committed','cancelled'].includes(row.status)||!Number.isSafeInteger(row.total_cents)||row.total_cents<0||typeof row.committed_at!=='string'||!/^\d{4}-\d{2}-\d{2}T/.test(row.committed_at)||!Number.isFinite(Date.parse(row.committed_at)))throw new Error('Invalid sale');
  ids.add(row.sale_id);
  if(row.status==='cancelled'){cancelled++;continue;}
  completed++;
  total+=row.total_cents;if(!Number.isSafeInteger(total))throw new Error('Unsafe total');
  const date=new Date(row.committed_at).toLocaleDateString('en-CA',{timeZone:'America/Sao_Paulo',year:'numeric',month:'2-digit',day:'2-digit'});
  days.set(date,(days.get(date)||0)+row.total_cents);
 }
 return {total_cents:total,completed,cancelled,loaded:rows.length,days:[...days].sort(([a],[b])=>a.localeCompare(b)).map(([date,cents])=>({date,cents}))};
}
export function formatOwnerMoney(cents,locale='pt-BR'){
 if(!Number.isSafeInteger(cents)||cents<0)throw new Error('Unsafe money');
 const value=BigInt(cents),formatter=new Intl.NumberFormat(locale,{style:'currency',currency:'BRL',minimumFractionDigits:2,maximumFractionDigits:2});
 return formatter.formatToParts(value/100n).map(part=>part.type==='fraction'?String(value%100n).padStart(2,'0'):part.value).join('');
}
