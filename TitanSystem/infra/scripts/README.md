# TitanSystem: Servidores, Scripts e CI/CD

**O que você vai codar aqui**: 
Arquivos `.sh` (Bash) e Pipelines do GitHub Actions/GitLab para deployar o Backend e construir os binários do Electron Desktop e os zips do Frontend Web de forma orquestrada, para enviar aos clientes ou à Nuvem central.

**Linguagem**: Bash (Shell Scripting), YAML.

**Processo e Regras**:
1. O processo de Build precisa embutir no binário (com `-ldflags` do Go) se aquela versão será Cloud, On-Premise ou Local SQLite.
2. Cada deploy para a nuvem cria um backup instantâneo de segurança do banco antes de rodar `AutoMigrate()`.

**Barreira de Segurança Contras Vazamentos**:
* Senhas SSH de servidores NUNCA estarão no código. Elas são injetadas pelos Secrets do CI/CD.
* O artefato gerado no desktop OS sempre deverá possuir Sign/Certificado Válido (Apple Developer e Windows Authenticode) para o antivírus do cliente não bloquear o App.
