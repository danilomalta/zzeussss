# Parte1 — fluxos do caixa94

## Entrega de frontend
O pacote é acumulado86–94. Modifica o PDV existente em `/local/pos`, mantendo sua
venda em dinheiro e o fechamento em `/local/cash/register`. Não modifica backend,
senhas, bancos comerciais, dependências ou integrações publicadas. Nenhum push.

| Área no PDV | Fluxo desta entrega | Situação financeira |
|---|---|---|
| Venda / pagamento | Composição de até20parcelas em dinheiro, débito, crédito, Pix e fiado; soma exata; dinheiro entregue cobre somente parte cash; salvar composição, remover e descartar com histórico | Composição é preparação local. Não cobra e não confirma uma venda |
| Venda em dinheiro | Mantém o envio existente, recuperação idempotente, itens confirmados e troco após confirmação | Cliente atual aceita apenas cash; confirmar requer composição simples e nenhum desconto pendente para o carrinho |
| Vendas suspensas | Guarda carrinho sem CPF/custo, retoma com catálogo/preços atuais carregados, preserva histórico e marca concluído após recibo confirmado | Rascunho local, sem venda ou reserva de estoque. Produto fora do catálogo carregado exige atualização antes de retomar |
| Movimentos | Solicitar sangria/suprimento com valor e motivo; registro durável; revisão e retirada preservadas | Não efetiva movimento na API nem muda saldo oficial |
| Autorizações | Desconto com motivo, limitado ao total e vinculado à composição exata do carrinho; revisão por dono/gerente com manage_cash; recusa/retirada auditadas | Revisão é local, sem alteração de preços. Venda integral bloqueada enquanto a solicitação atual não for resolvida/retirada |
| Trocas/devoluções | Selecionar venda/recibo reais, item, quantidade, solução(reembolso/troca/crédito) e motivo; valor proporcional exato; limite cumulativo de quantidade; cancelar venda inteira em solicitação separada | Não apaga a venda, devolve dinheiro ou muda estoque. Pedido ativo de devolução impede pedido integral conflitante |
| Malote | Lê contagem92confirmada; código único; destino; política de duas pessoas ou responsável único; lacre, entrega, recebimento, segunda contagem e diferença com motivo | Cadeia de conferência local; sem transmissão oficial de malote |
| Livro de caixa | Vendas da sessão por parcelas, solicitações com estado e histórico; CSV imediato sem seletor de pasta | Distingue venda registrada da solicitação não efetivada. Não soma pedidos locais ao saldo real |
| Conexões | Cartão, Pix, fiscal, clube, gaveta, impressora, câmera móvel e backup com estado real de integração pendente; acesso às configurações | Nenhum equipamento ou serviço anunciado como conectado |

As solicitações, decisões e malotes ficam em IndexedDB por empresa/loja/aparelho/
usuário e turno. Revisão otimista rejeita gravação sobre alteração de outra janela.
Falha de leitura não autoriza sobrescrever registros vazios. Credenciais, tokens,
CPF do cliente e imagens não são gravados neste livro. Somente campos permitidos
entram nas solicitações. Mudanças não salvas têm aviso de saída.

Um gerente/dono com manage_cash pode consultar, no mesmo navegador e estação,
os registros de outros operadores daquele turno. A enumeração exige essa permissão
e mantém fronteiras de empresa/loja/aparelho/turno; as chaves por usuário permanecem
separadas. Isso permite a segunda pessoa conferir um malote local. Não é sincronização
entre computadores. A autorização definitiva deve continuar no backend.

Descartar ou editar uma composição já salva retira a preparação anterior antes de
alterar a tela. Nunca transforma automaticamente cartão/Pix em dinheiro nem deixa
uma preparação antiga ativa depois de voltar explicitamente ao pagamento simples.

