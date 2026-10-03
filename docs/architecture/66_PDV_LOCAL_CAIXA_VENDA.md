# PDV local — caixa e venda em dinheiro

Base: `bc5f594`. Integração do frontend com as APIs existentes, sem alteração
do banco, do servidor ou das regras de autorização do backend.

## Caminho demonstrável

1. Iniciar `titan-local serve` com a instalação existente, chaves públicas
   confiáveis em `--issuer-keys` e contrato vigente que habilite POS e suas
   dependências. O login funciona sem licença, mas isso não habilita vendas.
2. Iniciar o Vite com `npm --prefix frontend-web run dev` e abrir
   `http://localhost:3000/local/login`.
3. Autenticar o operador da instalação. No catálogo, clicar **Abrir PDV local**.
4. Abrir o caixa informando o fundo em reais. O identificador do turno é criado
   uma vez, antes do envio.
5. Selecionar uma gôndola real, informar quantidade, buscar produtos na página
   e adicioná-los ao carrinho. Produtos por unidade só aceitam inteiros;
   produtos por peso/volume aceitam até três casas decimais da unidade cadastrada.
6. Informar o dinheiro recebido. O troco é apresentado; o pagamento enviado ao
   backend corresponde ao total da venda, não ao valor entregue antes do troco.
7. Concluir. Apenas depois de consultar o registro persistido, o carrinho é
   liberado para outra venda. O registro exibido é operacional, não fiscal.
8. Com o carrinho vazio, abrir **Fechamento cego**, informar o dinheiro contado e
   confirmar. O valor esperado e a diferença aparecem só após a resposta do
   fechamento; não se consulta saldo esperado durante a contagem.

Dados necessários: produtos, gôndolas (`shelf`) e saldo real na gôndola. A tela
não cria estoque nem dados de demonstração. Preços vêm do catálogo e são
recalculados pelo backend na transação. Um preço alterado depois da seleção pode
recusar a venda; após recusa confirmada e ausência de registro, o carrinho pode
ser refeito com preços atualizados. Falta de estoque também é recusada pelo
backend. O PDV não estima saldo com dados do navegador.

## APIs utilizadas

| Método | Caminho local | Uso |
| --- | --- | --- |
| GET | `/products?limit=50&offset=…` | Catálogo paginado |
| GET | `/locations` | Gôndolas da loja autenticada |
| GET | `/cash/current` | Turno do operador, sem saldo esperado |
| POST | `/cash/open` | Abertura idempotente |
| POST | `/sales` | Venda atômica em dinheiro |
| GET | `/sales/:id` | Confirmação do registro persistido |
| POST | `/cash/close` | Fechamento cego idempotente |

Empresa, loja, aparelho e operador são derivados da sessão pelo servidor.
Os preços, nomes e dados de identidade não são enviados no corpo da venda.
`Location.ID` é atualmente serializado pelo Go como `ID`; o cliente normaliza
esse campo para `id` sem mudar o contrato de outros consumidores.

## Operações sem resposta

- Antes de enviar, salva-se uma pendência no navegador, vinculada ao contexto
  confirmado por `/me`: empresa, loja, aparelho e operador.
- A pendência contém identificadores, quantidades e centavos; não contém
  senha, token de sessão ou nome de produto. É removida após confirmação.
- Uma nova sessão do mesmo operador nesse navegador consegue retomar a
  pendência após atualizar a página ou reiniciar o navegador.
- Web Locks serializa os envios entre abas do mesmo navegador/contexto. Se a
  proteção ou a gravação local não estiver disponível, novas mutações param.
- Venda pendente: consultar seu ID. Encontrada e compatível, confirmar sem
  novo POST. Somente `404` permite reenviar exatamente o mesmo pedido.
- Abertura e fechamento repetem os mesmos identificadores e valores; o
  backend decide a idempotência.
- Erro de transporte, resposta incompatível, erro de consulta ou resultado
  incerto preserva a pendência e bloqueia operações novas.
- Depois de uma recusa explícita sem incerteza anterior, permite-se editar.
  Para venda, exige-se antes uma consulta que confirme ausência do registro.
  Uma recusa posterior não apaga a incerteza de um envio anterior.
- Registro já cancelado é mostrado como cancelado; nunca provoca nova venda.
- Limpar os dados do navegador apaga essa referência local. Ela não substitui
  o banco SQLite nem é uma fila de vendas offline independente do servidor.

## Validação desta entrega

- Build TypeScript/Vite.
- Testes existentes de login e catálogo.
- Testes de dinheiro/peso exatos, respostas incompatíveis, ciclo HTTP com
  respostas controladas, reabertura do estado local, resposta perdida,
  reenvio sem novos IDs, recusa versus incerteza, concorrência, persistência
  indisponível e isolamento do estado entre contextos.
- Aplicação do patch sobre a base Git e `git diff --check`.

Os testes de cliente usam respostas controladas. Não constituem demonstração
contra o Go nem prova em smartphone. O aceite no PC inclui executar os testes
Go existentes e percorrer o ciclo acima em banco de teste preparado e licenciado.

## Limites e próximas integrações

Cartão/Pix, venda móvel com banco próprio, emissão fiscal, tela de cadastro e
entrada de estoque, sangria/suprimento visual, cancelamento visual e descontos
ainda precisam de integrações próprias. A busca filtra os 50 produtos da página;
não simula busca global. Carrinho não enviado fica em memória; somente a operação
que chegou à etapa de envio é preservada. Não há nova dependência npm nem IA.

O transporte entregue em 9A–9L permanece uma base técnica. O roteiro original
de pedido ao fornecedor, doca e recebimento não é concluído por esta tela.
