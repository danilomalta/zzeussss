# Backend — inventário revisado, materiais e diagnóstico

Entrega de 22/10/2026. Código adicional; não substitui o backend inteiro.

## Origem e limites

- Frontend consultado: `feat/backend-checkout-authority`, commit `9a510880ba1cfcd4065254d4cb7206ae230a2c85`.
- Núcleo de produção recuperado: `feat/producao-receitas`, commit `ba29cbdf6a0a2081754609de63ce003414c74f79`, migrações 1–44.
- As 44 migrações históricas foram comparadas por SHA-256 com o manifesto B02B13: idênticas.
- B02B13 local do proprietário tem migrações 1–53. Seus fontes completos não foram publicados nesse commit. O pacote preserva essas migrações e acrescenta a 54, mas NÃO afirma que a combinação B13 foi executada aqui. O aplicador compila e testa a candidata local antes de aplicar.
- A mesma SQL foi executada como migração 45 na base recuperada de 44. Arquivos históricos não são alterados.
- Não há alteração de frontend, Electron, banco comercial, provedor financeiro/fiscal ou publicação Git.

## Inventário: fluxo oficial adicional

`open → count → submit → approve`, com `reopen` para recontar uma solicitação enviada e `reject`/`cancel` terminais. Não há aprovação automática por prazo ou frequência obrigatória. Uma nova contagem é iniciada quando a empresa decidir.

- Até 200 produtos por inventário, em um único local. Escopo explícito; não inferir que produtos fora dele foram contados como zero.
- `manage_stock`, vínculo ativo e aparelho autorizado em toda leitura/escrita. Escritas novas exigem módulo `inventory` contratado na mesma transação.
- `approve` e `reject` exigem owner/manager com permissão vigente. O revisor não pode ser o autor de nenhuma contagem corrente desse inventário, mesmo sendo owner.
- Contagem zero é válida; ausência é `null`. Unidade e quantidades em milésimos, inteiras e limitadas a 9.007.199.254.740.991. Produto `unit` exige múltiplos de 1000.
- Registrar a contagem e enviá-la não muda estoque. A aprovação muda todos os produtos, auditoria, recibo e outbox em uma transação.
- Cada contagem registra o saldo e uma impressão dos movimentos. Movimentação posterior exige reabrir e recontar: duas movimentações opostas também invalidam a contagem. Não há ajuste baseado em saldo antigo.
- A aprovação revalida reservas pelo helper comum de estoque e recusa saldo contado inferior ao reservado. Não consome/libera reserva de produção.
- Reenvio idêntico retorna o recibo original. Mesmo ID com outro conteúdo/ator falha. Recibo histórico não usa o estado posterior como se fosse o resultado original.
- Leituras históricas exigem permissão/aparelho vigentes, mas não renovação da licença. Revogação não autoriza novas leituras nem reenvios.
- Histórico conserva pedidos e resultados originais, inclusive quantidades anteriores a recontagens. Não apaga rejeições ou ajustes.
- Lista e histórico paginados, totais globais, exportação CSV com campos numéricos e neutralização de fórmulas em texto.
- Eventos `inventory.reviewed` entram na outbox. Isso não implementa aplicação remota desses eventos, eleição de coordenador ou venda offline distribuída.
- O método legado `inventory.Count` não foi removido nem transformado em aprovação. A nova API é o fluxo revisado; não há migração silenciosa de ajustes antigos.

## Estimativa de materiais

Receita publicada e ordem congelada recebem uma consulta de estimativa. Usa quantidades oficiais e custo ATUAL por unidade do catálogo dentro de um snapshot transacional.

- Exige `manage_production` E `view_accounting`. Ter acesso à receita não dá acesso a custos.
- Aritmética racional com `big.Int`/`big.Rat`, sem float. A resposta conserva numerador/denominador exatos; arredondamento half-up ocorre sobre o agregado.
- Limites e mudanças de unidade são verificados; não interpreta kg como gramas automaticamente.
- Custo zero é marcado para conferência. `labor_cost_cents` e `overhead_cost_cents` são `null`, `is_complete=false`.
- Não é custo real histórico, média ponderada, folha, margem/lucro, recomendação de preço ou lançamento financeiro. Não congela o custo atual como se fosse custo de produção já realizada.
- Não movimenta estoque nem publica evento de operação financeira.

## Identificação autenticada do serviço

`GET /local/v1/system/info` exige sessão e `view_catalog`. Retorna serviço, revisão de build (ou `unknown`), versão de esquema aplicada, versão desta API e contexto da sessão. Sem caminho do banco, senha ou chave.

Não altera `/health`, não prova identidade criptográfica de instalação/banco e não autoriza o Electron a adotar um processo desconhecido. A revisão pode ser definida no build com `-ldflags '-X titansystem-backend/internal/localapi.SystemBuildRevision=REVISAO'`.

## Migração, backup e instalação

O aplicador aceita somente bases verificadas de 44 ou 53 migrações. Na base 44 acrescenta 45; na B13 acrescenta 54. Com outra base, para e informa o caminho. A extensão de versões aceitas pelo backup mantém checksum, integridade, isolamento, criptografia e revogação de sessões na restauração.

Atualiza apenas o ponto de montagem no arquivo stock.go e os números de versão nos testes/backup, preservando o restante do conteúdo. Não substitui server.go nem as rotas B13. Cada transformação exige a estrutura exata esperada e a candidata é testada antes de escrever no projeto.

O banco existente não é aberto pelo aplicador. Depois de aplicar o código, uma inicialização futura do serviço executa a migração normal: preparar backup restaurável e ensaio descartável antes de atualizar uma instalação comercial. Não tentar abrir esquema novo com binário antigo.

## Verificação

O pacote inclui logs de testes Go, detector de corrida, vet, build e testes do aplicador. Testes de API exercitam sessão real e SQLite descartável; não são mocks de pagamentos nem homologação comercial. Restauração cifrada de inventário testada.

Comandos das unidades novas:

```
go test ./internal/localdb/stocktaking ./internal/localdb/production ./internal/localapi -run 'ReviewedInventory|MaterialCost'
go test -race ./internal/localdb/stocktaking ./internal/localdb/production ./internal/localapi -run 'ReviewedInventory|MaterialCost'
```

Sem teste de Windows, smartphone, impressora, balança, PSP, SEFAZ, desempenho em PC antigo ou serviço B13 real do proprietário.

## O que continua pendente

Frontend dos endpoints novos, configurações de frequência de inventário, inventário cego/opcional por política, custo real completo, estoque contábil médio, integração bancária/fiscal real, transmissão de eventos comerciais entre empresas, rastreamento móvel e fechamento do ERP completo. Não apresentar esta entrega como conclusão desses itens.
