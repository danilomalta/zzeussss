import type {createLocalClient} from './localClient.mjs';
import type {CartItem} from './posModel.mjs';
export function quoteCart(client: Pick<ReturnType<typeof createLocalClient>, 'quoteSale'>, token: string, cart: CartItem[], cashSessionID: string): Promise<{cart: CartItem[]; total_cents: number; changed: boolean}>;
export function confirmCashQuote<T>(client: Pick<ReturnType<typeof createLocalClient>, 'quoteSale'>, token: string, cart: CartItem[], cashSessionID: string, received: number, onChanged: (cart: CartItem[]) => void, confirm: (cart: CartItem[]) => Promise<T>): Promise<T>;
