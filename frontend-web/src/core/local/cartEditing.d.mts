import type {CartItem} from './posModel.mjs';
import type {LocalProduct} from './localClient.mjs';
export function addCartProduct(cart:CartItem[],product:LocalProduct,locationID:string,quantity:string):CartItem[];
export function replaceCartQuantity(cart:CartItem[],index:number,value:string):CartItem[];
export function restoreCartItem(cart:CartItem[],item:CartItem):CartItem[];
export function selectCatalogProduct(products:LocalProduct[],search:string,index?:number):LocalProduct;
