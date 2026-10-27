# Aceite obrigatório das integrações do backend

Base: `3adf48088379bef7d25adebe37b699906e53d532`, branch `feat/backend-b02a-compat`. Esta unidade fecha lacunas de validação; não acrescenta operações comerciais nem atualiza serviços existentes.

## Ambiente reproduzível no CI

O workflow `backend-pilot.yml` cria PostgreSQL17 exclusivo do job, em porta dinâmica. Credencial declarada é fixture descartável do runner, sem acesso a instalação real. Cada teste PostgreSQL cria seu próprio schema UUID, sem executar a migração histórica destrutiva. O serviço inteiro desaparece com o runner; nenhum banco externo é selecionado.

O frontend de referência fica fixado em `b5b786d900365dfa6288f6273123d4ca8f1a6a7b`. Segundo checkout lê somente os adaptadores e testes de `frontend-web`; Node24 os executa sem instalar dependências. O código Go é sempre o SHA do evento. Não se usa a interface de uma branch flutuante para aprovar um backend diferente.

A suíte completa agora habilita obrigatoriamente:

- `TestPostgresDurableRotationConcurrencyAndRollback`: sessões duráveis, rotação concorrente, revogação e rollback no PostgreSQL real.
- `TestPostgresCatalogCreationAndDiscountAtomicity`: catálogo, descontos e demais cenários existentes de isolamento/atomicidade no PostgreSQL real.
- `TestCashJournalFrontendBridge`: adaptador do livro de caixa contra API Go HTTP real.
- `TestDeliveryFrontendBridge`: recebimento/recusa e recuperação contra API Go HTTP real.
- `TestMaintenanceFrontendBridge`: manutenção contra API Go HTTP real.

`tools/verify_backend_acceptance.py` exige aprovação desses cinco testes e dos pacotes correspondentes, recusa qualquer falha e qualquer teste ignorado fora de três exceções explícitas: dois auxiliares subprocessados pelos testes principais e a fixture visual opt-in. O JSON original é publicado junto do resumo. Esse gate impede aprovar apenas porque `go test` retornou zero com integrações ignoradas.

Depois permanecem vet, compilação dos seis executáveis Linux e os três pilotos com detector de corridas e `titan-backup` real, incluindo recebimento parcial, empresas autenticadas, reinícios e restauração. Artefatos são evidência e binários de teste, não deploy automático.

## Execução local e limites

Os cinco testes foram executados inicialmente separados em25/10/2026: dois PostgreSQL e três pontes HTTP aprovados. A execução completa é registrada junto desta entrega. PostgreSQL17.9 foi inicializado pelo usuário em diretório temporário novo, com senha sintética privada e porta livre; somente esse cluster foi desligado no final. Sem sudo, instalação, banco comercial ou reaproveitamento da porta5432. A ferramenta histórica `tools/test_online_postgres.py` continua disponível para sessões ou catálogo em um servidor autorizado; ela não é executada pelo workflow novo.

Não confundir ponte de adaptadores com ensaio visual de todas as telas. A fixture visual permanece opt-in e a interface de referência é a versão fixada acima. Isso não homologa Windows, fiscal, TEF/Pix, integração de hardware, todo o ERP ou operação comercial na Internet. Os dois serviços existentes não são substituídos por essa validação. O backend instalado3adf480 permanece o mesmo; nesta unidade não mudou o código dos seis executáveis.
