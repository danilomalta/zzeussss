# Identidade visual do TitanSystem

Referência aprovada pelo dono em 2026-10-02: imagem lado a lado de login claro e escuro, com cartão central, marca azul em forma de T, ondas azuis suaves e controles discretos. Esta direção visual passa a orientar as próximas telas. O texto de interface usa **Titan**, nome curto do produto TitanSystem. A arte serve de referência para composição e cores; campos e ações correspondem às APIs de cada instalação.

## Regras constantes

- Fundo limpo em branco frio no claro e azul quase preto no escuro, com duas ondas azuis suaves atrás do conteúdo. Não colocar texto operacional sobre essas ondas.
- Cabeçalho discreto: marca à esquerda e alternância `Claro / Escuro` à direita. Preferência salva no navegador e aplicada a todas as páginas. Azul principal `#1767f5`; texto de alto contraste e bordas leves.
- Cartão central com largura próxima de 380 px, cantos moderados, sombra suave e respiro. Título direto e subtítulo curto. Campos com rótulo externo, ícone à esquerda, foco visível e botão primário azul.
- Em telas maiores, conteúdo operacional em painéis claros/escuros de mesma família, respeitando largura e leitura; no celular, cartão e barras se reorganizam sem zoom horizontal. Temas mantêm os mesmos espaçamentos, hierarquia, logo, componentes e estados.
- Mensagens informam o estado real. Sem botão que finja cadastrar, recuperar senha, pagar, vender ou transmitir enquanto a API correspondente não existir. Incluir loading, erro e estado vazio legítimos. Acessibilidade: rótulos, foco, contraste e redução de movimento.
- A IA permanece em módulo/rota própria e não aparece por padrão em login, caixa, assinatura ou catálogo. Recursos contratados e permissões devem ser validados pelo backend antes de mostrar resultados sensíveis.

## Aplicação atual

O componente `BrandShell` centraliza fundo, marca, tema e rodapé; `SignInCard` fornece os dois logins com campos reais distintos: e-mail online e ID do operador local. `LocalCatalog` consulta a API existente e usa a mesma família visual. As antigas telas online sem dados reais mostram seu estado de integração. `brand.css` contém os tokens usados nas páginas futuras.

O cadastro comercial, a recuperação de senha e a cobrança web dependem de backend próprio e não estão concluídos por esse ajuste visual. Um novo formulário deve ser ligado ao endpoint correspondente antes de substituir as mensagens informativas atuais.
