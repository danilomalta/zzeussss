const rows={
 title:['Estado operacional da receita','Recipe operational state','Estado operativo de la receta','État opérationnel de la recette','配方运行状态'],
 help:['Este estado tem revisão própria. Inativar impede novos planos; não cancela ordens existentes nem altera versões publicadas.','This state has its own revision. Inactivation prevents new plans; it does not cancel existing orders or change published versions.','Este estado tiene revisión propia. Inactivar impide nuevos planes; no cancela órdenes existentes ni cambia versiones publicadas.','Cet état a sa propre révision. Désactiver empêche les nouveaux plans ; ne supprime pas les ordres existants ni les versions publiées.','此状态有独立修订号。停用会阻止新计划，但不会取消现有生产单或更改已发布版本。'],
 active:['Ativa','Active','Activa','Active','启用'],inactive:['Inativa','Inactive','Inactiva','Inactive','停用'],
 activate:['Revisar reativação','Review reactivation','Revisar reactivación','Vérifier la réactivation','审核重新启用'],
 deactivate:['Revisar inativação','Review inactivation','Revisar inactivación','Vérifier la désactivation','审核停用'],
 unknown:['Estado ainda não confirmado; novos planos bloqueados.','State not yet confirmed; new plans blocked.','Estado aún no confirmado; nuevos planes bloqueados.','État non confirmé ; nouveaux plans bloqués.','状态尚未确认；已阻止新计划。']
} satisfies Record<string,[string,string,string,string,string]>;
export function recipeStateWords(locale:string){const l=locale.toLowerCase(),n=l.startsWith('pt')?0:l.startsWith('es')?2:l.startsWith('fr')?3:l.startsWith('zh')?4:1;return Object.fromEntries(Object.entries(rows).map(([k,v])=>[k,v[n]])) as Record<keyof typeof rows,string>;}
