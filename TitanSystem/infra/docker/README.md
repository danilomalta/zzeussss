# TitanSystem: Dockers e Contêineres (A Nuvem e Servidor Próprio)

**O que você vai codar aqui**: 
Arquivos `docker-compose.yml`, `Dockerfile` e arquivos bash de inicialização limpa.

**Linguagem**: YAML, Dockerfile e BASH.

**Processo e Regras**:
1. Haverá um Docker para o Backend Go Central e outro exclusivo para o PostgreSQL para o "Servidor da Nuvem do Cliente".
2. Multi-stage build para fazer o binário do Go não pesar nem 30MB, sem dependências de compilação sobrando na imagem final de produção.

**Barreira de Segurança Contras Vazamentos Nível Militar INFRA**:
* O banco rodando em Docker **NÃO PODE EXPOR A PORTA 5432 PARA O MUNDO EXTERNO** jamais, exceto que esteja em rede VPC interna. Se for para fora, exigirá túneis VPN.
* O arquivo `.env` definitivo na máquina final precisa de `chmod 600`, rodando com usuário `titan` e não `root`.
