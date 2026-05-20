# TitanSystem: Certificados e Criptografia em Trânsito

**O que você vai codar aqui**: 
Scripts para validar Certbot (Let's Encrypt), certificados autoassinados (para On-Premise sem domínio) e certificados TLS das máquinas clientes na API local.

**Linguagem**: Bash e Configurações Traefik / Nginx reverse proxy.

**Processo e Regras**:
1. Toda rota do TitanSystem (Web e Mobile para o Backend Central em Go) será encriptada `https://`. Nenhum tráfego claro será admitido.

**Barreira de Segurança Contras Vazamentos**:
* Proteção contra chaves fracas. Renovar certificados. Nenhuma chave privada local `.key` deve dar commit. Isso ficará ignorado do git.
