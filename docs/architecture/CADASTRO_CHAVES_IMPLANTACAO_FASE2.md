# Fase 2 — chaves do cliente e destino explícito

## Custódia

O cliente executa a instalação em infraestrutura que ele controla. Nenhuma chave
é criada no provedor de ERP, enviada a servidor Titan ou recuperável por conta
mestra. Quem administra o host e o processo pode tecnicamente ler memória e
arquivos: o fornecedor do ERP não deve possuir esse acesso. O código não promete
proteger segredos de um administrador da infraestrutura que o próprio cliente
escolheu autorizar.

Cada entrada fiscal passa a aceitar `vault_path` e `master_key_file`, ambos
privados e definidos na configuração da instalação, nunca no HTTP. O cofre fica
vinculado criptograficamente a um tenant. O carregamento recusa reutilizar cofre
ou chave entre clientes. A abertura verifica uma marca criptografada inclusive
em cofre vazio; chave errada ou ausente recusa iniciar o provedor.

A compatibilidade de uma instalação antiga com FISCAL_MASTER_KEY é restrita a
um cliente por cofre. Arquivos legados que misturam tenants são recusados:
precisam de migração explícita autorizada, não são divididos nem apagados
automaticamente. Nenhum cofre real foi aberto ou migrado nesta entrega.

## Comandos locais

Escolher diretório privado existente na instalação do cliente, fora do Git.
Os caminhos abaixo são estruturais; não executar sobre arquivos existentes.

```sh
titan-fiscal vault-init --vault /dados-cliente/cofre.sqlite --key-file /dados-cliente/master.key --lang pt-BR
titan-fiscal vault-export-key --key-file /dados-cliente/master.key --out /backup-cliente/master.key --lang pt-BR
```

A chave aparece uma vez no terminal privado, nunca em stdout, log ou resposta
HTTP. Os arquivos são privados, criados sem sobrescrita. O cliente escolhe e
protege o destino de backup. Não há exportação de chave pelo navegador.

Antes da rotação, o cliente deve parar o serviço e confirmar o backup antigo:

```sh
titan-fiscal vault-rotate --vault /dados-cliente/cofre.sqlite --key-file /dados-cliente/master.key --out /dados-cliente/master-nova.key --backup-confirmed --lang pt-BR
```

A rotação recifra registros e verificação da chave em uma transação; processos
com chave antiga não conseguem gravar depois. Atualizar `master_key_file` na
configuração privada para o arquivo novo e reiniciar o serviço. Exportar o novo
backup. A chave antiga não é removida: backups antigos ainda podem exigir essa
chave. Em falha, arquivos preparados são preservados para investigação; nenhum
backup é apagado ou substituído automaticamente. Perder a chave perde os
segredos; o fornecedor não consegue recuperá-los. Aviso visível nos cinco idiomas
na tela de cadastro da empresa e no comando local.

Exemplo de entrada fiscal sem nenhum valor de segredo:

```json
{
 "vault_path": "/dados-cliente/cofre.sqlite",
 "master_key_file": "/dados-cliente/master.key",
 "customer_managed": true,
 "provider": "external",
 "environment": "homologation",
 "crt": "1",
 "certificate_type": "A1",
 "binding": {"tenant_id":"cliente","store_id":"loja","device_id":"estacao","profile_id":"customer-setup","profile_revision":1}
}
```

Token/CSC continuam entrando somente pelo proprietário autenticado. Certificados
Own usam o mecanismo de cofre já existente; não foi criado upload A3 nem acesso
de suporte. A política fiscal de produção do checkout não foi relaxada.

## Local, servidor próprio e nuvem

Perfis existentes versão1 local/servidor permanecem. Nuvem/híbrido versão1 não
viram outra coisa silenciosamente. Novo perfil **versão2, mode cloud** executa
este mesmo núcleo SQLite em host do cliente, no caminho `--db` escolhido por ele.
Não é a integração PostgreSQL/híbrida de versão1, não fornece HA nem sincronização
entre bancos. O cliente pode usar conexão HTTPS remota no desktop para esse host.

O perfil exige endereço numérico de escuta explícito, certificado TLS válido
para esse endereço, chave privada e CIDRs permitidos explícitos; sem abertura
0.0.0.0/0 ou ::/0 por padrão. Autenticação, aparelho, papéis e licenças continuam
na API. O destino e as chaves são configurados pelo cliente, sem endereço central
Titan embutido e sem migração ao trocar conexão.

## Evidências

Testes passaram: fiscalproduction (5,071s), deployment (0,011s), cmd/titan-fiscal
(0,221s), localapi apenas cadastro/onboarding e checkout selecionados (4,516s),
cinco testes JS do cadastro e TypeScript. Sempre Go com timeout120s/count1.
Os testes de isolamento existentes foram fortalecidos com cofres e chaves
independentes, sem retirar verificações. Nenhum serviço ativo ou dado comercial
foi utilizado. Os segredos históricos em .env continuam com revogação externa
não comprovada; esta entrega não altera contas/credenciais reais ou histórico Git.
