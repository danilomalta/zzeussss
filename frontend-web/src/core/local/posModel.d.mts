import type { LocalProduct, SaleInput, SaleReceipt } from './localClient.mjs';
export interface CartItem { product: LocalProduct; location_id: string; quantity_milli: number; }
export function isID(value: unknown): boolean;
export function isCents(value: unknown): boolean;
export function parseMoney(value: string): number;
export function parseQuantity(value: string, unit: string): number;
export function lineCents(price: number, quantity: number): number;
export function cartTotal(items: CartItem[]): number;
export function makeSale(items: CartItem[], cashSessionID: string, uuid?: () => string): SaleInput;
export function validSale(input: unknown): boolean;
export function receiptMatches(receipt: SaleReceipt, input: SaleInput): boolean;
