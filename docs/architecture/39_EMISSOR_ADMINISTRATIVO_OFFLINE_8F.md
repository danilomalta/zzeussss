# Fase 8F — Emissor administrativo offline

## Entrega

Novo comando `backend/cmd/titan-license`, separado de `titan-local` e da API.
Não cria rotas, não abre SQLite/PostgreSQL, não consulta clientes e não altera
migrações ou casos de uso existentes.

Possui três operações explícitas:

- `init --private-key CAMINHO --key-id ID`: cria chave privada Ed25519 nova,
  com versão e ID emissor, em arquivo exclusivo 0600. Não imprime o segredo.
- `public --private-key CAMINHO --out NOVO_CAMINHO`: exporta somente o mapa de
  chaves públicas compatível com `--issuer-keys` da 8E.
- `sign --private-key CAMINHO --out NOVO_CAMINHO --tenant ID --revision N
  --expires RFC3339 --modules LISTA`: gera Envelope aceito pelo verificador
  existente. Empresa, revisão, vencimento e seleção são escolhas explícitas
  do administrador. Não há licença automática nem seleção padrão de módulos.

Os caminhos acima descrevem argumentos, não arquivos reais já existentes.
Não executar comandos de emissão com valores fictícios para ativar produção.

Selecionar `pos` inclui suas dependências `core` e `inventory`. `production`
inclui `core` e `inventory`, sem POS. A seleção efetiva é ordenada e impressa
após emissão. Módulos vazios, duplicados ou desconhecidos são rejeitados.
O contrato é verificado em memória antes de gravar. Revisão positiva e
vencimento posterior ao horário de emissão são obrigatórios.

O emissor não mantém um livro de revisões; o administrador deve escolher uma
revisão maior ao substituir o contrato. O cliente rejeita versões antigas ou
conflitantes conforme 8C/8E. Não há integração com cobrança ou portal SaaS.

## Segurança e arquivos

A chave privada pertence ao ambiente administrativo do emissor Titan/parceiro
autorizado. Nunca distribuir aos aparelhos dos clientes, incluir no instalador,
Git, imagens Docker ou enviar para mensagens de suporte. O arquivo público e
o Envelope são os únicos artefatos distribuíveis; a chave pública ainda precisa
chegar por canal confiável para não autorizar um emissor indevido.

Todos os arquivos de saída são criados exclusivamente, sem sobrescrever os
existentes. Diretórios precisam existir antes. Uma falha de escrita preserva
o arquivo para investigação; não há remoção/recriação automática.

Leitura da chave exige arquivo regular 0600, rejeita symlink, JSON desconhecido,
duplicado ou extra, mais de 8 KiB e chave privada cujo seed/public key sejam
inconsistentes. A validação atual de permissões e os testes têm como ambiente
de referência Ubuntu; isto não certifica empacotamento do emissor em Windows,
macOS ou dispositivos móveis. O código de domínio permanece independente
dos aplicativos móveis ainda planejados.

Fazer backup seguro e controlar quem acessa a chave emissora é responsabilidade
operacional. Sem HSM, trilha administrativa de emissões ou relógio confiável
nesta etapa. Não reaproveitar chave privada de aparelho como chave emissora.

## Verificação

Seis testes novos com subcenários: geração sem imprimir segredo; exportação
pública e emissão verificável; recusa de sobrescrita; entradas inválidas sem
criar contrato; permissões/symlink/JSON/chave inconsistente; comandos inválidos.
Só criam arquivos em diretórios descartáveis de teste e chaves em memória.

O ambiente que preparou esta etapa não possui Go. Executar testes, vet e build
no PC antes de registrar conclusão. Compilar e testar não emite chave ou
contrato de produção. A geração operacional será feita separadamente, com
IDs, revisão, módulos e vencimento reais definidos pelo proprietário.

Após esta etapa, a integração HTTP de estoque, caixa e venda ainda precisa
ser concluída. Módulos licenciáveis não equivalem a módulos funcionais prontos.
