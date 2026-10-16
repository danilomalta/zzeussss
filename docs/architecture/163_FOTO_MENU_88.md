# Foto de fundo e hierarquia do menu — entrega88

## UI

- Configurações → Aparência e fundo: foto sem desfoque ou desfoque mínimo (3px). O padrão anterior continua sem desfoque. A foto só recebe o efeito em modo de fundo personalizado; “Reduzir efeitos” desativa o filtro para máquinas lentas, sem alterar a preferência salva nem a imagem original.
- O menu já apresentava o PDV em um card maior. Conservamos esse destaque, com seu estado em validação visível. Estados repetidos em cards comuns deixam de disputar atenção visual: a informação continua disponível para leitor de tela e na legenda expandível. “Arquivo local” e estados indisponíveis continuam sinalizados visualmente. Nenhum módulo foi promovido a “Operacional”.
- A barra de contexto mostra o nome de exibição local quando configurado; caso contrário, usa “Empresa vinculada” e “Loja vinculada”. A sessão continua sendo verificada pelo servidor; o nome local não é apresentado como nome oficial. A explicação no painel de contexto continua disponível.

## Estado do ERP

Esta entrega é de UI. Não implementa fiscal, pagamentos, financeiro, integrações bancárias, e-commerce ou backup operacional. A lista e a profundidade pendentes continuam no inventário da entrega87. Preferir completar fluxos nos módulos pais antes de adicionar cartões novos de telas sem operações reais.

## Verificação

- Antigas preferências sem o campo novo são normalizadas para desfoque 0; valores fora de 0/3 são rejeitados.
- Teste de interface no DOM virtual cobriu leitura da preferência antiga, prévia instantânea, gravação e preservação do nome da empresa. Todos os idiomas têm as novas chaves.
- Testes de contexto/menu/configuração e build TypeScript/Vite passaram. Nenhum backend ou banco comercial foi alterado. Inspeção visual em navegador real ainda pendente.
- O editor de planilhas existente tem bundle grande e continua pendente de otimização para PCs antigos.
