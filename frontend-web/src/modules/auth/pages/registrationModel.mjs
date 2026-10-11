export const plans = [
  {id:'pos', name:'PDV e varejo', price:35000, previous:0, description:'Frente de caixa e estoque para venda no varejo.'},
  {id:'production', name:'Empresa e produção', price:250000, previous:350000, description:'Planejamento de produção e estoque. PDV contratado separadamente.'},
  {id:'staff', name:'Escritório e RH', price:250000, previous:350000, description:'Funcionários, ponto e gestão de pessoas, conforme as permissões da empresa.'},
  {id:'accounting', name:'Contabilidade e contador', price:99900, previous:150000, description:'Área contábil separada, com acesso autorizado às informações.'},
];
export function recommendedModule(activity) {
  return ({'Varejo':'pos','Produção / indústria':'production','Escritório de RH':'staff','Contabilidade / contador':'accounting'})[activity] ?? null;
}
export function planTotals(selected) {
  const chosen = plans.filter(plan => selected.includes(plan.id));
  return {monthly:chosen.reduce((sum, plan) => sum + plan.price, 0), savings:chosen.reduce((sum, plan) => sum + Math.max(0, plan.previous - plan.price), 0)};
}
export function formatMoney(cents, locale = 'pt-BR') {
  return new Intl.NumberFormat(locale, {style:'currency', currency:'BRL'}).format(cents / 100);
}
