# Agenda local de docas e recebimentos — 22/10/2026

Entrega de backend após Inventário/Custos R2 e Gestão/Produção. Não altera
frontend, Electron, pagamentos, emissão fiscal ou escrita oficial de estoque.

## Fluxo implementado

1. Dono/gerente com `manage_stock` e `view_orders` cadastra docas, renomeia e
   ativa/desativa com revisão esperada. Não permite desativar doca com reservas,
   chegadas ou descargas em aberto, mesmo antigas; é necessária resolução humana.
2. Responsável com ambas permissões reserva uma janela LOCAL para pedido local
   existente e fornecedor ativo. O fornecedor vem do pedido, não do cliente HTTP.
   Pedido cancelado não admite nova reserva/chegada/início/reagendamento.
3. Janelas usam [início,fim), precisão de segundo, RFC3339 convertido para UTC;
   duração de5minutos a8horas. Um fim pode coincidir com o início seguinte.
   Há uma reserva ativa por pedido e uma descarga ativa por doca.
4. `reserved → arrived → unloading → completed`. `reserved → no_show` só após o
   fim programado e por comando humano. Cancelamento local de `reserved` ou
   `arrived`; reagendamento só de `reserved`, inclusive mudança de doca.
5. Atraso não libera uma descarga. Outra não pode iniciar na mesma doca enquanto
   houver `unloading`, mesmo que as janelas previstas sejam adjacentes ou antigas.
   Uma reserva também não expira/libera por timeout: exige ação explícita.
6. Detalhe fornece atraso de chegada (negativo quando antecipada), espera e
   duração de descarga, com dados ausentes null. Horários reais são do servidor,
   não enviados pelo aparelho; durações de espera/descarga são inteiras em segundos.
7. Ocupação atual independe da janela planejada: mostra descarga ativa e número
   de veículos marcados como chegados, inclusive atrasados de dias anteriores.
8. Lista paginada por janela, doca e estado; histórico paginado de doca/reserva;
   CSV da página; recibo imutável por operação, vinculado ao usuário/aparelho.

Não são confirmados: fornecedor, motorista, entrega comercial, disponibilidade
de rota, horário garantido do caminhão, quantidade conferida, entrada de estoque,
devolução, pagamento ou nota fiscal. `supplier_confirmed=false` e
`stock_received=false` são explícitos. Concluir descarga NÃO conclui recebimento
conferido. Cancelar agenda NÃO cancela pedido ou estorna qualquer valor.

## Persistência e segurança

Migração incremental:46 sobre base local de produção45;55 sobre B13+R2/54.
Todas as migrações anteriores permanecem intactas, com checksum verificado.
Tabelas: recursos, reservas, recibos e eventos. Relações carregam empresa/loja,
pedido, fornecedor, usuário e aparelho; índices de lista e unicidade parcial.

Toda escrita valida sessão/usuário/aparelho e permissões atuais, exige contrato
assinado do módulo `logistics` com dependências existentes, e executa mudança,
recibo, evento e outbox na mesma transação. O módulo não se ativa sozinho.
Cadastro de doca exige também papel dono/gerente. Operação idêntica reenviada
retorna recibo original; mesmo ID com outro conteúdo/autor é conflito. Erro ou
escrita ignorada reverte tudo. Não apaga registros nem usa última gravação vence.

A validação de sobreposição está dentro da transação de escrita autorizada:
o contrato atualiza seu relógio persistido antes da observação/escrita da agenda,
obtendo o lock de escritor SQLite. Índices únicos protegem pedido ativo e uso
físico simultâneo. Corrida de conexões independentes não pode confirmar duas
reservas; uma pode receber conflito ou erro de bloqueio transitório, devendo
consultar seu recibo antes de tentar novamente com o MESMO identificador.
Não há retentativa com novo ID automática, eleição de coordenador ou reserva
offline distribuída. Alteração SQL fora do backend é fora deste protocolo.

Histórico/detalhe/recibos requerem permissões e aparelho atuais, mas não renovação
de licença para recuperar dados já existentes. Token revogado não ganha acesso.
Nenhum relatório registra dados de motorista/cliente ou credenciais em console.
`Cache-Control:no-store`; CSV protege campos textuais contra fórmulas.

Se pedido for cancelado após uma reserva, a agenda continua bloqueando o horário
até decisão local. O detalhe sinaliza cancelamento; novas ações operacionais
de chegada/início/reagendamento falham. Se já estiver descarregando, sua conclusão
pode ser registrada para liberar a doca; isso não altera o pedido cancelado.

## Contratos e limites

Onze operações em `/local/v1/logistics`; OpenAPI em `docs/api/dock-agenda.openapi.json`.
Campos desconhecidos, duplicados, nulos fornecidos ou números não inteiros são
recusados. IDs1..128bytes; nomes1..120bytes; justificativas1..255bytes, UTF-8
válido e sem controles ASCII. Revisões até9007199254740991. Page limit1..100,
offset0..1000000. Janela de consulta até31dias; horários da reserva em2000..2200.
Não há bloqueio automático de uma reserva histórica apenas por ser antiga.

Horários exigem segundos inteiros; timestamps reais registrados podem incluir
nanossegundos. A API não depende do fuso do PC para comparar janelas.
Dados de reserva são históricos; mudança posterior do fornecedor não muda o ID
preservado. Não permite usar agenda para reinterpretar pedido de outra loja.

## Migração, backup e aplicação

Aplicador exige o complemento Gestão/Produção intacto e histórico conhecido;
testa candidata antes de gravar. Acrescenta uma SQL, amplia suporte do backup
existente ao novo esquema, atualiza expectativas de versão e move probes
sintéticos para a versão seguinte. No B13, move também o probe de journal55→56.
Nenhuma asserção é removida. Mapas globais de rotas são ampliados quando presentes.

Fontes originais em backup separado; dados comerciais não são abertos pelo
aplicador. Migração só na inicialização normal do backend atualizado. Antes de
instalação comercial, validar backup restaurável e ensaiar em banco descartável.
Restaurar fontes não desfaz migração; binário antigo não deve abrir esquema novo.

Registro: `docs/architecture/BACKEND_AGENDA_DOCAS_APLICADO.json`. Verificação
idempotente usa ESTE aplicador. Reaplicar um pacote predecessor depois disso
detectará mudanças em seus arquivos e deverá parar, preservando a evolução.

## Fora desta entrega

Integração das telas, app do motorista, confirmação remota do fornecedor,
geolocalização, otimização de rota, política de prioridade entre fornecedores,
notificações entregues, sincronização dessa agenda entre coordenadores e operação
comercial offline distribuída. Outbox local é evidência durável; não comprova
envio ou aceitação remota. Desempenho em PC antigo/Windows não foi demonstrado.
