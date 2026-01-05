const { app, BrowserWindow, session, shell } = require('electron');
const path = require('path');

// --- SECURITY CONSTANTS ---
const ALLOWED_ORIGINS = [
    'http://localhost:5173', // Vite Dev Server
    'https://titan-system.com' // Production Domain
];

function createWindow() {
    // 1. Secure Window Configuration
    const mainWindow = new BrowserWindow({
        width: 1280,
        height: 720,
        backgroundColor: '#09090b', // Titan Prime Dark BG
        autoHideMenuBar: true,
        kiosk: false,
        fullscreen: true,
        webPreferences: {
            // --- CORE SECURITY ---
            contextIsolation: true, // ISOLATION: Renderer cannot access Node.js
            nodeIntegration: false, // NO NODE: Renderer cannot require() modules
            sandbox: true,          // SANDBOX: Chrome-style sandbox
            webSecurity: true,      // CORS: Enforce Same-Origin Policy
            allowRunningInsecureContent: false,
            preload: path.join(__dirname, 'preload.js')
        }
    });

    // 2. Load Content
    mainWindow.loadURL('http://localhost:5173');

    // 3. Block New Windows (Popups)
    mainWindow.webContents.setWindowOpenHandler(({ url }) => {
        if (url.startsWith('https:')) {
            shell.openExternal(url);
        }
        return { action: 'deny' }; // BLOCK ALL POPUPS
    });

    // 4. Prevent Navigation to Unauthorized Sites
    mainWindow.webContents.on('will-navigate', (event, url) => {
        const parsedUrl = new URL(url);
        if (!ALLOWED_ORIGINS.includes(parsedUrl.origin)) {
            event.preventDefault();
            console.log(`🚫 Blocked navigation to: ${url}`);
        }
    });
}

// --- APP LIFECYCLE ---

app.whenReady().then(() => {
    // 5. CSP Injection (Content Security Policy)
    session.defaultSession.webRequest.onHeadersReceived((details, callback) => {
        callback({
            responseHeaders: {
                ...details.responseHeaders,
                'Content-Security-Policy': [
                    "default-src 'self' http://localhost:*; script-src 'self' http://localhost:*; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:; font-src 'self' https: data:;"
                ]
            }
        });
    });

    createWindow();

    app.on('activate', () => {
        if (BrowserWindow.getAllWindows().length === 0) {
            createWindow();
        }
    });
});

app.on('window-all-closed', () => {
    if (process.platform !== 'darwin') {
        app.quit();
    }
});
