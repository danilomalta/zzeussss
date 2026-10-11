# Entrega 53 — Estação SQLite operacional isolada

Base no PC: main, 396d48d. O laboratório PostgreSQL da entrega 52 permanece
independente. Esta entrega não usa seu banco online ou a branch de produção.

## Cinco funções

1. `init`: compila titan-local e titan-peer completos e cria uma instalação
   SQLite nova, com empresa, loja, dono e aparelho aprovados. Usa dados com
   identificação explícita de laboratório e senha aleatória privada.
2. `serve`: inicia a API local real em 127.0.0.1:18181, no primeiro plano;
   Ctrl+C encerra o processo preservando o SQLite. Nenhum daemon fica escondido.
3. `check`: exige a porta livre, inicia seu próprio servidor, realiza login,
   verifica a identidade e contexto, faz logout e exige rejeição da sessão
   revogada. Repete após reiniciar o processo e verifica a identidade persistida.
4. `inspect`: verifica banco, referências e aparelho usando titan-local check.
   Essa manutenção pode aplicar migrações e registrar desafios; não é operação
   somente leitura. Não imprime senha ou chave do aparelho.
5. `peer`: cria uma chave de transporte aprovada pelo dono local, exporta o
   descritor público assinado e confere suas impressões digitais com titan-peer.
   Um arquivo existente diferente em Downloads bloqueia a exportação.

O estado fica em ~/.local/share/titansystem/local53 (0700). Banco, identidade,
metadados e senha são 0600; binários privados, 0700. Nada disso entra no Git.
Arquivos incompletos são preservados e impedem uma nova inicialização automática.
Não copie banco, senha, device.station ou peer.encryption para outro aparelho.
O arquivo compartilhável é exclusivamente
~/Downloads/TitanSystem_PC53_Descritor_Publico.json.

## Demonstração

O instalador executa testes Python e Go e uma demonstração real de init,
inspect, check e peer antes de criar o commit. O check termina ambos os
processos que criou, sem matar processos externos. A instalação fica em disco.
Para manter a API local funcionando, execute na raiz do projeto:

    python3 -B tools/titan_local_lab.py serve

Em outra janela, acesse:

    http://127.0.0.1:18181/local/v1/health

Para verificar novamente, encerre o serve com Ctrl+C e execute:

    python3 -B tools/titan_local_lab.py check

Os executáveis e o SQLite persistem após parar o servidor. Após baixar módulos
e compilar, esses comandos locais não requerem PostgreSQL nem a API online.
Os testes HTTP usam somente loopback e ignoram proxies. Uma demonstração com
a internet fisicamente desconectada ainda depende do PC: desligue somente a
internet e repita serve/health/check, mantendo sua rede local quando houver.

## Limites explícitos

Não existe comunicação PC-PC demonstrada nesta entrega. O descritor público
prepara a etapa de pareamento; não concede acesso a um parceiro nem sincroniza
dados. Cada aparelho precisa de sua própria identidade e aprovação, não de
cópia do SQLite principal. Acesso LAN requer o perfil HTTPS apropriado; esta
entrega fica restrita a loopback.

Sem chave emissora/contrato válido, titan-local mantém cadastros comerciais
indisponíveis, como já previsto no núcleo. Esta entrega não contorna licenças,
cria vendas fictícias ou declara aplicativo móvel pronto. Cadastro inicial
online também continua separado deste laboratório local.

Validação de preparação: sete testes Python de privacidade, falha preservada,
exportação e sessão; testes/vet Go dos comandos locais; demonstração contra
SQLite e HTTP reais, com logout e reinício. O aceite no PC deve repetir a
demonstração e confirmar o commit, sem push automático.
