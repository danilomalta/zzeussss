# O Fosso Intercomunicacional do App
**Linguagem**: NodeJS/TypeScript
**Papel**: O Front de Caixa (React) NUNCA toca no SQLite local diretamente, ele repassa pelo ContextBridge IPC seguro para evitar roubo de token.
