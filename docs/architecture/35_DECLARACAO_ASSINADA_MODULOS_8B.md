# Backend 8B — Declaracao assinada de modulos

Base informada: commit 0ebc746, contrato de modulos 8A testado no PC e commitado.
Este pacote adiciona verificacao criptografica isolada. Nao altera migracoes,
banco, rotas, permissao dos funcionarios, frontend ou instalacoes existentes.

## Origem confiavel

A empresa nao prova uma contratacao enviando uma lista de modulos na requisicao.
O emissor autorizado assina uma declaracao com Ed25519. O verificador recebe
somente as chaves publicas previamente confiaveis. Nenhuma chave publica recebida
no envelope se torna confiavel; o envelope nao tem esse campo.

As chaves privadas usadas pelos testes sao geradas em memoria. Elas nao sao
chaves comerciais e nao ficam em arquivos. A chave de pareamento do aparelho
nao e automaticamente uma chave emissora de contratos.

Ainda falta implementar o emissor, a distribuicao e rotacao de chaves confiaveis
e a politica de revogacao. Nao instalar uma chave de emissor fornecida pelo
cliente ou frontend para converter um contrato arbitrario em autorizado.

## Formato e verificacao

Envelope: key_id, payload e signature. Os campos binarios usam base64 na
serializacao JSON padrao do Go. Assinatura fixa: Ed25519. A mensagem assinada e
`TitanSystem.modules/v1` seguida de byte zero, key_id, byte zero e os bytes exatos
do payload. Nao reserializar o payload para verificar a assinatura.

Payload: version=1, tenant_id, revision positiva, issued_at, not_before,
expires_at e modules. Datas sao segundos Unix; inicio e inclusivo, vencimento
e exclusivo. issued_at deve ser positivo e nao posterior a not_before.

O verificador recebe a empresa esperada do contexto confiavel do backend e
rejeita contrato de outra empresa. Rejeita assinatura invalida, chave nao
confiavel, payload maior que 16 KiB, JSON com campos desconhecidos ou duplicados,
modulos desconhecidos ou duplicados e dependencias incompletas.

Uma lista vazia nao concede nenhum modulo. Nao significa cancelamento remoto:
e apenas uma declaracao sem capacidades. Validar assinatura nao autentica a
pessoa, nao verifica sua loja e nao prova autorizacao do aparelho.

## Offline e limites desta etapa

A verificacao pode executar sem internet enquanto houver declaracao e chave
confiavel instaladas. Ela ainda nao esta integrada ao servidor local.

O horario e fornecido pelo chamador. Ainda nao existe protecao contra retrocesso
do relogio, restauracao de backup antigo ou reapresentacao de revisao antiga.
revision permite o futuro armazenamento rejeitar retrocessos; o verificador
isolado nao conhece historico e nao promete essa protecao.

Revogacao imediata nao pode ser garantida em aparelho desconectado. Vigencia,
renovacao, eventual tolerancia e politica para operacoes em andamento precisam
ser definidas antes de ativar bloqueios. Este pacote nao encerra caixa nem
interrompe vendas existentes.

## Privacidade

A declaracao contem somente identificador da empresa, lista de modulos, revisao
e vigencia. Nao necessita vendas, estoque, funcionarios ou documentos fiscais.
Ela comprova disponibilidade funcional; nao transfere dados de negocio.

## Aceite e proxima etapa

- Testes da verificacao passam no PC e a suite do backend continua passando.
- Nenhuma permissao existente depende deste pacote ainda.
- Proxima etapa: armazenamento local atomico de contratos verificados por empresa,
  controle de revisao, historico de alteracoes e leitura confiavel de capacidades.
- Somente depois integrar essa leitura aos casos de uso, junto das permissoes de
  pessoa, loja e aparelho, com testes de rollback e uso offline.
