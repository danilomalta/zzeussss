#!/bin/bash

echo "🌐 CONFIGURANDO TITAN WEB SOFTWARE (REACT + VITE)..."

# Garante que a pasta existe
mkdir -p TitanSystem/titan-web-software
cd TitanSystem/titan-web-software

# ==============================================================================
# 1. PACKAGE.JSON
# ==============================================================================
cat > package.json <<EOF
{
  "name": "titan-web-software",
  "private": true,
  "version": "1.0.0",
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "tsc && vite build",
    "preview": "vite preview"
  },
  "dependencies": {
    "react": "^18.2.0",
    "react-dom": "^18.2.0"
  },
  "devDependencies": {
    "@types/react": "^18.2.66",
    "@types/react-dom": "^18.2.22",
    "@vitejs/plugin-react": "^4.2.1",
    "typescript": "^5.2.2",
    "vite": "^5.2.0"
  }
}
EOF

# ==============================================================================
# 2. VITE.CONFIG.TS
# ==============================================================================
cat > vite.config.ts <<EOF
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [react()],
})
EOF

# ==============================================================================
# 3. INDEX.HTML
# ==============================================================================
cat > index.html <<EOF
<!doctype html>
<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>Titan System</title>
  </head>
  <body>
    <div id="root"></div>
    <script type="module" src="/src/main.tsx"></script>
  </body>
</html>
EOF

# ==============================================================================
# 4. SRC/MAIN.TSX
# ==============================================================================
mkdir -p src
cat > src/main.tsx <<EOF
import React from 'react'
import ReactDOM from 'react-dom/client'
import App from './App.tsx'
import './index.css'

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>,
)
EOF

# ==============================================================================
# 5. SRC/APP.TSX
# ==============================================================================
cat > src/App.tsx <<EOF
import React from 'react';

const App = () => {
  return (
    <div style={{
      backgroundColor: '#000000',
      color: '#ffffff',
      height: '100vh',
      width: '100vw',
      display: 'flex',
      flexDirection: 'column',
      justifyContent: 'center',
      alignItems: 'center',
      fontFamily: 'Inter, system-ui, Avenir, Helvetica, Arial, sans-serif',
      margin: 0,
      padding: 0,
      overflow: 'hidden'
    }}>
      <div style={{
        textAlign: 'center',
        animation: 'fadeIn 2s ease-in'
      }}>
        <h1 style={{
          fontSize: '5rem',
          fontWeight: '900',
          letterSpacing: '0.8rem',
          margin: 0,
          background: 'linear-gradient(45deg, #ffffff 30%, #666666 90%)',
          WebkitBackgroundClip: 'text',
          WebkitTextFillColor: 'transparent',
          textShadow: '0 0 30px rgba(255,255,255,0.1)'
        }}>TITAN SYSTEM</h1>
        <p style={{
          color: '#666666',
          fontSize: '1.2rem',
          marginTop: '1.5rem',
          letterSpacing: '0.2rem',
          textTransform: 'uppercase'
        }}>Enterprise Architecture</p>
      </div>
      <style>{ \`
        @keyframes fadeIn {
          from { opacity: 0; transform: translateY(20px); }
          to { opacity: 1; transform: translateY(0); }
        }
        body { margin: 0; background: #000; }
      \` }</style>
    </div>
  );
};

export default App;
EOF

# ==============================================================================
# 6. SRC/VITE-ENV.D.TS & CSS
# ==============================================================================
cat > src/vite-env.d.ts <<EOF
/// <reference types="vite/client" />
EOF

# CSS Básico para garantir reset
cat > src/index.css <<EOF
:root {
  font-family: Inter, system-ui, Avenir, Helvetica, Arial, sans-serif;
  line-height: 1.5;
  font-weight: 400;
  color-scheme: dark;
}
body {
  margin: 0;
  padding: 0;
  min-width: 320px;
  min-height: 100vh;
  background-color: #000;
}
EOF

echo "✅ TITAN WEB SOFTWARE CONFIGURADO!"
echo "---------------------------------------------------"
echo "🌐 Arquivos criados:"
echo "   - package.json"
echo "   - vite.config.ts"
echo "   - index.html"
echo "   - src/main.tsx"
echo "   - src/App.tsx (Tela Bonita)"
echo "---------------------------------------------------"
echo "👉 Para rodar: cd TitanSystem/titan-web-software && npm install && npm run dev"
