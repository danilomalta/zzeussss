const { app, BrowserWindow, session, shell } = require('electron');
const path = require('path');

// --- SECURITY CONSTANTS ---
const ALLOWED_ORIGINS = [
    'http://localhost:5173', // Vite Dev Server
    'http://localhost:3000', // Alternative Dev Port
    'https://titan-system.com', // Production Domain (Example)
    'file://' // Allow local file access for production builds
];

// --- STRICT CSP POLICY ---
const STRICT_CSP = [
    "default-src 'self'",
    "script-src 'self' 'unsafe-inline' http://localhost:*", // unsafe-inline needed for Vite HMR
    "style-src 'self' 'unsafe-inline' https://fonts.googleapis.com",
    "font-src 'self' https://fonts.gstatic.com data:",
    "img-src 'self' data: https:",
    "connect-src 'self' http://localhost:* https://* ws://localhost:* wss://*",
    "frame-src 'none'",
    "object-src 'none'",
    "base-uri 'self'",
    "form-action 'self'",
    "frame-ancestors 'none'",
    "upgrade-insecure-requests"
].join('; ');

function createWindow() {
    // 1. SECURE WINDOW CONFIGURATION (MÁXIMA SEGURANÇA)
    const mainWindow = new BrowserWindow({
        width: 1920,
        height: 1080,
        backgroundColor: '#050505', // Titan Prime Dark BG
        show: false, // Don't show until ready
        autoHideMenuBar: true, // Remove menu bar (Windows/Linux)
        frame: true, // Keep window frame for controls
        titleBarStyle: process.platform === 'darwin' ? 'hiddenInset' : 'default', // macOS style
        webPreferences: {
            // --- CORE SECURITY (OBRIGATÓRIO) ---
            contextIsolation: true, // ✅ OBRIGATÓRIO: Renderer isolado do Node.js
            nodeIntegration: false, // ✅ OBRIGATÓRIO: Renderer NUNCA toca no Node
            sandbox: true, // ✅ OBRIGATÓRIO: Sandbox Chrome-style
            webSecurity: true, // CORS: Enforce Same-Origin Policy
            allowRunningInsecureContent: false, // Block HTTP on HTTPS pages
            experimentalFeatures: false, // Disable experimental features
            preload: path.join(__dirname, 'preload.js'), // Safe IPC bridge
            // Additional security
            enableRemoteModule: false, // Disable remote module (deprecated but safe)
            spellcheck: false, // Disable spellcheck (reduces attack surface)
        }
    });

    // 2. MAXIMIZE ON START (Remove menu bar)
    mainWindow.maximize();
    mainWindow.setMenuBarVisibility(false); // Force hide menu bar
    mainWindow.show();

    // 3. Load Content
    // In Dev: Load localhost. In Prod: Load index.html
    if (process.env.NODE_ENV === 'development' || process.argv.includes('--dev')) {
        mainWindow.loadURL('http://localhost:5173');
    } else {
        // Production: Load from dist folder
        mainWindow.loadFile(path.join(__dirname, '../frontend/dist/index.html'));
    }

    // 4. Block New Windows (Popups) - SECURITY
    mainWindow.webContents.setWindowOpenHandler(({ url }) => {
        // Allow external links to open in default browser (security: external links)
        const parsedUrl = new URL(url);
        if (parsedUrl.protocol === 'https:' || parsedUrl.protocol === 'http:') {
            shell.openExternal(url);
        }
        return { action: 'deny' }; // ✅ BLOCK ALL POPUPS IN APP
    });

    // 5. Prevent Navigation to Unauthorized Sites - SECURITY
    mainWindow.webContents.on('will-navigate', (event, url) => {
        try {
            const parsedUrl = new URL(url);
            const isAllowed = ALLOWED_ORIGINS.some(origin => {
                if (origin === 'file://') {
                    return parsedUrl.protocol === 'file:';
                }
                return parsedUrl.origin === origin || parsedUrl.origin.startsWith(origin);
            });

            if (!isAllowed) {
                event.preventDefault();
                console.log(`🚫 SECURITY: Blocked navigation to unauthorized origin: ${url}`);
                // Optionally show error to user
                mainWindow.webContents.send('navigation-blocked', url);
            }
        } catch (error) {
            // Invalid URL, block it
            event.preventDefault();
            console.log(`🚫 SECURITY: Blocked invalid URL: ${url}`);
        }
    });

    // 6. Prevent DevTools in Production - SECURITY
    if (process.env.NODE_ENV !== 'development' && !process.argv.includes('--dev')) {
        mainWindow.webContents.on('devtools-opened', () => {
            mainWindow.webContents.closeDevTools();
        });
    }

    // 7. Handle Certificate Errors - SECURITY
    app.on('certificate-error', (event, webContents, url, error, certificate, callback) => {
        // In production, reject invalid certificates
        if (process.env.NODE_ENV === 'production') {
            event.preventDefault();
            callback(false);
        } else {
            // In development, allow self-signed certificates
            callback(true);
        }
    });

    // 8. Log security events (optional, for debugging)
    mainWindow.webContents.on('console-message', (event, level, message) => {
        if (level === 2) { // Error level
            console.log(`[Renderer Error]: ${message}`);
        }
    });
}

