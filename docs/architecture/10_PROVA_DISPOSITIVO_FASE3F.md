# Fase 3F — prova de dispositivo após pareamento

A migração SQLite `0005_device_auth_challenges.sql` armazena desafios de 32 bytes com validade de um minuto e consumo único. `IssueDeviceChallenge` fornece desafio apenas a dispositivo aprovado. `CompleteDeviceChallenge` verifica assinatura Ed25519 da mensagem `DeviceAuthMessage`, com empresa, loja, aparelho, ID do desafio e nonce. Ela consulta novamente o estado `approved` ao consumir o desafio, inclusive para impedir uso após revogação. O retorno identifica somente o **dispositivo**, não um funcionário.

A chamada que expuser esse serviço deve limitar taxa de solicitações e ligar o resultado a um canal protegido. Nenhum token de sessão de usuário foi criado; a API, o app mobile e o login offline ainda não chamam estas funções. O código não declara o celular pronto para vender. Relógios divergentes e autenticação posterior à conexão exigem política separada. A chave privada Ed25519 permanece no aparelho; criar/guardar essa chave é trabalho do aplicativo e da plataforma.

Testes em SQLite descartável cobrem assinatura correta, contexto de outra empresa, repetição, desafio expirado e revogação antes de completar o desafio. Não aplicar migração em banco real nesta etapa.
