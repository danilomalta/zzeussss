import {createContext,useContext} from 'react';
import type {LocalCapabilities} from '../src/core/local/localClient.mjs';
export const fixture89:LocalCapabilities={tenant_id:'DEMO-COMPANY',store_id:'DEMO-STORE',device_id:'DEMO-PC',identity_id:'DEMO-USER',role:'owner',permissions:['manage_cash','sell','manage_stock','manage_production','view_orders','view_catalog','manage_staff','view_accounting'],license:{state:'active',remaining_days:30,modules:['core','pos','inventory','production','orders','staff','accounting','logistics','finance','fiscal']}};
export const MockContext=createContext<{capabilities:LocalCapabilities;refresh:()=>void}>({capabilities:fixture89,refresh:()=>{}});
export const useLocalAccess=()=>useContext(MockContext);
