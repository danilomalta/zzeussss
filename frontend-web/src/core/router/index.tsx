import React, { Suspense } from 'react';
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
const LocalStockAvailability = React.lazy(() => import('../../modules/catalog/pages/LocalStockAvailability'));
const LocalReceivingHistory = React.lazy(() => import('../../modules/catalog/pages/LocalReceivingHistory'));
const LocalProduction = React.lazy(() => import('../../modules/production/pages/LocalProduction'));
const LocalComparison = React.lazy(() => import('../../modules/catalog/pages/LocalComparison'));
const LocalOrders = React.lazy(() => import("../../modules/catalog/pages/LocalOrders"));
const LocalHome = React.lazy(() => import('../../modules/tenant/pages/LocalHome'));
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
const SupplyChain = React.lazy(() => import('../../modules/catalog/pages/SupplyChain'));
const AdminDashboard = React.lazy(() => import('../../modules/tenant/pages/AdminDashboard'));

// Wrapper reutilizável de Suspense para renderizar um estado de carregamento leve
const SuspenseWrapper = ({ children }: { children: React.ReactNode }) => (
  <Suspense
    fallback={
      <div className="flex h-screen items-center justify-center bg-slate-900 text-indigo-500 font-medium">
        Carregando módulo...
      </div>
    }
  >
    {children}
  </Suspense>
);

// Definição da árvore de rotas modularizada do TitanSystem
export const router = createBrowserRouter([
  { path: '/register', element: <SuspenseWrapper><Registration /></SuspenseWrapper> },
  { path: '/local/login', element: <SuspenseWrapper><LocalLogin /></SuspenseWrapper> },
  { path: '/local', element: <LocalSessionGuard><LocalAccessProvider><SuspenseWrapper><LocalWorkspace /></SuspenseWrapper></LocalAccessProvider></LocalSessionGuard>, children: [
    { path: 'receiving', element: <SuspenseWrapper><LocalReceivingHistory /></SuspenseWrapper> },
    { path: 'production', element: <SuspenseWrapper><LocalProduction /></SuspenseWrapper> },
    { path: 'prices', element: <SuspenseWrapper><LocalComparison /></SuspenseWrapper> },
    { path: "orders", element: <SuspenseWrapper><LocalOrders /></SuspenseWrapper> },
    { index: true, element: <SuspenseWrapper><LocalHome /></SuspenseWrapper> },
    { path: 'home', element: <SuspenseWrapper><LocalHome /></SuspenseWrapper> },
    { path: 'stock', element: <SuspenseWrapper><LocalStockAvailability /></SuspenseWrapper> },
    { path: 'subscription', element: <SuspenseWrapper><LocalSubscription /></SuspenseWrapper> },
    { path: 'staff', element: <SuspenseWrapper><LocalStaff /></SuspenseWrapper> },
    { path: 'catalog', element: <SuspenseWrapper><LocalCatalog /></SuspenseWrapper> },
    { path: 'pos', element: <SuspenseWrapper><LocalPointOfSale /></SuspenseWrapper> },
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
