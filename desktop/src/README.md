# Processo desktop

Esta pasta contém somente infraestrutura do Electron: conexão, transporte e entrega dos arquivos compilados. As telas de PDV, financeiro, estoque, frota e demais módulos continuam em `../../frontend-web/src/`.

Não criar outro ERP aqui. O backend Go continua sendo serviço separado e autorizado, local ou hospedado conforme os contratos de implantação. Ele não roda dentro de uma página HTML.
