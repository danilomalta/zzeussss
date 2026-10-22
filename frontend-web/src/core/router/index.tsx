import React, { Suspense } from 'react';
import {AppearanceProvider80} from '../settings80/runtime';
import {useTranslation} from 'react-i18next';
import { Outlet, createBrowserRouter } from 'react-router-dom';
import { OnlineSessionGuard, LocalSessionGuard } from '../local/SessionGuards';

/*
ROTEAMENTO MODULAR COM DIVISÃO DE CÓDIGO (LAZY LOADING)
======================================================
Desenvolvido sob o conceito de Micro-frontends/Code-Splitting, otimizando o 
consumo de memória RAM (importante para hardwares limitados do PDV varejista).
*/

// Carregamento assíncrono e sob demanda de cada página/módulo
import {LocalAccessProvider} from '../local/LocalAccess';
const LocalComparison = React.lazy(() => import('../../modules/catalog/pages/LocalComparison'));
const LocalOrders = React.lazy(() => import("../../modules/catalog/pages/LocalOrders"));
const LocalDistributionMap = React.lazy(() => import('../../modules/tenant/pages/LocalDistributionMap'));
const LocalSalesAnalysis = React.lazy(() => import('../../modules/tenant/pages/LocalSalesAnalysis'));
const LocalOwnerDashboard = React.lazy(() => import('../../modules/tenant/pages/LocalOwnerDashboard'));
const LocalHumanResources = React.lazy(() => import('../../modules/tenant/pages/LocalHumanResources'));
const LocalSpreadsheet = React.lazy(() => import('../../modules/tenant/pages/LocalSpreadsheet'));
const LocalWorkbench73 = React.lazy(() => import('../../modules/tenant/pages/LocalWorkbench73'));
const LocalIntegrations74 = React.lazy(() => import('../../modules/tenant/pages/LocalIntegrations74'));
const LocalEnterprise89 = React.lazy(() => import('../../modules/tenant/pages/LocalEnterprise89'));
const LocalOperation90 = React.lazy(() => import('../../modules/tenant/pages/LocalOperation90'));
const LocalSettings80 = React.lazy(() => import('../../modules/tenant/pages/LocalSettings80'));
const LocalBusiness85 = React.lazy(() => import('../../modules/tenant/pages/LocalBusiness85'));
const LocalSectors86 = React.lazy(() => import('../../modules/tenant/pages/LocalSectors86'));
const LocalOperations84 = React.lazy(() => import('../../modules/tenant/pages/LocalOperations84'));
const LocalHome = React.lazy(() => import('../../modules/tenant/pages/LocalMainMenu'));
const LocalSubscription = React.lazy(() => import('../../modules/tenant/pages/LocalSubscription'));
const LocalStaff = React.lazy(() => import('../../modules/tenant/pages/LocalStaff'));
const Registration = React.lazy(() => import('../../modules/auth/pages/Registration'));
const LocalWorkspace = React.lazy(() => import('../../shared/brand/LocalWorkspace'));
const LocalLogin = React.lazy(() => import('../../modules/auth/pages/LocalLogin'));
const LocalCatalog = React.lazy(() => import('../../modules/catalog/pages/LocalCatalog'));
const LocalPointOfSale = React.lazy(() => import('../../modules/pos/pages/LocalPointOfSale'));
const Login = React.lazy(() => import('../../modules/auth/pages/Login'));
const PointOfSale = React.lazy(() => import('../../modules/pos/pages/PointOfSale'));
const Checkout = React.lazy(() => import('../../modules/financial/pages/Checkout'));
const LocalPayables113 = React.lazy(() => import('../../modules/financial/pages/LocalPayables113'));
const LocalPayablesAgenda114 = React.lazy(() => import('../../modules/financial/pages/LocalPayablesAgenda114'));
const SupplyChain = React.lazy(() => import('../../modules/catalog/pages/SupplyChain'));
const AdminDashboard = React.lazy(() => import('../../modules/tenant/pages/AdminDashboard'));

// Wrapper reutilizável de Suspense para renderizar um estado de carregamento leve
const SuspenseWrapper = ({children}:{children:React.ReactNode}) => {
  const {t} = useTranslation();
  return <Suspense fallback={<div className="flex h-screen items-center justify-center text-base" style={{background:'var(--bg)',color:'var(--text-muted)'}}>{t('common.loading')}</div>}>{children}</Suspense>;
};

const LocalLossPrevention76 = React.lazy(() => import('../../modules/tenant/pages/LocalLossPrevention76'));

