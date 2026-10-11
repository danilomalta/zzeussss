# P77 — Cinco filtros de reservas ativas da página

Base: `6d970ed`, branch `feat/producao-receitas`. SQLite permanece 44.

1. ID exato da ordem de produção.
2. ID exato da versão imutável da receita.
3. ID exato do responsável planejado da ordem.
4. Período de criação da reserva, desde/até, inclusivo, em UTC.
5. Quantidade mínima por reserva na unidade explícita do saldo consultado.

Aplica os critérios conjuntamente somente aos até 50 registros carregados. Não pesquisa o histórico ou outras páginas. Zero correspondências não significa ausência global. Responsável não é necessariamente quem criou a reserva. Período refere-se à criação da reserva, não à criação/conclusão da ordem.

IDs exatos e sensíveis à caixa, até 128 bytes UTF-8. A interface apara espaços externos dos IDs antes de aplicar; o validador puro recusa IDs não normalizados. Datas reais no calendário entre 0001 e 9999, ano zero recusado; início não pode superar fim. Um único limite também é aceito. Criação é validada como RFC3339 UTC com até nove casas fracionárias, conforme gravação atual do módulo. Hora/segundo/dia inválidos e deslocamentos de fuso não presumidos são recusados. A comparação usa o dia UTC original e inclui todo o último dia, sem conversão ao fuso do navegador.

Mínimo positivo em notação decimal com vírgula, até três casas; sem sinal, ponto, expoente ou separador de milhares. Converte com BigInt à escala de milésimos, limitado ao inteiro seguro existente. Quantidade mínima comparada individualmente, inclusiva, sem soma de reservas nem conversão entre unidades. O rótulo exibe a unidade consultada. Filtro não é solicitação de reserva/consumo.

## Integridade e interface

Antes de filtrar, valida toda a página, identidade de referências, unidades, quantidades, datas, duplicidade de reserva, tamanho/paginação, saldo físico = reservado + livre e soma da página <= reservado global. Registro inconsistente não é escondido por um filtro. O filtro não transforma nem corrige os dados. IDs de ordem/responsável repetidos entre reservas são permitidos; ID da reserva deve ser único.

Separação explícita entre campos editados e filtros aplicados. Mostra critérios efetivos, correspondências, tamanho da página e total global. Erro ao aplicar preserva critérios anteriores e informa a falha. Limpar retorna à página inteira. Nova consulta, mudança de página, produto/local ou sessão limpa o painel via `useReadTask` e remontagem. Respostas antigas continuam descartadas pelo mecanismo existente.

Saldo global, contagem global, agrupamentos da página e CSV permanecem completos, sem serem recalculados pelas correspondências. Descrição do CSV passa a dizer que exporta a página inteira antes dos filtros. Navegação de páginas mantém a busca original.

## Contratos compartilhados

`LocalStockAvailability.tsx`: tabela recebe os itens do novo painel local; exportação ganha descrição de escopo. `package.json`: inclui o teste. Leitor `stockRead`, API e cálculo dos agrupamentos não mudam. Reutiliza consulta autorizada existente; não envia filtros ao servidor e não adiciona HTTP.

Testes unitários: cinco critérios, combinação, limites inclusivos de datas/quantidades, nanosegundos, calendário, unidades, entrada inválida, duplicidade, inconsistência fora do filtro, paginação e totais preservados. Frontend completo/build; backend testes HTTP de reservas, suíte completa uma vez, vet e corrida dos testes HTTP de reservas. Patch e script verificados sobre cópia limpa, inclusive base incorreta e etapa já registrada.

Sem migração, ajuste de backup, escrita, reserva/consumo, main, merge/push ou bancos comerciais/demonstração. Sem aceite visual em navegador.
