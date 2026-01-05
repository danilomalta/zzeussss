# 🔒 Configurações de Segurança - Titan System Desktop

## ✅ Regras de Blindagem Aplicadas

### 1. Context Isolation: `true` ✅
- **Status:** HABILITADO
- **Localização:** `main.js` linha 25
- **Efeito:** O renderer process está completamente isolado do Node.js. Não pode acessar `require()`, `process`, etc.

### 2. Node Integration: `false` ✅
- **Status:** DESABILITADO
- **Localização:** `main.js` linha 26
- **Efeito:** O renderer NUNCA toca no Node.js. Todas as APIs Node são bloqueadas.

### 3. Sandbox: `true` ✅
- **Status:** HABILITADO
- **Localização:** `main.js` linha 27
- **Efeito:** Sandbox estilo Chrome ativado. Máxima proteção contra exploits.

### 4. CSP Headers (Content Security Policy) ✅
- **Status:** INJETADO NA SESSÃO
- **Localização:** `main.js` linhas 58-68
- **Política Aplicada:**
  ```
  default-src 'self'
  script-src 'self' 'unsafe-inline' http://localhost:*
  style-src 'self' 'unsafe-inline' https://fonts.googleapis.com
  font-src 'self' https://fonts.gstatic.com data:
  img-src 'self' data: https:
  connect-src 'self' http://localhost:* https://* ws://localhost:* wss://*
  frame-src 'none'
  object-src 'none'
  base-uri 'self'
  form-action 'self'
  frame-ancestors 'none'
  upgrade-insecure-requests
  ```

### 5. Kiosk/Maximize ✅
- **Status:** MAXIMIZADO + MENU BAR REMOVIDO
- **Localização:** `main.js` linhas 35-36
- **Efeito:** 
  - App abre maximizado (não fullscreen, mantém controles)
  - Menu bar removido (`setMenuBarVisibility(false)`)
  - `autoHideMenuBar: true` para garantir remoção

## 🛡️ Proteções Adicionais Implementadas

### Headers de Segurança HTTP
- `X-Content-Type-Options: nosniff`
- `X-Frame-Options: DENY`
- `X-XSS-Protection: 1; mode=block`
- `Referrer-Policy: strict-origin-when-cross-origin`
- `Permissions-Policy: geolocation=(), microphone=(), camera=(), payment=(), usb=(), bluetooth=()`

### Bloqueio de Navegação
- Apenas origens permitidas podem ser acessadas
- Popups bloqueados (abrem no navegador externo)
- URLs maliciosas bloqueadas (Tor, executáveis)

### Prevenção de Múltiplas Instâncias
- Apenas uma instância do app pode rodar
- Segunda instância foca a janela existente

### DevTools
- Desabilitado em produção
- Apenas disponível em modo desenvolvimento

### Certificados SSL
- Em produção: certificados inválidos são rejeitados
- Em desenvolvimento: permite self-signed (para testes locais)

## 📋 Checklist de Segurança

- [x] Context Isolation: `true`
- [x] Node Integration: `false`
- [x] Sandbox: `true`
- [x] CSP Headers injetados
- [x] App maximizado
- [x] Menu bar removido
- [x] Web Security: `true`
- [x] Remote Module: `false`
- [x] Navigation bloqueada para origens não autorizadas
- [x] Popups bloqueados
- [x] Múltiplas instâncias prevenidas
- [x] Headers de segurança HTTP
- [x] Preload script seguro

## 🚀 Como Testar

1. **Modo Desenvolvimento:**
   ```bash
   npm start
   # ou
   electron . --dev
   ```

2. **Modo Produção:**
   ```bash
   NODE_ENV=production electron .
   ```

3. **Verificar Segurança:**
   - Tente abrir DevTools em produção (deve ser bloqueado)
   - Tente navegar para site externo (deve ser bloqueado)
   - Verifique que menu bar não aparece
   - Verifique que app abre maximizado

## ⚠️ Notas Importantes

- O `preload.js` expõe apenas APIs seguras via `contextBridge`
- Todas as comunicações IPC são validadas por whitelist
- URLs externas abrem no navegador padrão (não dentro do app)
- O app está configurado para máxima segurança por padrão

## 📚 Referências

- [Electron Security Guide](https://www.electronjs.org/docs/latest/tutorial/security)
- [CSP Reference](https://developer.mozilla.org/en-US/docs/Web/HTTP/CSP)
- [OWASP Electron Security](https://owasp.org/www-community/vulnerabilities/Electron_Security)

