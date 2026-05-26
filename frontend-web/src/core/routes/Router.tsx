import { BrowserRouter, Routes, Route } from 'react-router-dom';

/*
ROUTER GLOBAL (ESQUELETO - FSD)
===============================
Define a estrutura de navegação entre os módulos principais do TitanSystem:
- Autenticação
- PDV (Frente de Caixa)
- ERP/Backoffice (Checkout, Faturamento)
- Inteligência Logística B2B (Supply Chain)
*/

import Login from '../../modules/auth/pages/Login';
import PointOfSale from '../../modules/pos/pages/PointOfSale';
import Checkout from '../../modules/financial/pages/Checkout';
import SupplyChain from '../../modules/catalog/pages/SupplyChain';

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
