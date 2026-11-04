# P04 — revogação histórica sob controle dos titulares

Data: 04/11/2026. Base: `5ac74939c3e5bf0a56426b9d65ea73325ebf687f` (P02).
Estado: procedimento e checklist disponíveis; **revogação efetiva EXTERNA, não comprovada**.
Não foram abertos arquivos de ambiente, blobs históricos, bancos ou chaves,
nem tentada autenticação com qualquer credencial antiga. Nenhuma foi trocada.

## Evidência limitada a metadados Git

Histórico acessível pelas refs locais, sem clone raso. Não representa refs
remotas não obtidas, cópias externas, forks ou objetos fora dessas refs.

- `ae7d1c547798539dd57dde63fca5dab7af9d047c`, 20/05/2026:
  inclusão de `backend/.env` (161 bytes) e `frontend-web/.env` (60 bytes).
- `12ebaedc8af1918c73ca13e27302c7b607ad9122`, 02/06/2026:
  alteração de `backend/.env` (135 bytes).
- `157d37ed2d176ee9d3bfcce7467c6ba90fa33f0b`, 25/09/2026:
  alteração de `backend/.env` (164 bytes).
- `c2697af0dcc506ac2c37f3e96cec44a52158751b`, 29/09/2026:
  remoção de `backend/.env` do índice Git.
- `dfd430e616bb115256f9da4dddc28d6b12f3add9`, 30/10/2026:
  remoção de `frontend-web/.env` do índice Git.
- Os blobs acima têm modo Git `100644`; isso não comprova as permissões
  atuais dos arquivos privados no disco. Nenhum desses conteúdos foi lido.
- Na base desta unidade, `git ls-files` encontra somente `.env.example`,
  `backend/.env.example` e `frontend-web/.env.example` nesse recorte.
  Isso não exclui segredos em outros caminhos ou no histórico.

**Presença de arquivo não prova que um segredo real existia nele. Remoção
do índice não revoga credenciais nem elimina o histórico.** Não se declara
seguro ou comprometido um valor desconhecido por inferência do nome.

Inventário seguro reutilizado: comandos nativos de metadados Git. A busca
dirigida nos utilitários existentes não identificou inventário dedicado de
revogação histórica. `tools/titan_diagnostics.py` executa checks gerais,
não atesta revogação. `tools/titan_lab.py` protege arquivos privados, mas
seus comandos podem ler credenciais; nenhum deles foi executado nesta etapa.
Não foi criado outro scanner nem alterado `tools/`.

Comandos **para o terminal**, somente metadados, na raiz do checkout:

```bash
git log --all --full-history --no-renames --format='%H %cs' --name-status -- '*.env' '.env.*' '**/.env.*' ':!*.env.example' ':!**/.env.example' ':!*.env.sample' ':!**/.env.sample'
git ls-files --stage -- '*.env' '.env.*' '**/.env.*'
git ls-tree -l 157d37ed2d176ee9d3bfcce7467c6ba90fa33f0b -- backend/.env
```

Não acrescentar `-p`, `git show <commit>:<arquivo>`, dump de ambiente,
conteúdo de logs, DSN ou valores de variáveis a esse inventário.

## Responsáveis e ordem de execução pelo titular

As cinco categorias abaixo estão **EXTERNAS / NÃO CONFIRMADAS**. Aplicabilidade
também exige confirmação; não presumir que exista uma conta de cada serviço.
O mantenedor do ERP trata suas próprias contas de desenvolvimento/publicação;
cada cliente trata as contas, dados e chaves da sua instalação. O provedor do
ERP não recebe segredos de clientes nem cria acesso mestre para executar isso.

1. **GitHub — titular da conta e administrador do repositório.** Inventariar
   credenciais de acesso da própria conta, aplicativos, chaves de acesso e
   automações que possam ter sido expostas. Pelo controle oficial do serviço,
   revogar as antigas identificadas; provisionar substitutas privadas com
   privilégio mínimo apenas onde necessárias e conferir permissões/ações
   suspeitas. Registrar ID administrativo não secreto, horário e evidência
   de estado revogado. Remover um segredo de Actions/Git não prova que o token
   foi revogado no emissor. Não testar login com token antigo.
2. **Provedores — titular de cada contrato/conta.** Confirmar quais existem
   (por exemplo fiscal, pagamentos, e-mail ou infraestrutura); não supor que
   Focus ou outro provedor esteja contratado. Revogar pelo emissor e atualizar
   consumidores autorizados com credencial nova privada. Registrar escopo,
   ambiente e ID administrativo não secreto. Para certificado comprometido,
   o titular confirma com o emissor/autoridade o procedimento de revogação e
   substituição; apagar o arquivo local não revoga o certificado.
3. **Banco — administrador autorizado do ambiente do titular.** Identificar
   onde a credencial realmente é usada e preparar manutenção/backup privado.
   No PostgreSQL, trocar a autenticação da conta pertinente, revisar privilégios
   e encerrar conexões antigas autorizadamente: trocar senha não encerra por
   si só uma conexão já aberta. Atualizar só os consumidores legítimos e
   verificar a operação com a NOVA configuração, sem registrar DSN/senha.
   SQLite não tem senha de usuário PostgreSQL: revisar proteção de arquivo,
   permissões e cifragem aplicável, sem inventar uma senha de banco SQLite.