// Definição da árvore de rotas modularizada do TitanSystem
export const router = createBrowserRouter([
  { path: '/register', element: <SuspenseWrapper><Registration /></SuspenseWrapper> },
  { path: '/local/login', element: <SuspenseWrapper><LocalLogin /></SuspenseWrapper> },
  { path: '/local', element: <LocalSessionGuard><LocalAccessProvider><SuspenseWrapper><AppearanceProvider80><LocalWorkspace /></AppearanceProvider80></SuspenseWrapper></LocalAccessProvider></LocalSessionGuard>, children: [
    { path: 'prices', element: <SuspenseWrapper><LocalComparison /></SuspenseWrapper> },
    { path: "orders", element: <SuspenseWrapper><LocalOrders /></SuspenseWrapper> },
    { index: true, element: <SuspenseWrapper><LocalHome /></SuspenseWrapper> },
    { path: 'home', element: <SuspenseWrapper><LocalHome /></SuspenseWrapper> },
    { path: 'losses', element: <SuspenseWrapper><LocalLossPrevention76 /></SuspenseWrapper> },
    { path: 'integrations', element: <SuspenseWrapper><LocalEnterprise89 initialArea="connections" /></SuspenseWrapper> },
    { path: 'workbench', element: <SuspenseWrapper><LocalWorkbench73 /></SuspenseWrapper> },
    { path: 'reports', element: <SuspenseWrapper><LocalSalesAnalysis /></SuspenseWrapper> },
    { path: 'overview', element: <SuspenseWrapper><LocalOwnerDashboard /></SuspenseWrapper> },
    { path: 'owner', element: <SuspenseWrapper><LocalOwnerDashboard /></SuspenseWrapper> },
    { path: 'map', element: <SuspenseWrapper><LocalDistributionMap /></SuspenseWrapper> },
    { path: 'settings', element: <SuspenseWrapper><LocalSettings80 /></SuspenseWrapper> },
    { path: 'operation', element: <SuspenseWrapper><LocalOperation90 /></SuspenseWrapper> },
    { path: 'integrations/diagnostics', element: <SuspenseWrapper><LocalIntegrations74 /></SuspenseWrapper> },
    { path: 'enterprise', element: <SuspenseWrapper><LocalEnterprise89 /></SuspenseWrapper> },
    { path: 'payables', element: <SuspenseWrapper><LocalPayables113 /></SuspenseWrapper> },
    { path: 'payables/agenda', element: <SuspenseWrapper><LocalPayablesAgenda114 /></SuspenseWrapper> },
    { path: 'delivery', element: <SuspenseWrapper><LocalSectors86 initialArea="delivery" /></SuspenseWrapper> },
    { path: 'recipes', element: <SuspenseWrapper><LocalSectors86 initialArea="recipes" /></SuspenseWrapper> },
    { path: 'sectors', element: <SuspenseWrapper><LocalSectors86 initialArea="sectors" /></SuspenseWrapper> },
    { path: 'food/previous', element: <SuspenseWrapper><LocalBusiness85 initialArea="food" /></SuspenseWrapper> },
    { path: 'fleet', element: <SuspenseWrapper><LocalBusiness85 initialArea="fleet" /></SuspenseWrapper> },
    { path: 'food', element: <SuspenseWrapper><LocalSectors86 initialArea="food" /></SuspenseWrapper> },
    { path: 'documents', element: <SuspenseWrapper><LocalBusiness85 initialArea="documents" /></SuspenseWrapper> },
    { path: 'purchasing', element: <SuspenseWrapper><LocalBusiness85 initialArea="purchasing" /></SuspenseWrapper> },
    { path: 'commercial', element: <SuspenseWrapper><LocalBusiness85 initialArea="commercial" /></SuspenseWrapper> },
    { path: 'operations', element: <SuspenseWrapper><LocalOperations84 /></SuspenseWrapper> },
    { path: 'stock', element: <SuspenseWrapper><LocalCatalog /></SuspenseWrapper> },
    { path: 'subscription', element: <SuspenseWrapper><LocalSubscription /></SuspenseWrapper> },
    { path: 'sheets', element: <SuspenseWrapper><LocalSpreadsheet /></SuspenseWrapper> },
    { path: 'hr', element: <SuspenseWrapper><LocalHumanResources /></SuspenseWrapper> },
    { path: 'staff', element: <SuspenseWrapper><LocalStaff /></SuspenseWrapper> },
    { path: 'catalog', element: <SuspenseWrapper><LocalCatalog /></SuspenseWrapper> },
    { path: 'pos', element: <SuspenseWrapper><LocalPointOfSale /></SuspenseWrapper> },
    { path: 'cash', element: <SuspenseWrapper><LocalOperations84 initialArea="closing" /></SuspenseWrapper> },
    { path: 'cash/register', element: <SuspenseWrapper><LocalPointOfSale mode="cash" /></SuspenseWrapper> },
  ] },
  {
    path: '/login',
    element: (
      <SuspenseWrapper>
        <Login />
      </SuspenseWrapper>
    ),
  },
  {
    path: '/pos',
    element: (
      <SuspenseWrapper>
        <OnlineSessionGuard><PointOfSale /></OnlineSessionGuard>
      </SuspenseWrapper>
    ),
  },
  {
    path: '/checkout',
    element: (
      <SuspenseWrapper>
        <OnlineSessionGuard><Checkout /></OnlineSessionGuard>
      </SuspenseWrapper>
    ),
  },
  {
    path: '/',
    element: <OnlineSessionGuard><Outlet /></OnlineSessionGuard>,
    children: [
      {
        path: '',
        element: (
          <SuspenseWrapper>
            <AdminDashboard />
          </SuspenseWrapper>
        ),
      },
      {
        path: 'logistics',
        element: (
          <SuspenseWrapper>
            <SupplyChain />
          </SuspenseWrapper>
        ),
      },
    ]
  }
]);
