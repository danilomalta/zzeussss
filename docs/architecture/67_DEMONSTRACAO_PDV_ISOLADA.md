# Demonstracao integrada do PDV em instalacao separada

Base esperada no PC: `3ed2457`. Preparacao por `tools/demo_pdv.py`, usando as
CLIs e APIs reais da copia local. Nao usa INSERT manual para instalar contratos,
produtos, estoque, caixa ou vendas. Nao modifica o banco de trabalho existente.

## Executar

Na raiz do repositorio, depois de aplicar e testar este patch:

```bash
python3 tools/demo_pdv.py --check
python3 tools/demo_pdv.py
```

Escolher e repetir uma senha de teste de 12 a 72 bytes. A digitacao nao aparece.
Depois, abrir o endereco e copiar o ID do operador que o terminal mostrar.
O login utiliza essa senha escolhida; nao existe senha oculta para procurar.

O terminal permanece aberto enquanto API e Vite rodam. Ctrl+C encerra somente
os dois processos criados pelo preparador. Processos existentes nao sao
encerrados. As portas padrao desta demonstracao sao 8182 (API, loopback) e 3001
(Vite, loopback). Se ocupadas, o preparador para antes de criar uma instalacao:

```bash
python3 tools/demo_pdv.py --api-port 8183 --web-port 3002
```

Cada execucao cria uma pasta nova privada em
`~/.local/share/titansystem-tests/pdv-*`, mantida apos o encerramento. Nao se
apagam nem reutilizam instalacoes ou chaves anteriores. A configuracao normal
do Vite e a API padrao na porta 8181 continuam com seus destinos originais.

## Conteudo de teste

- Empresa, loja, dono e aparelho novos, autenticacao e prova do aparelho reais.
- Chave emissora administrativa exclusivamente de teste em subpasta privada;
  apenas sua chave publica e passada ao servidor desta demonstracao.
- Contrato assinado vigente por dois dias, com POS e dependencias, instalado
  pela API apos conferir empresa, loja, operador e aparelho da sessao.
- Uma gondola, Cafe a R$ 8,99 por unidade (20 unidades) e Arroz a R$ 10,00/kg
  (10 kg), cadastrados pelas APIs com entradas reais de estoque de teste.
- Conferencia do saldo via API e caixa inicialmente fechado.
- `demo.json` com IDs publicos e valores de teste. Sem token, senha ou chave
  privada. A senha nao e gravada em texto; o banco guarda o hash do login.

As chaves e o contrato desta demonstracao servem somente para a empresa nova.
Nao distribuir a chave privada emissora nem importar a chave publica de teste
na instalacao de trabalho. Isto nao substitui a emissao comercial de licencas.

## Aceite manual da venda

1. Fazer login em `http://127.0.0.1:3001/local/login` com o ID exibido e a senha
   de teste escolhida. Abrir o PDV pelo catalogo.
2. Abrir caixa com R$ 100,00. Selecionar **Gondola de teste**.
3. Adicionar 1 unidade de Cafe (R$ 8,99) e 0,500 kg de Arroz (R$ 5,00).
4. Ver total R$ 13,99. Informar recebido R$ 20,00; ver troco R$ 6,01.
5. Concluir e verificar **Venda confirmada**, ID persistido e total R$ 13,99.
6. Atualizar a pagina, entrar novamente e conferir que o mesmo turno continua
   aberto. Nao repetir a venda: a operacao confirmada ja terminou.
7. Se nao houve outras operacoes, fechar com contagem R$ 113,99. Conferir
   fechamento confirmado e diferenca zero.

Se aparecer pendencia, usar **Verificar / reenviar mesma operacao** antes de
iniciar outra venda. O preparador nao faz a venda nem fecha o caixa sozinho;
esses passos devem ser demonstrados pela interface.

## Validacao e limites

- Testes Python: porta ocupada, caminho externo/redirecionamento, respostas
  incompatíveis, mensagens sem dados privados, contexto estrangeiro antes de
  qualquer cadastro, arquivo exclusivo 0600 e encerramento de processo proprio.
- Teste Go: portas invalidas rejeitadas antes de abrir/criar arquivos.
- Sintaxe do inicializador Vite e testes/build existentes do frontend.
- Go, compilacao dos executaveis e demonstracao real precisam ser executados
  no PC com toolchain e dependencias disponiveis.

Uma falha preserva a pasta de teste e nao reenvia cadastros automaticamente.
Os logs privados ficam nessa pasta. Nao anexar station.json, private.json, banco
ou logs sem revisao; o estado publico e o resultado mostrado pela tela bastam
para a primeira conferencia.
