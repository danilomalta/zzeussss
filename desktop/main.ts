const { app, BrowserWindow, ipcMain } = require('electron');
const path = require('path');

/*
ELECTRON DESKTOP MAIN PROCESS
=============================
Este arquivo gerencia o ciclo de vida do aplicativo desktop do PDV.

Integração de Hardware (Regras de Negócio):
1. [IMPRESSORAS]: Comunica-se com as portas seriais/USB para impressão térmica de comprovantes não fiscais e Danfe NFCe.
2. [BALANÇAS]: Escuta continuamente a porta serial (ex: COM3) para capturar o peso do produto no checkout.
3. [TEF/PINPAD]: Encapsula as bibliotecas DLL do TEF para transação de cartão de crédito.
4. Segrega totalmente a execução do Node.js (backend desktop) do renderizador React via `preload.js` por motivos de segurança.
*/

function createWindow() {
  const mainWindow = new BrowserWindow({
    width: 1280,
    height: 800,
    kiosk: false, // Pode ser alterado para true em produção para travar a tela do PDV
    webPreferences: {
      preload: path.join(__dirname, 'preload.js'),
      nodeIntegration: false,
      contextIsolation: true
    }
  });

  // Em dev, apontaria para o Vite na porta 3000
  // Em prod, carregaria o index.html buildado
  mainWindow.loadURL('http://localhost:3000/pos');
}

app.whenReady().then(() => {
  createWindow();

  app.on('activate', function () {
    if (BrowserWindow.getAllWindows().length === 0) createWindow();
  });
});

app.on('window-all-closed', function () {
  if (process.platform !== 'darwin') app.quit();
});
