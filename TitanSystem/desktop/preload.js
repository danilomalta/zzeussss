/**
 * preload.js
 * Secure Preload Script for Titan System Desktop
 * 
 * This script runs in an isolated context before the renderer loads.
 * It provides a safe bridge between the renderer and the main process.
 * 
 * SECURITY NOTES:
 * - contextIsolation: true (Renderer cannot access Node.js directly)
 * - nodeIntegration: false (Renderer cannot require() modules)
 * - sandbox: true (Chrome-style sandbox)
 */

const { contextBridge, ipcRenderer } = require('electron');

// --- SECURE API EXPOSURE ---
// Only expose safe, whitelisted APIs to the renderer

contextBridge.exposeInMainWorld('electronAPI', {
    // Safe IPC communication
    sendMessage: (channel, data) => {
        // Whitelist of allowed channels
        const validChannels = ['toMain', 'app-version', 'platform-info'];
        if (validChannels.includes(channel)) {
            ipcRenderer.send(channel, data);
        } else {
            console.warn(`🚫 Blocked IPC channel: ${channel}`);
        }
    },

    onReceiveMessage: (channel, func) => {
        // Whitelist of allowed channels
        const validChannels = ['fromMain', 'navigation-blocked', 'app-update'];
        if (validChannels.includes(channel)) {
            // Remove all listeners first to prevent memory leaks
            ipcRenderer.removeAllListeners(channel);
            ipcRenderer.on(channel, (event, ...args) => func(...args));
        } else {
            console.warn(`🚫 Blocked IPC channel: ${channel}`);
        }
    },

    // Safe platform information (read-only)
    getPlatform: () => {
        return process.platform;
    },

    // Safe app version (read-only)
    getVersion: () => {
        return process.versions.electron;
    },

    // Remove listener helper (for cleanup)
    removeListener: (channel) => {
        const validChannels = ['fromMain', 'navigation-blocked', 'app-update'];
        if (validChannels.includes(channel)) {
            ipcRenderer.removeAllListeners(channel);
        }
    }
});

// --- SECURITY: Prevent accidental exposure ---
// Delete Node.js globals to prevent access (extra safety)
delete window.require;
delete window.exports;
delete window.module;

// Log that preload script loaded successfully
console.log('🔒 Titan System Preload Script - Secure Mode Active');
