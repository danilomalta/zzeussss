# Fase 8D — Cadastros com contrato na transação

## Entrega

`catalog.CreateProductWithContract` e `catalog.CreateLocationWithContract`
exigem `ManageStock` e módulo `inventory`, escolhido pelo backend.
Recebem o mesmo banco usado pelo Store de contratos. Sessão humana e prova
do aparelho devem ser resolvidas pelo chamador antes da chamada.

Validação de vínculo, loja, aparelho, assinatura, vigência, módulo, observação
de horário e cadastro compartilham uma única transação SQLite. Falha de
cadastro desfaz também a observação de horário. Não há autorização em uma
transação separada seguida de gravação desprotegida.

Preço e custo continuam em centavos inteiros. Nenhuma migração nova.
Leitura de catálogo continua condicionada a vínculo, loja e aparelho,
sem exigir licença vigente; expiração não apaga nem impede essa consulta.
Isso não implementa exportação contábil ou política de encerramento de caixa.

## Transição explícita

As funções antigas `CreateProduct` e `CreateLocation` são compatibilidade
legada, verificam permissões mas NÃO consultam contratos. Foram mantidas
para preservar os chamadores atuais enquanto se integra a configuração.
Não usar esses caminhos em novas rotas.

A API local e o comando `serve` NÃO foram alterados nesta etapa e ainda
chamam o caminho legado. Portanto esta entrega NÃO comprova aplicação de
licenciamento nas rotas, estoque, caixa, vendas ou produção.

Próxima integração: chaves públicas emissoras confiáveis na configuração
do servidor, instalação por dono autenticado e troca dos chamadores HTTP
pelas funções com contrato. Não aceitar chave emissora enviada junto do
contrato como fonte de confiança; não instalar módulos fictícios por padrão.

## Verificação

Sete testes novos, usando banco descartável e chaves geradas em memória:
ausência de contrato/verificador; dono sem módulo; cadastro autorizado e
valores exatos; papel/revogação; expiração com leitura autorizada; falha de
INSERT com rollback da observação; aparelho de outra empresa/revogado.

Go não está disponível no ambiente que preparou este patch. Compilação,
testes e vet devem ser observados no PC antes de registrar a etapa pronta.
Nenhum banco real deve ser aberto para validar esses testes.
