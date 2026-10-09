# Estado apos P40–P42

Base PC038ba17. P40 publicacao de receitas versionadas com fila persistida,
P41 planejamento por versao e responsavel, P42 decisao humana de aprovar/cancelar.
Novo GET de recibo por tipo/ID, scoped ao operador/aparelho. Sem nova migracao,
schema44 e whitelist/formato/validacoes do backup preservados.

Frontend96/100/104 testes nas respectivas etapas e builds TypeScript/Vite.
Backend: testes especificos de GET, suite completa, vet, race do novo GET.
Todos locais com fixtures descartaveis. Scripts repetem checks antes de commit.
Nenhum servidor iniciado/banco real; aceite visual ainda pendente.

Para concluir esta frente ainda falta:
- Formulario ativo/inativo de receitas; reservas/consumo/resultados; etapas,
  perdas, lotes e qualidade com operacoes pendentes e revisoes preservadas.
- Catalogo/estoque avancados com concorrencia e verificacao de saldo.
- Autorizar/receber/recusar/corrigir compras visualmente; intercambio real depende
  dos contratos de integracao entre empresas e nao deve ser simulado.
- Aceite de ponta a ponta no navegador e revisao das duas branches juntas.
Main/auth/infra/recuperacao sao do outro chat. Nenhuma integracao silenciosa.
