# Handlers HTTP (Controladores)
**Linguagem**: Go
**Papel**: Recebem requisições HTTP, parseiam/verificam o JSON e chamam os Services.
**Segurança Militar**: Sanitização obrigatória de todos os JSONs e Query Params via validações estritas `validator` para evitar cross-site scripting back-end.
