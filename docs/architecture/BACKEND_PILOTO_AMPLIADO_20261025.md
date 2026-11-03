# Backend: piloto ampliado, restauração pelo executável e CI

Base:8d8d91cce137cce670937ea3936df7cdb1970725, branch feat/backend-b02a-compat. Nenhuma migração nova ou aplicação do ZIP64. Frontend permanece separado e preservado.

## Mudanças funcionais

`titan-backup inspect` autentica e valida um arquivo cifrado antes de produzir JSON. Não é download de banco pela API. Não altera o banco original, não revoga suas sessões e não restaura automaticamente. A saída contém versão/schema, SHA256 do arquivo cifrado, contagens e `business_sha256` do perfil `pilot_cash_v1`.

O perfil inclui14tabelas: vendas/parcelas, caixas/movimentos, pedidos/itens/decisão manual/viagens/recebimentos/tratativas, obrigações/parcelas/vínculos em dinheiro e movimentos de estoque. Não inclui credenciais, sessões, estação ou chaves. Os dados são lidos somente dentro do snapshot validado e descartável. Somente as contagens e o hash saem do processo. O hash não é uma soma de quantidades ou moedas; não mistura unidades. Também não é atestado de todos os módulos do ERP. Tabelas inexistentes em schemas anteriores são listadas em `unavailable`, sem retornar zero fictício. O [contrato JSON](../cli/titan-backup-inspect.schema.json) descreve a saída.

A ordenação do resumo usa todas as colunas, não rowid, mantendo o resultado após VACUUM/restauração. O hash do arquivo cifrado muda quando um backup novo é criado, pois usa nonce novo; o resumo comercial e as contagens devem permanecer iguais se os dados não mudaram. Para schemas diferentes, compare somente segundo procedimento de migração, nunca suponha hashes iguais.

`help`, `--help` e `-h` explicam os comandos e limites sem abrir banco ou ler chave. Os comandos anteriores create/verify/restore/watch/key-init permanecem disponíveis. A chave não é exibida e a restauração continua exigindo destino novo. Limite de snapshot128MiB preservado.

## Circuito parcial com avaria

O teste ampliado reutiliza o circuito integral. Há5unidades iniciais, venda de2, sugestão de2 e pedido aprovado. A entrega informa2unidades, mas somente1é aceita; a outra é recusada por avaria. Rastreio exige entregue2000/aceito1000/recusado1000/restante1000milésimos. Apenas a aceita entra no estoque:5000−2000+1000=4000.

O reenvio da mesma operação com quantidade alterada recebe409. Uma tratativa de avaria solicita reposição, passa a in_progress e continua pendente. Reiniciar o serviço nesse ponto não muda quantidades nem resolve a tratativa. Não há reposição, nova compra, crédito ou pagamento automático.

Neste acordo sintético, somente a unidade aceita é documentada por1025centavos. Outro gerente aprova a obrigação; o criador é recusado. Retirada e vínculo são distintos e não duplicam dinheiro. Caixa fecha1000+3198−1025=3173centavos, diferença zero. A unidade avariada não é contabilizada como estoque aceito nem paga nesse documento. O preço é entrada explícita do documento do cenário; o backend não deduz o acordo comercial a partir do custo cadastral.

Após outro reinício e restauração em banco novo, o teste compara19recursos/rastreios/históricos e recibos; só ignora o measured_at de consulta da tratativa, depois de validar seu formato. Horários comerciais, versões, IDs e evidência de abertura continuam sendo comparados. A sessão antiga é recusada com401. Reenvios explícitos com os IDs originais não duplicam venda/recebimento/pagamento. A tratativa deve permanecer in_progress no banco restaurado.

## Prova pelo executável

Com `-pilot-backup-cli=CAMINHO_ABSOLUTO`, os dois circuitos usam um binário Go real, não `run()` chamado de dentro do teste. A estação/chaves são criadas somente na fixture descartável; não se lê instalação comercial.

O teste executa key-init, create com API ativa, verify e inspect; exige recusa de chave errada e empresa errada; executa restore em caminho novo e exige recusa de sobrescrita. Cria outro backup do banco restaurado, inspeciona e compara contagens e business_sha256. As duas saídas BACKUP REPORT são registradas no log sem linhas de tabelas ou credenciais. Depois a API abre o banco restaurado e o circuito confirma os mesmos resultados eIDs.

O teste integral mantém101requisições HTTP/quatro processos; o parcial faz119requisições/cinco processos. Com CLI, há comandos de processo adicionais fora dessas contagens HTTP. O caminho HTTP usa handlers de produção/TCP/SQLite; a variante opcional Electron continua sendo uma verificação local separada. O CI desta unidade não instala Electron nem automatiza cliques de UI.

## CI e reprodução

O novo workflow `.github/workflows/backend-pilot.yml` dispara push para feat/backend-b02a-compat, PR para main/develop com mudanças do backend e acionamento manual. Possui somente contents:read, actions oficiais fixadas por SHA, Go1.25.8, vet, build dos executáveis, suíte inteira e os dois circuitos com detector de corridas e CLI compilado. Logs e binários Linux são artefatos do próprio run, disponíveis por14dias. Não faz merge, deploy, instalação no cliente nem publica arquivos de banco/estação/chaves.

A aprovação do CI deve ser conferida no run vinculado ao SHA. A existência do arquivo YAML não é aprovação. O histórico antigo permanece visível. Os resultados locais e a URL final observada serão apresentados na entrega e no registro único de continuidade do frontend; não atribuir o sucesso antigo da main a estes fontes.

No terminal, já com Go compatível e dependências existentes, use os comandos do README. O binário entregue foi compilado para Linux amd64 com CGO_ENABLED=0; não substitui a configuração explícita da empresa/loja/aparelho. Não execute sobre banco comercial sem definir caminhos, licença e custódia das chaves.

## Limites que continuam abertos

Confirmação do fornecedor ainda é manual externa; não é autenticação entre empresas. Não foi criado protocolo comercial ou concessão de acesso a outro tenant. Windows requer aceite em máquina limpa e adaptação/validação das proteções de arquivo0600/0700; a simples compilação não prova segurança ou operação. Nenhum instalador Windows foi homologado. Não inclui fiscal, TEF/Pix, vendas móveis ou sincronização offline comercial. Esta unidade não deve ser anunciada como conclusão de todo o backend do ERP.