4. **JWT — administrador da instalação de cada cliente.** Seguir o plano P02
   em `CONTINUIDADE_TITANSYSTEM.md`: parar todas as instâncias/renovadores;
   revogar sessões persistidas e auditar em transação antes de reabrir;
   gerar material exclusivo com CSPRNG e entregar em armazenamento privado;
   instalar a chave em todos os consumidores autorizados e exigir novo login.
   Formato/comprimento aceitos não provam entropia ou exclusividade. Trocar só
   JWT_SECRET não revoga refresh opaco. A evidência de rejeição da sessão antiga
   deve vir dos ensaios descartáveis e da revogação registrada; nesta tarefa
   não se tenta usar chave/token antigo contra serviço real.
5. **Chaves de backup/cofre — cliente custodiante e administrador autorizado.**
   Antes de alterar qualquer chave, mapear privadamente os backups que dela
   dependem e guardar cópia recuperável sob controle do próprio cliente.
   Usar o mecanismo de rotação/backup existente, com destino escolhido pelo
   cliente; não confundir chave JWT com chave de cifragem. Recriptografar as
   cópias necessárias e provar restauração/integridade em área descartável
   controlada pelo cliente, sem executar sobre banco comercial. Se a política
   de privacidade não permitir copiar dados reais para ensaio, definir área
   privada autorizada e ensaiar o procedimento com dados sintéticos primeiro.
   **Não apagar/destruir a chave antiga antes de recriptografar e comprovar a
   restauração de todos os backups retidos necessários.** Enquanto houver
   dependência antiga, conservar a chave segregada, privada e inventariada pelo
   cliente. O provedor não mantém cópia nem consegue recuperar chave perdida.

As etapas podem ser coordenadas em paralelo por titulares distintos. Dependência
entre consumidores e chave de backup exige planejamento antes do corte; não
trocar indiscriminadamente nem interromper serviços do cliente nesta tarefa.

## Checklist preenchível de evidências — uma cópia privada por categoria/serviço

O registro preenchido pertence ao titular, **fora do repositório público**.
Não colar segredos, parte de segredo, fingerprint derivado de segredo, DSN,
certificado, token, export de configuração ou captura de tela sem sanitização.
Usar identidade administrativa não secreta do recurso e referência privada
de evidência; não publicar identidade de cliente/dados pessoais neste modelo.

- Categoria: `[GitHub | provedor | banco | JWT | backup/cofre]`
- Titular/responsável autorizado: `[preencher no registro privado]`
- Serviço/recurso, ambiente e instalação: `[ID administrativo não secreto]`
- Aplicabilidade: `[não confirmada | aplicável | não aplicável + justificativa]`
- Estado atual: `[EXTERNO — NÃO CONFIRMADO]`
- [ ] Inventário de consumidores e privilégio mínimo conferido pelo titular.
- [ ] Nova credencial/chave gerada exclusivamente no ambiente do titular;
      armazenamento, permissões e entrega privada conferidos. Data: `[AAAA-MM-DD hh:mm fuso]`.
- [ ] Credencial antiga revogada no EMISSOR; ou autenticação substituída no
      banco e conexões anteriores encerradas; ou sessões JWT revogadas com
      auditoria transacional. Serviço/ID não secreto: `[ ]`; data/fuso: `[ ]`.
- [ ] Evidência administrativa do estado inativo/revogado recebida do titular.
      Referência privada sem URL com token/query: `[ ]`; verificador: `[ ]`.
- [ ] Nova configuração verificada em ambiente autorizado, sem tentativa de
      autenticar com segredo antigo. Resultado/data/fuso: `[ ]`.
- [ ] Se backup/cofre: relação privada dos backups afetados, recriptografia,
      verificação de integridade e restauração em área descartável comprovadas.
      Versão do programa/esquema e referência da prova: `[ ]`.
- [ ] Se backup/cofre: destino e custódia de chaves/cópias definidos pelo
      cliente; chave antiga PRESERVADA enquanto necessária. Responsável/data: `[ ]`.
- [ ] Revisão final: `[confirmado pelo titular | pendente | falhou]`, motivo
      sem valor secreto: `[ ]`; próxima ação/responsável/data: `[ ]`.

Nenhum campo está preenchido como revogado nesta entrega. Sem evidência do
titular, permanece EXTERNO; teste com fixture ou leitura do Git não comprova
revogação de credencial de serviço. Um teste que precise de segredo novo deve
ser executado no ambiente autorizado do titular, sem repassá-lo ao Codex.

## Critério de encerramento e continuidade

P04 encerra o procedimento/documentação, não as revogações externas. O aceite
de implantação que depende dessas credenciais continua pendente até prova do
titular; trabalho de código/diagnóstico independente pode continuar. Não se
reescreve histórico nem se faz force push. Próximo ID independente: **P05**;
não iniciado nesta unidade. Na entrega original P04, P03 permanecia WIP separado. A conclusão posterior
de P03/P05 está registrada em CONTINUIDADE_TITANSYSTEM.md; as revogações reais
deste checklist continuam EXTERNAS, sem comprovação por esta documentação.
