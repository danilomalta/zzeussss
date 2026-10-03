const max = BigInt(Number.MAX_SAFE_INTEGER);
export const isID = (value) => typeof value === 'string' && value.trim() === value && value.length > 0 && value.length <= 128;
export const isCents = (value) => Number.isSafeInteger(value) && value >= 0;
export function checkedNumber(value) {
  if (value < 0n || value > max) throw new Error('Valor fora do intervalo suportado.');
  return Number(value);
}
function decimal(value, places) {
  if (String(value).length > 32) throw new Error('Número fora do intervalo suportado.');
  const match = String(value).trim().match(new RegExp(`^(\\d+)(?:[,.](\\d{1,${places}}))?$`));
  if (!match) throw new Error(`Informe um número positivo com até ${places} casas decimais, sem separador de milhar.`);
  return checkedNumber(BigInt(match[1]) * 10n ** BigInt(places) + BigInt((match[2] || '').padEnd(places, '0')));
}
export const parseMoney = (value) => decimal(value, 2);
export function parseQuantity(value, unit) {
  const quantity = decimal(value, 3);
  if (quantity <= 0 || (unit === 'unit' && quantity % 1000 !== 0)) throw new Error('Informe uma quantidade válida; produtos por unidade exigem quantidade inteira.');
  return quantity;
}
export function lineCents(price, quantity) {
  if (!isCents(price) || !Number.isSafeInteger(quantity) || quantity <= 0) throw new Error('Preço ou quantidade inválidos.');
  return checkedNumber((BigInt(price) * BigInt(quantity) + 500n) / 1000n);
}
export function cartTotal(items) {
  return checkedNumber(items.reduce((sum, item) => sum + BigInt(lineCents(item.product.price_cents, item.quantity_milli)), 0n));
}
export function validSale(input) {
  return input && [input.operation_id, input.sale_id, input.cash_session_id].every(isID) &&
    Array.isArray(input.items) && input.items.length > 0 && input.items.length <= 500 &&
    input.items.every((item) => item && isID(item.product_id) && isID(item.location_id) && Number.isSafeInteger(item.quantity_milli) && item.quantity_milli > 0) &&
    Array.isArray(input.payments) && input.payments.length === 1 && input.payments[0]?.method === 'cash' && isCents(input.payments[0].amount_cents) && input.payments[0].amount_cents > 0 &&
    new TextEncoder().encode(JSON.stringify(input)).length <= 128 * 1024;
}
export function makeSale(items, cashSessionID, uuid = () => crypto.randomUUID()) {
  const total = cartTotal(items);
  const input = { operation_id: uuid(), sale_id: uuid(), cash_session_id: cashSessionID,
    items: items.map((item) => ({ product_id: item.product.id, location_id: item.location_id, quantity_milli: item.quantity_milli })),
    payments: [{ method: 'cash', amount_cents: total }] };
  if (!validSale(input)) throw new Error('Carrinho inválido ou total igual a zero.');
  return input;
}
export function receiptMatches(receipt, input) {
  const keys = (items) => items.map((item) => JSON.stringify([item.product_id, item.location_id, item.quantity_milli])).sort();
  return receipt.sale_id === input.sale_id && receipt.operation_id === input.operation_id && receipt.cash_session_id === input.cash_session_id &&
    ['committed', 'cancelled'].includes(receipt.status) && receipt.total_cents === input.payments[0].amount_cents &&
    JSON.stringify(keys(receipt.items)) === JSON.stringify(keys(input.items)) && receipt.payments.length === 1 &&
    receipt.payments[0].method === 'cash' && receipt.payments[0].status === 'confirmed' && receipt.payments[0].amount_cents === receipt.total_cents;
}
