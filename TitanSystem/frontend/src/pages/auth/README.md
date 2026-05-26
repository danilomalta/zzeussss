# `src/pages/auth/` (Telas de autenticação)

Esta pasta contém as telas relacionadas a autenticação do TitanSystem (web):

- login
- recuperação de acesso
- (futuro) cadastro e troca de senha, se aplicável

## Regras de segurança (frontend)

- Nunca armazenar senha em armazenamento persistente do navegador.
- Evitar logs de console com dados sensíveis.
- Tratar mensagens de erro sem revelar detalhes (ex.: não distinguir “usuário existe” vs “senha errada”).
- Implementar limitação de tentativas no backend; o frontend apenas respeita e exibe o estado (ex.: “aguarde X segundos”).

## Como se conecta ao resto do sistema

- Essas telas consomem a API do backend (rotas de autenticação).
- O desktop (Electron) reutiliza essas telas ao empacotar o frontend web.

