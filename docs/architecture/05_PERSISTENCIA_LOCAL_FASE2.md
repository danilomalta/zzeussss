# TitanSystem — fundação SQLite local (Fase 2)

## Alcance desta entrega

O pacote `backend/internal/localdb` cria um **arquivo novo escolhido pelo chamador**; não é chamado pelo `main.go` atual e não altera PostgreSQL. `backend/db/migrations/000001_init.sql` não é executada. Nenhuma tela, venda real, serviço de sync, criptografia, autorização ou app mobile é habilitado por esta entrega.

O esquema inicial contém empresa, loja, dispositivo, usuário, produto, localização de estoque, movimento, sessão e movimento de caixa, venda, item, pagamento e outbox. Chaves compostas impedem que um movimento da empresa A cite produto, dispositivo ou local da empresa B. Quantidade é inteiro em milésimos de unidade; dinheiro é inteiro em centavos. Vendas, estoque e eventos terão operações transacionais próprias em fases futuras.

## Caminho de instalação e durabilidade

- Cada dispositivo usa **seu próprio arquivo local** em disco interno. Não abrir o mesmo SQLite via compartilhamento de rede nem copiar arquivo aberto como método de sincronização.
- `Open(ctx,path)` cria o arquivo com permissão 0600, limita a uma conexão por processo, verifica chaves estrangeiras, ativa WAL, timeout de ocupação de 5 segundos e `synchronous=FULL`. O sistema pode negar WAL em filesystem inadequado.
- WAL permite leitores enquanto há escritor, mas há um escritor por vez. Medir latência e concorrência em hardware real antes de colocar em operação. Backup deve usar mecanismo consistente do SQLite, com os arquivos WAL considerados, e ainda exige projeto de criptografia e restauração.
- Migrações deste pacote são incorporadas no binário, aplicadas em transação, com versão e checksum. Migração aplicada nunca é reescrita; adicionar `0002_*.sql` e testar em cópia descartável. O parser simples aceita instruções terminadas por ponto e vírgula e **não** aceita triggers ou ponto e vírgula dentro de literais; substituir por mecanismo adequado antes de tais migrações.
- `Open` não adivinha caminho padrão e não é chamado automaticamente na inicialização da API. O aplicativo futuro decidirá o diretório por instalação e como obter a chave de criptografia, antes de usar dados reais.

## Verificação exigida no PC do proprietário

Instalar dependência Go do driver puro, rodar testes do pacote, todos os testes Go e `go vet ./...`; verificar `git diff --check` e `git status --short`. Os testes criam arquivos somente sob `t.TempDir()` e exercitam reabertura, reaplicação, rollback, isolamento entre empresas e geração de ID. Se o driver não compilar, **não** marcar a fase como concluída; enviar o erro para correção. O PostgreSQL real não participa.

## Restrições ainda abertas

`localdb` não autentica usuário nem fornece API para venda; apenas as FKs não impedem leitura indevida por uma query sem filtro. A fase de autorização deverá derivar tenant/loja da sessão. Criptografia em repouso, backup recuperável, restauração, medição em aparelho real e sync bidirecional são trabalhos posteriores. Nunca publicar o arquivo SQLite de teste nem considerá-lo nota fiscal ou relatório de produção.
