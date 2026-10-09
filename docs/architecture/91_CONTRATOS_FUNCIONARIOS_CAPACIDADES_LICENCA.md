# Funcionários, capacidades e licença local — entrega 16

Base de aplicação: main `c2f86e5`. Quatro operações existentes recebem contrato detalhado em `docs/api/staff-capabilities.openapi.json` e testes HTTP. Não altera handlers, migrações, frontend ou a branch de produção.

| API | Função efetiva |
| --- | --- |
| GET /local/v1/capabilities | Contexto, papel, dez permissões operacionais selecionadas e metadados do contrato |
| GET /local/v1/staff | Até 100 membros da loja por nome/id; exige Staff vigente e manage_staff |
| POST /local/v1/staff | Identidade local, senha com hash, vínculo de loja e registro auditável |
| POST /local/v1/module-contracts | Instalação pelo dono de contrato assinado por emissor já confiável |

## Uso correto pela interface

Permissão e contratação são informações distintas. O dono pode ter sell e não ter POS contratado. Uma área operacional exige a permissão correspondente, o módulo e a vigência; o backend revalida todos esses requisitos na operação. O teste cobre esse caso: capacidades do dono não tornam possível abrir caixa sem POS.

A lista de permissões desta resposta é um subconjunto operacional, não catálogo completo da política de acesso. Ela não substitui as APIs de administração de permissões/departamentos.

Estados de licença: active, expired, not_yet_valid, unavailable, not_installed, invalid e clock_blocked. Contratos autenticados expirados ou futuros conservam seus módulos para exibição; isso não permite escrita. Sem confiança, módulos ficam vazios. remaining_days arredonda para cima somente em active; expires_unix é opcional. A consulta pode atualizar o relógio observado da licença.

Essa validade não é data de fatura. Não implementa cobrança, pagamento, emissão de nova licença após pagamento, aviso de sete dias ou carência comercial de três dias. Essas funções permanecem pendentes.

## Funcionários e repetição

Nome é normalizado com trim. IDs até 128 bytes; nome até 255 bytes; senha de 12 a 72 bytes sem espaços nas bordas. JSON exige exatamente cinco campos. Senha é writeOnly no contrato e não aparece em respostas.

Owner pode criar manager, production, accountant, supplier, employee, cashier e stock. Outros autores autorizados só criam employee/cashier/stock. Não se cria outro owner por esse endpoint. Papel supplier não estabelece parceria autenticada entre empresas.

Novo cadastro retorna 201; replay idêntico retorna 200 e verifica a senha atualmente armazenada. Troca posterior dessa senha impede replay usando a anterior. O cliente não deve guardar a senha indefinidamente para repetir operações.

A consulta tem limite fixo de 100 sem offset, total ou has_more. RH completo, folha, ponto, avaliações e paginação dessa lista ainda não foram implementados por esta entrega.

## Contrato assinado

O envelope tem key_id, payload base64 e signature base64. O payload contém os bytes assinados; reserializar seu JSON pode invalidar assinatura. Chave privada nunca acompanha a requisição. A instalação verifica dono, aparelho, assinatura, empresa, vigência, revisão e dependências; resposta nova ou repetida é 200.

## Aceite

Testes usam fixtures temporárias: instalação e replay, cadastro e consulta sem credenciais, permissão sem módulo, sete estados de licença e bloqueios de consulta Staff. Testes existentes cobrem autorização, escalada de papel, rollback e integridade dos contratos.

Verificar JSON/referências, API local, entitlementstore, vet e race dos testes novos. O validador de payload cobre o subconjunto de schemas usado pelo projeto; não certifica todos os recursos do padrão OpenAPI. Nenhum servidor, banco comercial, migração real ou push é necessário.
