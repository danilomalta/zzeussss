# Frontend consolidado68 — 26/10/2026

Base recomposta a partir dos fontes exatos dos pacotes65,66,67. As dependências e os20 arquivos66 foram comparados com o manifesto67: sem divergências. Commit de instalação no PC informado pelo proprietário:7cfa15e. A base recomposta possui um commit local próprio; não representa clone do HEAD do PC.

## Código entregue
- Preserva catálogo, estoque, PDV, caixa, produção, receitas, rendimento, perdas, compras, financeiro, manutenção, relatórios e pedidos autenticados já presentes. Não recria módulos nem assume aceite por existirem.
- Contenção de erro de renderização e de rota em toda a árvore. Falha numa tela mantém o shell pai sempre que a exceção estiver no limite da folha. Exceção no próprio shell sobe ao limite do pai. A última barreira cobre falhas de renderização do roteador. Não cobre erros assíncronos em handlers nem rejeições de rede já tratadas pelos clientes.
- Tela de recuperação em cinco idiomas, sem exibir mensagem/stack/URL do erro. Cópia explícita contém somente versão,código genérico,área local/online e dataUTC. Nenhuma chamada ao backend, gravação de fila ou repetição de operação é feita pelo novo tratamento.
- Menu passa pelos guardas existentes. Recarregar exige confirmação pois campos voláteis podem se perder. A mensagem não cancela nem confirma venda,pagamento ou estoque; consultar recibo antes de repetir permanece obrigatório.
- Link de salto para conteúdo, foco no título após mudar caminho e anúncio acessível. Filtro por query não muda foco; navegação por hash é preservada; interação de teclado/ponteiro durante carregamento cancela a mudança de foco. Espera até15s pelo título de um módulo lazy, sem repetir importação nem transação.
- Aviso de rede usa somente o estado do navegador e declara que isso não comprova disponibilidade do servidor local. Não bloqueia vendas indiscriminadamente nem reenvia operação ao voltar a rede.

## Validação observada nesta entrega
-2475 testes Node do frontend,57 testes do desktop, TypeScript sem erro.
-7 testes React/DOM com JSDOM: contenção real de exceção em rota,404,última barreira,clipboard recusado,diagnóstico,teclado/foco/rede,cinco idiomas. DOM emulado não substitui Electron nativo ou leitor de tela.
-Build real desktop/React268 arquivos,27.41s,backend_bundled=false. Continua aviso de chunk de planilhas de6MB. Nenhuma tentativa de ocultá-lo alterando o limite.
-Pontes HTTP produção67 e pedidos66 foram executadas nas entregas anteriores; seus fontes e evidências acompanham o histórico, não foram repetidas como se fossem testes novos68.
-O aplicador repete npmci, toda a suíte frontend, TypeScript, desktop e build numa candidata descartável dos arquivos atuais do projeto, incluindo alterações independentes. A gravação acontece somente se os arquivos não mudarem durante os testes.

## Instalação e preservação
O pacote contém fontes consolidados de referência. NÃO copie a pasta inteira sobre o projeto. Somente os caminhos de manifest.changes são gravados, com verificação de preimagem e git apply --check. Sem backend,autenticação,sessões,migrações,bancos,serviços,merge ou push. Fontes novos coincidentes sem marcador são recusados para evitar substituir trabalho alheio. Continuidade compartilhada não é alterada: esta nota é o registro próprio68.

O ZIP não contém.env,estações,senhas,chaves,bancos,node_modules,Electron nem perfil de usuário. Dependências são instaladas conforme locks existentes; download do Electron só é feito pelo atualizador explícito se faltar executável. Abrir versão nova pode focar instância antiga: fechar a janela antiga manualmente antes de abrir a nova evita confusão; nenhum processo é morto pelo pacote.

O commit separado confere hashes e seleciona somente arquivos68. Os arquivos de carrinhos e a continuidade independente permanecem fora do índice. O pacote não utiliza a rotina de commit67 que exigia o hash integral do documento compartilhado.

## Limites que permanecem
Fiscal/SEFAZ,adquirente/TEF/Pix,liquidação bancária,venda móvel independente,Windows e periféricos exigem contratos,serviços ou homologação próprios. Pagamentos mistos não se tornam aprovados pela existência da tela. APIs existentes precisam estar disponíveis no serviço escolhido pelo proprietário. Operação visual/comercial completa permanece pendente no inventário96itens; não foi reduzida artificialmente a zero.
