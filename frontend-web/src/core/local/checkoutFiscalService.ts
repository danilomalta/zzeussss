import {createCheckoutFiscalClient} from './checkoutFiscal.mjs';
import {useLocalSession} from './useLocalSession';

export const checkoutFiscalClient=createCheckoutFiscalClient({getCurrent:useLocalSession.getState});
