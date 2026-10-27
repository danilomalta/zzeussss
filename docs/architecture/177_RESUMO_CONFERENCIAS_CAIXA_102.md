# Frontend102 — Resumo e separação das conferências do caixa

Somente UI/UX, estilos e traduções; requer frontend101. Não altera Go, SQL, APIs, pagamento, estoque, contagem, autorização, sessões ou persistência. Reaproveita GET oficial da B02B2 já integrado na101.

## Tela
PDV → Relatórios. Após Consultar relatório oficial, mostra esperado, contado e diferença em três cartões. Texto e sinal identificam falta, sobra ou ausência de diferença, sem depender apenas de cor. A composição completa fica recolhida em Ver composição do fechamento.

Conferência local tem seção própria: pagamentos/comprovantes e contagem/malotes expansíveis. Valores salvos anteriormente do fechamento só aparecem quando há registro, sob Registros anteriores, com ressalva sobre a situação atual no servidor. Sem quadro duplicado de treze traços quando não há registro.

CSV/impressão oficiais continuam usando os dados oficiais validados da101. CSV/impressão locais continuam usando os cálculos e registros da98 e agora têm rótulos explícitos. Nenhuma soma entre as fontes. Mudanças de turno/usuário continuam descartando a visualização oficial antiga. O botão de consulta não encerra caixa nem realiza cobrança.

## Arquivos e tradução
Dois componentes existentes ajustados: Report98.tsx e OfficialReport101.tsx. Novos estilos report102.css, words102.ts, cinco arquivos JSON de11 chaves, teste de cobertura de idiomas e harness browser102. Este documento registra a entrega. Sem dependências novas. en/es/fr/zh-CN precisam de revisão por falante nativo antes da produção; nenhuma chave nova vazia ou ausente.

## Testar
1. Consultar caixa encerrado com B02B2. Conferir esperado, contado e diferença; abrir composição e comparar os mesmos valores no CSV/impressão oficial.
2. Testar diferenças zero, negativa e positiva. Conferência local não muda os cartões oficiais.
3. Abrir comprovantes e contagem/malotes. Baixar CSV local e verificar que os valores oficiais da consulta não foram misturados.
4. Alternar turno arquivado e atual. O relatório oficial antigo desaparece até nova consulta.
5. Cinco idiomas, temas claro/escuro e larguras1440/768/390. Detalhes abrem por Enter/Espaço e foco visível já existente. Tabela local conserva rolagem interna no celular.

## Evidência e limites
30 testes focados e regressões de modelo; TypeScript/Vite; Firefox com requisições interceptadas de exemplo, todos conforme VALIDACAO.txt. Os testes do navegador cobrem cartões, três estados, detalhes recolhidos, CSV separado, troca de arquivo e5 idiomas ×3 larguras ×2 temas. Não executados Go/servidor real, impressora física, adquirente, SEFAZ nem aceite no PC do proprietário. Sem novas rotas necessárias. A integração real da101/B02B2 entre branches permanece dependente do trabalho no chat de backend.
