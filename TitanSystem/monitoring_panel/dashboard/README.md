# TitanSystem: Dashboard Olho de Deus (Monitoramento de Saúde)

**O que você vai codar aqui**: 
Gráficos e telas de monitoramento que puxam dados de ping, uso de CPU, disco e logs do backend e servidores dos clientes.

**Linguagem**: React (TS) ou Grafana embebed (Processo de observabilidade).

**Processo e Regras**:
1. Mostrar em tempo real (via WebSocket ou Server-Sent Events) quais PDVs estão logados.
2. Monitorar os 3 tipos de ambiente de banco (Cloud, On-Premise e SQLite Local).

**Barreira de Segurança Contras Vazamentos**:
* Apenas IP do Dono/Admins têm permissão de acessar este painel.
* Login por Auth de Dois Fatores (OTP).
* Excesso de informações sensíveis nos logs ("Fulano comprou 2 remédios") deve ser omitida deste log, mostrando apenas metadados técnicos ("Carga na rota /venda foi concluída").
