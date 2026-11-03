# Frontend106 — Conferência de leitura dos comprovantes

Requer a105. A instalação105 no PC foi confirmada por120 testes e build20s. A106 aplica chaves de contexto empresa/loja/operador/caixa nos dois chamadores para descartar leituras pendentes na troca e altera o componente OCR96 compartilhado pelo fechamento92 e conferência pelo celular95. A assinatura onRead(text) foi preservada. Não muda API, Go, SQLite, saldo, adquirente, regras de pareamento, autorização, contrato ou persistência comercial. B01/B02 ficam preservados. Inclui a correção de import type dos três tipos Axios na API cliente, como no PR aprovado no GitHub; altera apenas esse import e aceita o arquivo já corrigido. Não muda URL, timeout, interceptores ou autenticação. Se o arquivo tiver outras diferenças locais, a aplicação para antes de escrever e preserva os arquivos.

## Fluxo entregue
Câmera já aberta pela tela, imagem escolhida explicitamente ou texto colado → leitura local → prévia editável de valor, modalidade, NSU/autorização e data/hora → marcação de conferência → usar campos no formulário existente → registro/conciliação pelas regras existentes. A106 não inicia a câmera por conta própria, não salva imagem e não confirma pagamento nem baixa financeira. Escolher uma imagem é uma alternativa; a câmera continua lendo um frame em memória, sem arquivo de foto permanente.

O comprovante fica na memória da tela enquanto em revisão. Texto bruto é descartado ao preparar os campos. Ao usar ou descartar, os campos são limpos. Bloqueio da interface cancela leitura e limpa campos; retorno assíncrono antigo não preenche o novo estado. Componentes antigos de autorização continuam responsáveis por permitir acesso e submissão. Desmontagem cancela o reconhecimento. Confirmação é exigida antes de substituir uma leitura pendente. Há beforeunload ao fechar/recarregar com texto/campos não usados; navegação interna continua sob os controles existentes do pai, sem promessa de um guard global novo.

## Validações
Não assume débito nem crédito; modalidade ambígua fica vazia. Múltiplos valores, identificadores diferentes ou datas/horários diferentes exigem decisão humana. Não inventa campos ausentes. Valida calendário real, inclusive ano bissexto, sem converter o horário impresso silenciosamente para outro fuso. Valor positivo em centavos exatos, sem arredondar dados inseridos; código de3–40 letras/números/hífens. Isso cobre NSU/autorização aceito pelo leitor atual, não todos os formatos proprietários de adquirentes. Não é validador de autenticidade do comprovante.

Editar um campo remove a confirmação. Clique repetido usa trava síncrona. Se o formulário de destino lançar erro, os campos são preservados. Depois do uso, a próxima leitura começa vazia. O onRead apenas preenche campos, não grava comprovante. Duplicidade e vínculo de comprovantes continuam passando pelos validadores92 existentes quando o comprovante é registrado. O módulo de origem deve continuar validando empresa/loja/operador/caixa; frontend não é barreira de segurança.

Baixa confiança do OCR mostra alerta para revisar todos os campos. O número de confiança é do motor de reconhecimento, não probabilidade de pagamento válido. Leitura incompleta bloqueia o uso até completar/corrigir e marcar conferência. O formulário seguinte ainda pode exigir adquirente e terminal. O papel não comprova liquidação bancária. No exemplo R$315/R$219, o restante continua sendo pendência de conciliação pelas regras anteriores, sem mudar valor de venda.

## Interface e desempenho
Sem novas dependências de produto, OCR carregado somente por ação explícita e worker existente. Usa estilos e tokens neutros da marca; não adiciona azul ou dourado. Campos16px, auxiliares14px, botões44px, layout2colunas/1no celular e foco de teclado.23 chaves novas em pt-BR/en/es/fr/zh-CN, todas preenchidas. en/es/fr/zh-CN precisam de revisão por falante nativo. Os textos de controles da prévia são explicitamente de demonstração, separados dos textos do produto traduzidos.

## Validação
Base de teste: main024dd3f com fontes97–105 recompostas a partir dos pacotes íntegros; nenhuma alteração remota.470 testes do frontend passaram, incluindo11 novos. Typecheck com tsconfig.app.json e build Vite executados. Firefox: revisão obrigatória, edição invalida checkbox, falha preserva campos, duplo clique, ambiguidade, calendário inválido, bloqueio limpa campos, retorno atrasado descartado, alerta de baixa confiança, cinco idiomas ×três larguras ×dois temas e nenhuma requisição a API de aplicação. Motor Tesseract real testado com PNG sintético; campos reconhecidos e confirmação humana ainda obrigatória, com tráfego externo bloqueado. Essa prova sintética não mede precisão de comprovantes impressos amassados, modelos de maquininha, iluminação ou câmera física. Câmera física, transporte entrePC/celular, backend real e aceite comercial não executados nesta entrega.

## Como conferir
1. /local/pos → fechamento de caixa → comprovantes. Leia pela câmera, escolha imagem ou cole texto. Deve abrir a prévia sem registrar automaticamente.
2. Exemplo: VALOR:219,00; CREDITO; NSU:ABC123;19/10/2026 15:32, em linhas separadas. Conferir, marcar checkbox e usar campos. Continuar no formulário de registro existente.
3. Texto com CREDITO e DEBITO deve deixar modalidade em branco. Corrigir; campos obrigatórios completos habilitam checkbox. Editar depois desmarca.
4. Repetir no painel de celular já pareado pela integração existente. Caso o adaptador de pareamento esteja indisponível, a106 não cria nem simula conexão comercial.
5. Testar cinco idiomas, temas, tablet/celular. A prévia fornecida é isolada em memória, sem login/API e sem registrar venda.

## Próximas integrações
Nenhum endpoint novo foi inventado. A conferência usa os contratos já existentes. Execução comercial, liquidação de pagamentos, pareamento físico e fechamento oficial dependem do backend/adaptadores e das permissões reais. A106 não conclui toda a Parte1.
