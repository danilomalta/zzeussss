import { api } from './api';

/*
SERVIÇO DE DASHBOARD (REQUISICÕES HTTP REAIS)
=============================================
Camada de comunicação para carregamento de dados reais do backend (PostgreSQL).
*/

export interface DashboardMetrics {
  totalSales: number;
  activeClients: number;
  totalCosts: number;
  netProfit: number;
  criticalIssues: number;
  averageUptime: string;
  totalProducts: number; // Adicionado para suportar o card do catálogo real
}

export interface RecentSale {
  id: string;
  clientName: string;
  amount: number;
  status: 'completed' | 'pending' | 'failed';
  createdAt: string;
}

export const dashboardService = {
  /**
   * Busca as métricas consolidadas do painel (faturamento, clientes ativos, custos, uptime e total de produtos)
   */
  getMetrics: async (): Promise<DashboardMetrics> => {
    try {
      const response = await api.get<DashboardMetrics>('/metrics/overview');
      return response.data;
    } catch (error) {
      console.error('Erro ao requisitar /metrics/overview:', error);
      throw error;
    }
  },

  /**
   * Busca a lista de vendas recentes realizadas no PDV
   */
  getRecentSales: async (): Promise<RecentSale[]> => {
    try {
      const response = await api.get<RecentSale[]>('/pos/sales/recent');
      return response.data;
    } catch (error) {
      console.error('Erro ao requisitar /pos/sales/recent:', error);
      throw error;
    }
  },
};
