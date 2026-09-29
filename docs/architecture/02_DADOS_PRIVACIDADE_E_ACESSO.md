# TitanSystem — dados, privacidade e permissões (Fase 1)

**Estado:** proposta arquitetural, sem alegar criptografia, recuperação ou isolamento já implementados. A empresa é responsável por seus dados de operação; cada compartilhamento entre organizações precisa ser concedido, registrado e revogável.

## Matriz de dados

`Local` indica a cópia operacional primária prevista. `Relay` indica apenas o mínimo necessário para transporte ou reserva; uma cópia cifrada opcional não dá acesso automático ao operador Titan.

| Dado | Titular operacional | Local primário | Destinatário autorizado | Visibilidade mínima do serviço Titan |
|---|---|---|---|---|
| Vendas, itens, pagamentos, caixa | Mercado | Aparelho/instalação do mercado | Gerência; contador com concessão | ID de mensagem, destino, estado e tamanho; conteúdo cifrado se houver relay |
| Catálogo, preço, custo, saldos por local | Mercado | Instalação e dispositivos autorizados | Equipe conforme função; fornecedor apenas dados oferecidos no pedido | Metadados de roteamento; nenhum custo ou saldo livre |
| Sugestão de reposição | Mercado | Mercado | Gerente/comprador | Nenhum conteúdo obrigatório antes da aprovação |
| Pedido aprovado e entrega | Mercado e fornecedor, cada qual com seu registro | Cópia em cada organização | Contraparte vinculada; contador por concessão | Identificadores de vínculo, envio e estado; conteúdo cifrado |
| Ocupação, janela e reserva de doca | Organização que opera a doca | Agenda local e serviço de reserva autorizado | Mercado e fornecedor envolvidos | Doca, janela, capacidade, estado, IDs e horários necessários para impedir sobreposição |
| Lotes, etapas, perdas e qualidade | Fornecedor/produtor | Instalação do fornecedor | Equipe; mercado apenas dados acordados de entrega/rastreio | Metadados de envio, se necessário |
| Ponto, tarefas, metas e avaliações | Empresa empregadora | Dispositivo autorizado e instalação da empresa | Funcionário vê os próprios dados permitidos; gerente/RH conforme função | Nenhum conteúdo livre; telemetria mínima sem avaliação |
| XML e documentos contábeis | Empresa emissora/recebedora | Instalação e backup escolhido pela empresa | Contador convidado com escopo e prazo | Arquivo cifrado no relay opcional; metadados necessários ao transporte |
| Licença e suporte | Cliente e operador Titan | Serviço Titan | Dono da conta e equipe Titan autorizada | Conta, plano, vencimento, versão, dispositivos e registros técnicos necessários |

`Titular operacional` aqui é uma regra de produto, não uma determinação jurídica sobre titularidade de dados pessoais. Obrigações legais e fiscais serão especificadas separadamente antes de produção.

## Limite honesto da promessa de privacidade

O serviço Titan **não pode prometer conhecer absolutamente nada**: para licenças, entrega de mensagens e agendamento ele precisa processar alguns identificadores, estados, horários, tamanho de mensagens, endereços de conexão e dados de cobrança. Ele não deve ter **acesso livre ao conteúdo comercial** por padrão. Relatórios globais, suporte remoto, contador e fornecedor só recebem o escopo autorizado pelo cliente. Não expor conteúdo em logs ou telemetria.

## Compartilhamento e consentimento operacional

- Uma aprovação de reposição cria mensagem específica para um fornecedor explicitamente vinculado. O fornecedor não recebe o catálogo inteiro nem outras vendas do mercado.
- O mercado concede ao contador acesso a empresas, lojas, períodos e tipos de documentos definidos; registra concessão, uso e revogação. Revogação bloqueia acesso futuro, mas não desfaz cópias legais já recebidas.
- Avaliação de funcionário é privada: gerente/RH autorizados podem registrar e consultar conforme política; nem revendedor nem outros funcionários recebem esse conteúdo.
- Suporte excepcional que exija conteúdo depende de ação visível do responsável pela empresa, escopo mínimo, duração e auditoria; nunca de uma chave mestra Titan invisível.

## Chaves, dispositivo novo e backup — decisão de desenho

**Recomendação:** dados locais cifrados em repouso onde a plataforma permitir; mensagens sensíveis cifradas ponta a ponta entre organizações/dispositivos autorizados. Cada instalação possui identidade criptográfica própria. Chaves privadas de conteúdo ficam sob controle do cliente ou em serviço por ele escolhido. Titan pode guardar chaves públicas e ciphertext, sem chave mestra para ler tudo.

1. Um administrador autenticado aprova o pareamento de aparelho novo; código temporário sozinho não concede acesso. A transferência de chaves é protegida, vinculada ao dispositivo e auditada.
2. Rotação e revogação impedem acesso futuro. Aparelho perdido exige revogar credenciais e trocar as chaves afetadas; eventos locais ainda não enviados podem ser perdidos sem backup.
3. Backup cifrado é periódico e verificável. O cliente escolhe onde guardá-lo e mantém material de recuperação separado. Restaurar em aparelho novo exige autorização e teste de integridade; não duplicar identidade do dispositivo antigo.
4. Se o cliente perder todas as chaves e cópias de recuperação, Titan não deve prometer restaurar conteúdo inacessível. Qualquer opção de custódia assistida muda a promessa de privacidade e exige escolha explícita.

**Estado atual:** essas medidas são requisitos futuros. O repositório não demonstrou criptografia ponta a ponta, pareamento ou backup recuperável na Fase 0.

## Papéis mínimos propostos

Todos os papéis são vinculados a uma organização; um usuário de mais de uma empresa escolhe contexto autorizado. A API verifica a sessão e o vínculo em cada operação.

| Papel | Pode fazer | Não pode fazer apenas por esse papel |
|---|---|---|
| Dono/admin do mercado | Configurar lojas, convites, políticas, compartilhamentos e auditoria | Ler dados de outra organização |
| Gerente do mercado | Aprovar reposição e desconto na alçada, conferir recebimento, gerir equipe | Alterar registros auditados sem evento compensatório |
| Caixa/vendedor | Vender, consultar itens necessários e seu turno | Aprovar a própria exceção, acessar avaliações ou custos completos |
| Estoquista/comprador | Contar locais de estoque, propor/comprar dentro da alçada, conferir mercadoria | Confirmar pagamento bancário sem integração/autorização |
| Funcionário | Marcar o próprio ponto e consultar suas tarefas/metas | Ver avaliações de colegas ou conceder privilégios |
| Admin do fornecedor | Aceitar pedidos, atribuir produção, expedir e administrar sua doca | Ver vendas ou estoque completo do mercado |
| Produção/logística | Registrar etapa, perda, lote e entrega autorizada | Consultar finanças de outra empresa |
| Contador convidado | Consultar/exportar período e documentos concedidos | Administrar loja, alterar venda ou obter dados não concedidos |
| Operador Titan | Licença, encaminhamento e suporte técnico restrito | Abrir conteúdo comercial, avaliações, XMLs ou chaves privadas por padrão |

Permissões granulares, alçadas, negação explícita, revogação e testes de acesso cruzado entram na implementação da Fase 3. O frontend sozinho não concede nem restringe autoridade.