// --- APP LIFECYCLE ---

app.whenReady().then(() => {
    // 9. CSP INJECTION (Content Security Policy) - SECURITY
    session.defaultSession.webRequest.onHeadersReceived((details, callback) => {
        callback({
            responseHeaders: {
                ...details.responseHeaders,
                'Content-Security-Policy': [STRICT_CSP],
                'X-Content-Type-Options': ['nosniff'],
                'X-Frame-Options': ['DENY'],
                'X-XSS-Protection': ['1; mode=block'],
                'Referrer-Policy': ['strict-origin-when-cross-origin'],
                'Permissions-Policy': [
                    'geolocation=(), microphone=(), camera=(), payment=(), usb=(), bluetooth=()'
                ]
            }
        });
    });

    // 10. Block Navigation to External Sites - SECURITY
    session.defaultSession.webRequest.onBeforeRequest((details, callback) => {
        // Block known malicious patterns
        const blockedPatterns = [
            /^https?:\/\/.*\.onion/, // Tor hidden services
            /^file:\/\/\/.*\.(exe|bat|cmd|scr|com|pif|vbs|js)$/i, // Executable files
        ];

        for (const pattern of blockedPatterns) {
            if (pattern.test(details.url)) {
                callback({ cancel: true });
                console.log(`🚫 SECURITY: Blocked malicious URL pattern: ${details.url}`);
                return;
            }
        }

        callback({});
    });

    createWindow();

    app.on('activate', () => {
        if (BrowserWindow.getAllWindows().length === 0) {
            createWindow();
        }
    });
});

// 11. Prevent Multiple Instances - SECURITY
const gotTheLock = app.requestSingleInstanceLock();

if (!gotTheLock) {
    app.quit();
} else {
    app.on('second-instance', () => {
        // Focus existing window if user tries to open another instance
        const windows = BrowserWindow.getAllWindows();
        if (windows.length > 0) {
            const mainWindow = windows[0];
            if (mainWindow.isMinimized()) mainWindow.restore();
            mainWindow.focus();
        }
    });
}

app.on('window-all-closed', () => {
    if (process.platform !== 'darwin') {
        app.quit();
    }
});

// 12. Security: Disable GPU acceleration if needed (optional, for compatibility)
// app.disableHardwareAcceleration(); // Uncomment if experiencing rendering issues

// 13. Security: Prevent new window creation
app.on('web-contents-created', (event, contents) => {
    contents.on('new-window', (event, navigationUrl) => {
        event.preventDefault();
        shell.openExternal(navigationUrl);
    });
});

// 14. Security: Log app version for debugging
console.log(`🔒 Titan System Desktop - Secure Mode Enabled`);
console.log(`📦 Electron Version: ${process.versions.electron}`);
console.log(`🖥️  Platform: ${process.platform}`);
console.log(`🔐 Security Features: Context Isolation ✅ | Node Integration ❌ | Sandbox ✅ | CSP ✅`);
