# `pages/accountant/` (Portal Contábil)

Esta pasta contém o Portal Contábil do TitanSystem, focado no perfil **Contador**.

## Objetivo

- Oferecer relatórios e exportações com foco fiscal/contábil.
- Permitir geração e download de arquivos exigidos por obrigações acessórias.

## Como os relatórios SPED/XML serão gerados

- O backend será responsável por:
  - consolidar dados de vendas, itens, impostos e cadastros
  - gerar arquivos no formato exigido (SPED e XML conforme escopo fiscal definido)
  - registrar auditoria da geração (quem solicitou, período, parâmetros)
- O frontend (esta pasta) será responsável por:
  - selecionar período, filtros e empresa/unidade
  - disparar a geração no backend
  - acompanhar status e baixar o arquivo gerado

## Regras de segurança

- Apenas o perfil **Contador** (e possivelmente **Dono**) pode acessar essas telas.
- Exportações devem ser auditadas e protegidas (links temporários, quando aplicável).

