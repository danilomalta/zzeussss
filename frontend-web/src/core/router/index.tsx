import React, { Suspense } from 'react';
import { Outlet, createBrowserRouter } from 'react-router-dom';

/*
ROTEAMENTO MODULAR COM DIVISÃO DE CÓDIGO (LAZY LOADING)
======================================================
Desenvolvido sob o conceito de Micro-frontends/Code-Splitting, otimizando o 
consumo de memória RAM (importante para hardwares limitados do PDV varejista).
*/

// Carregamento assíncrono e sob demanda de cada página/módulo
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

import Layout from '../../shared/components/Layout';

// Definição da árvore de rotas modularizada do TitanSystem
export const router = createBrowserRouter([
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
        <PointOfSale />
      </SuspenseWrapper>
    ),
  },
  {
    path: '/checkout',
    element: (
      <SuspenseWrapper>
        <Checkout />
      </SuspenseWrapper>
    ),
  },
  {
    path: '/',
    element: (
      <Layout>
        <Outlet />
      </Layout>
    ),
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
