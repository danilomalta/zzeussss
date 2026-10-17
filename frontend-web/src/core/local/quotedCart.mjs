// A quote has no payment side effects. Pending sale recovery must continue
// through posOperations, never through a fresh quote or a newly generated ID.
export async function quoteCart(client, token, cart, cashSessionID) {
  const quote = await client.quoteSale(token, {
    cash_session_id: cashSessionID,
    items: cart.map(item => ({product_id: item.product.id, location_id: item.location_id, quantity_milli: item.quantity_milli})),
  });
  const refreshed = cart.map((item, index) => ({...item, product: {...item.product,
    unit: quote.items[index].unit, price_cents: quote.items[index].unit_price_cents}}));
  const changed = cart.some((item, index) => item.product.price_cents !== refreshed[index].product.price_cents || item.product.unit !== refreshed[index].product.unit);
  return {cart: refreshed, total_cents: quote.total_cents, changed};
}

// Called only for a NEW cash sale. A pending sale must use posOperations.retry.
// Confirmation/tender persistence is never reached after a failed/stale quote.
export async function confirmCashQuote(client, token, cart, cashSessionID, received, onChanged, confirm) {
  const quoted = await quoteCart(client, token, cart, cashSessionID);
  if (quoted.changed) {
    onChanged(quoted.cart);
    throw new Error('O catálogo mudou. Os preços foram atualizados; confira o total e confirme novamente.');
  }
  if (!Number.isSafeInteger(received) || received < quoted.total_cents) {
    throw new Error('O valor recebido não cobre o total conferido pelo servidor.');
  }
  return confirm(quoted.cart);
}
