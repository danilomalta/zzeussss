import test from 'node:test';
import assert from 'node:assert/strict';
import {plans, recommendedModule, planTotals, formatMoney} from '../src/modules/auth/pages/registrationModel.mjs';

test('varejo recomenda apenas PDV e conserva preço existente', () => {
  const recommended = recommendedModule('Varejo');
  assert.equal(recommended, 'pos');
  assert.deepEqual(planTotals([recommended]), {monthly:35000,savings:0});
});
test('cada atividade tem recomendação própria; atividade desconhecida não contrata nada', () => {
  assert.equal(recommendedModule('Produção / indústria'), 'production');
  assert.equal(recommendedModule('Escritório de RH'), 'staff');
  assert.equal(recommendedModule('Contabilidade / contador'), 'accounting');
  assert.equal(recommendedModule('Desconhecida'), null);
});
test('total e economia contam apenas módulos conhecidos selecionados, sem duplicatas', () => {
  assert.deepEqual(planTotals(['pos','production','pos','inexistente']), {monthly:285000,savings:100000});
  assert.deepEqual(planTotals([]), {monthly:0,savings:0});
  assert.deepEqual(planTotals(['accounting']), {monthly:99900,savings:50100});
  assert.ok(plans.every(plan => Number.isSafeInteger(plan.price) && Number.isSafeInteger(plan.previous)));
});
test('formatação monetária exibe centavos em pt-BR', () => {
  assert.match(formatMoney(35000), /350,00/);
  assert.match(formatMoney(99901), /999,01/);
});
