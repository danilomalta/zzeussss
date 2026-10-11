# Entrega 52 — Ambiente PostgreSQL e API demonstrável

Base de aplicação no PC: main, 4a31d79. Esta entrega responde à falta de um
ambiente operacional visível. Não modifica a branch de produção ou o Compose
principal. Não instala Docker, não faz push e não usa o PostgreSQL do sistema.

## Cinco funções

1. `init`: gera quatro valores aleatórios privados e registra a identidade do
   laboratório. Uma segunda execução preserva os valores existentes.
2. `start`: compila os pacotes completos da API e do comando de manutenção;
   inicia PostgreSQL 15; cria a base somente no volume novo; aplica as migrações
   online; inicia a API e exige saúde real do banco e do HTTP.
3. `check`: confere saúde, checksums das migrações, inventário de tabelas e
   resposta HTTP. Executa somente comandos `check-*`, sem aplicar migrações.
4. `diagnose`: apresenta serviço, estado, saúde e código de saída; não imprime
   variáveis de ambiente ou logs brutos.
5. `stop`: para os containers com prazo de encerramento da API e preserva
   volume PostgreSQL, arquivos e credenciais.

Comandos após aplicar, a partir da raiz do projeto:

    python3 -B tools/titan_lab.py init
    python3 -B tools/titan_lab.py start
    python3 -B tools/titan_lab.py check
    python3 -B tools/titan_lab.py diagnose
    python3 -B tools/titan_lab.py stop

## Isolamento e persistência

Projeto Docker: `titansystem-lab52`. Volume: `titansystem_lab52_pgdata`.
Estado privado: `~/.local/share/titansystem/lab52`, diretório 0700 e arquivos
0600. Credenciais não entram no Git, no ZIP ou na imagem.
O contexto de build possui uma lista explícita de código permitido e exclui
arquivos de ambiente, chaves e bancos. Não utiliza a pasta TitanSystem duplicada.
Os recursos Docker carregam uma identidade aleatória de proprietário; colisão
com volume/container de outra identidade bloqueia a operação.

A API publica apenas `127.0.0.1:18080`. PostgreSQL 5432 não é publicado no
host; o serviço do sistema na porta 5432 continua independente. API e migração
compartilham o namespace de rede do PostgreSQL e conectam a `127.0.0.1`.
Isso atende à política existente que só permite SSL desabilitado em loopback,
sem enfraquecê-la ou criar exceções para hosts remotos. A aplicação usa um papel
sem superusuário, criação de banco ou criação de papéis.
Somente Docker Engine Linux no contexto local `default`, via socket Unix, é aceito.

`base.sql` contém criação não destrutiva das tabelas base e as migrações
incrementais 2, 3 e 4. Não executa `000001_init.sql` nem sua seção destrutiva.
Depois, a ferramenta existente verifica/aplica sessões e os escopos de catálogo,
revisão de descontos, custos, estoque, propostas, fornecedores, ofertas,
rascunhos e documentos de compra até a migração 23.
A inicialização oficial do PostgreSQL executa a base apenas em volume novo.
Se ela falhar, nenhum comando apaga ou reinicializa o volume automaticamente.

## Aceite e limites

O instalador exige Docker antes de aplicar o patch, confere base e árvore limpa,
valida o patch, executa os testes Python e Go e compila os dois executáveis.
Depois executa `init` e `start`; só cria o commit quando `check` confirma
PostgreSQL, migrações, tabelas e HTTP reais. Em falha, conserva alterações e
recursos. Não reaplique automaticamente. Um `start` após `stop` reutiliza os dados.
Os containers permanecem iniciados após sucesso; use `stop` quando terminar.
Nenhuma conta, venda, pedido ou dado comercial é inventado como demonstração.

Preparação: 15 testes Python de permissões, propriedade, Docker remoto,
falhas, verificações de saúde, modo somente consulta e parada; validações de
shell, SQL, Compose e compilação dos pacotes completos. Docker real não estava
disponível na preparação: o aceite real ocorre no PC e não pode ser declarado
concluído antes de sua saída. Imagens precisam de download na primeira execução.

Esta entrega instala um laboratório online, não o produto completo. Não inclui
interface, cadastro inicial de administrador, publicação LAN, TLS entre aparelhos,
pareamento PC–PC, reconciliação, mobile ou venda com PC desligado. A comunicação
offline requer demonstração própria dos transportes locais já existentes e um
cliente móvel com banco próprio; um PostgreSQL saudável não comprova essas funções.
