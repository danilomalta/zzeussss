# TitanSystem: Alertas Automáticos

**O que você vai codar aqui**: 
Conectores de envio de mensagens Telegram/Email/WhatsApp indicando "Servidor do cliente XYZ caiu".

**Linguagem**: Go ou Node.js (Serviço Cron/Worker).

**Processo e Regras**:
1. Roda a cada minuto verificando a tabela de logs e métricas. Se CPU > 90% ou erro fatal ocorrer no SPED, avise.

**Barreira de Segurança Contras Vazamentos**:
* Senha ou Token API do Telegram/SendGrid do Alerta NÃO pode estar no código, é 100% `os.Getenv()`.