## O que esta entrega NÃO conclui
O frontend da Parte1 ganhou os fluxos acima, mas a Parte1 OPERACIONAL continua
dependente de integração. O cliente oficial atual só admite dinheiro e recibo sem
fiscal autorizado. Cartão/Pix/fiado, pagamento dividido oficial, NFC-e/NF-e, clube,
reembolso, movimento financeiro, aplicação de desconto e hardware continuam sem
contrato servidor demonstrado. O backend oficial não está presente nesta base de
código disponível e não foi alterado pelo pacote. Nenhum endpoint foi inventado.

O saldo oficial esperado da gaveta continua vindo do fechamento existente. Sem
extrato completo de abertura/sangrias/suprimentos/devoluções, o valor de vendas em
dinheiro não pode ser apresentado como saldo da gaveta. Os pedidos locais não são
incluídos no esperado oficial. OCR92 depende de TextDetector; Firefox pode exigir
transcrição. Pareamento de câmera do celular com PC ainda não está implementado.

Vendas suspensas entre aparelhos, pagamento/estorno real, fiado com limite e cobrança,
restauração conjunta de todos os módulos, sincronização comercial PC/mobile e
testes com hardware real não são declarados concluídos por este pacote.

## Contratos de backend necessários
`tillContracts94.ts` descreve a interface tipada, sem URLs ou mock de produção:
- Parcelas por venda, chaves idempotentes, autorização/consulta/estorno/liquidação.
- Extrato completo da sessão, totais por meio e revisão consistente do saldo.
- Movimentos e revisão de solicitações com permissão e auditoria no servidor.
- Desconto autorizado vinculado à revisão do carrinho e preço aplicado na venda.
- Cancelamento/devolução parcial por item, saldo devolvível, estoque e reembolso.
- Malote com cadeia de custódia, identidades e contagem por operador/conferente.
Além disso: emissão fiscal/CPF, clube, drivers, pareamento e backup/restauração.

## Testar
1. Abrir PDV, ler primeiro produto e responder CPF/clube ou pular. No pagamento,
   produto30: compor dinheiro10+cartão20; informar entregue10; salvar composição.
   Não deve aparecer venda concluída nem ocorrer POST de venda por essa ação.
2. Voltar explicitamente ao pagamento simples. A preparação anterior é retirada;
   dinheiro100 → venda30confirmada → troco70. Resposta perdida não duplica venda.
3. Solicitar sangria50; revisar/recusar com motivo. Não subtrair50 do saldo real.
4. Solicitar desconto; confirmar bloqueio de venda integral até retirar/resolver.
5. Solicitar devolução parcial de venda real; tentar exceder a quantidade original.
   A soma dos valores de devoluções parciais não ultrapassa o total original.
6. Fechamento: contar130(abertura100+venda30da prévia), confirmar contagem. Voltar
   ao PDV/Malote, registrar destino e política. Duas pessoas exige identidade
   diferente na recepção/conferência; responsável único exige dono/gerente.
7. Conferir o malote, testar diferença com justificativa, imprimir identificação.
8. Livro: venda30 e pedidos locais separados; baixar CSV sem procurar pasta.
9. Atualizar página: registros permanecem. Duas janelas: revisão antiga é recusada.
10. Testar cashier sem aprovação, manager/owner com manage_cash,5idiomas, temas,
    teclado,100%zoom, tablet e celular. Não confundir prévia com dados comerciais.

Testes automatizados de regras, persistência com IndexedDB simulado, DOM virtual
com API simulada, instalador, TypeScript e Vite não substituem teste visual real,
câmera, impressora, gaveta, maquininha e execução comercial do backend.
Chaves novas completas nos5idiomas; en/es/fr/zh-CN exigem revisão nativa.

Para PCs modestos, a tabela de solicitações tem páginas de50registros. Livro e
atividade mostram registros recentes; CSVs exportam os dados completos carregados.
Sem dependências novas, vídeo contínuo, Excel ou OCR pesado neste pacote.
