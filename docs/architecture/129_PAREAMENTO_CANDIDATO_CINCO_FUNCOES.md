# Entrega 54 — Identidade candidata e pareamento público

Base no PC: main, d316551. Não toca produção, PostgreSQL ou dados comerciais.
O aceite usa exclusivamente o SQLite isolado criado pela entrega53.

## Cinco funções

1. Preparar candidato: `titan-peer candidate-init` gera um ID e duas chaves
   próprias, Ed25519 e X25519, a partir do escopo declarado no descritor público
   do principal. Exige ambas as impressões digitais conferidas. Não cria SQLite,
   associação à empresa, aprovação ou permissão de receber eventos.
2. Inspecionar troca: `titan-peer pair-inspect --in ARQUIVO` valida a estrutura
   assinada de desafio ou resposta e informa tipo, impressões públicas e prazo.
   Sempre informa approved=false: inspecionar não concede autorização.
3. Iniciar desafio: `pair-start` exige dono autenticado e impressões do candidato;
   registra pending e gera desafio assinado com prazo de até cinco minutos.
4. Responder: `pair-answer` exige a chave privada do candidato e a impressão do
   solicitante conferida; prova posse da chave sem aprovar o aparelho.
5. Concluir: `pair-finish` exige novamente dono autenticado, candidato e desafio
   correspondentes; registra prova e aprovação. Repetir o mesmo resultado é
   idempotente, sem repetir os três eventos de auditoria.

As etapas3–5 já existiam no núcleo; esta entrega integra a geração real de
candidato e inspeção para torná-las operacionais sem fabricar chaves de teste.

## Aceite automático local

O instalador executa testes Go, detector de corrida, vet, quatro testes Python
e a demonstração com os executáveis compilados. O demonstrador:

- cria candidato independente no diretório privado novo;
- confirma pending após o desafio e após a prova;
- aprova somente na etapa do dono;
- repete a conclusão e exige auditoria inalterada;
- exige ausência de confiança de transporte/permissões de eventos implícitas;
- verifica que arquivos públicos não contêm senha ou chave privada.

Estado: ~/.local/share/titansystem/pair54 (0700; arquivos privados 0600).
O SQLite principal é o laboratório local53, com identificação explícita de
laboratório. O candidato é local ao mesmo PC nesta demonstração. O demonstrador
usa sua própria chave candidata; não aprova descritores recebidos de terceiros.
Arquivo incompleto bloqueia uma nova execução sem sobrescrever dados.

Depois da instalação, na raiz do projeto:

    python3 -B tools/titan_pair_lab.py verify

## Fluxo entre aparelhos — ainda requer aceite em hardware

Compartilhe exclusivamente arquivos públicos. Nunca copie banco, device.station,
peer.encryption, senha ou pasta privada do principal.

No segundo PC, compile o mesmo pacote titan-peer. Execute candidate-init com
--parent apontando para o descritor público do principal, --dir apontando para
um diretório absoluto novo e --signing-sha256 / --encryption-sha256 com valores
conferidos diretamente com o dono por canal autenticado. Ler o arquivo e copiar
seus hashes sozinho não comprova a origem.

Devolva apenas peer-public.json. O principal verifica os hashes diretamente
com o segundo PC e executa pair-start com --db, --station, --owner, --in, --out,
--name e ambas as impressões. A senha do dono entra pelo stdin, nunca em flags.
Envie apenas o desafio público. O candidato executa pair-answer com --station,
--in, --out e --requester-sha256 conferido. Devolva a resposta pública.
O principal conclui pair-finish com os mesmos escopos e impressões.
Toda a troca precisa respeitar os cinco minutos; arquivos existentes não são
sobrescritos. Inspeção de arquivo expirado não renova prazo.

## Limite de conclusão

O candidato NÃO tem banco secundário nem API local: este é o preparo criptográfico
e a aprovação no banco principal. O próximo provisionamento deve instalar sua
própria base sem copiar a identidade ou conceder autorizações artificialmente.
Pareamento não substitui confiança X25519 ou aprovação por tipo de evento.
Não habilita transporte em rede, sincronização, reconciliação, HTTPS LAN ou mobile.
Esses aceites permanecem separados. Não anunciar dois PCs sincronizados por
causa do sucesso desta demonstração no mesmo computador.
