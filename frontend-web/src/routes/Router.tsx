import React from 'react';
import { BrowserRouter, Routes, Route } from 'react-router-dom';

/*
ROUTER GLOBAL (ESQUELETO)
=========================
Define a estrutura de navegação entre os módulos principais do TitanSystem:
- Autenticação
- PDV (Frente de Caixa)
- ERP/Backoffice (Checkout, Faturamento)
- Inteligência Logística B2B (Supply Chain)
*/

import Login from '../pages/Auth/Login';
import PointOfSale from '../pages/POS/PointOfSale';
import Checkout from '../pages/Payment/Checkout';
import SupplyChain from '../pages/Logistics/SupplyChain';

const Router = () => {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/" element={<Login />} />
        <Route path="/pos" element={<PointOfSale />} />
        <Route path="/checkout" element={<Checkout />} />
        <Route path="/logistics" element={<SupplyChain />} />
      </Routes>
    </BrowserRouter>
  );
};

export default Router;
