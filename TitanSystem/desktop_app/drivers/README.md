# TitanSystem: Integração Física de Hardware (Drivers)

**O que você vai codar aqui**: 
Código que converte a saída de recibos para linguagens de impressora fiscal, não-fiscal genéricas, balanças Toledo/Filizola e leitores de código de barras seriais.

**Linguagem**: C++ nativo compilado (via Node-Gyp) ou Node.js SerialPort, além da impressão via CUPS no Linux e Spooler do Windows ESC/POS.

**Processo e Regras**:
1. Abstrair "Comanda", "Cupom Não Fiscal" e "DANFE" de forma que as rotas chamem um serviço único.
2. Usar o buffer binário para se comunicar via USB ou rede (Socket) com as impressoras.

**Barreira de Segurança Contras Vazamentos**:
* Se o cliente tentar injetar scripts via código QR do papel de recibo, o script irá ser serializado em byte antes de chegar à interface de OS do Linux/Windows. O Driver deve rejeitar binários inesperados vindos do leitor e só aceitar formato EAN13 ou UTF-8 saneado.
