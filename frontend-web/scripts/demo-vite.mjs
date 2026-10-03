import { createServer } from 'vite';

const [webPort, apiPort] = process.argv.slice(2).map(Number);
if (![webPort, apiPort].every((port) => Number.isInteger(port) && port > 0 && port <= 65535) || webPort === apiPort) {
  throw new Error('Portas de demonstracao invalidas.');
}
const server = await createServer({ server: {
  host: '127.0.0.1', port: webPort, strictPort: true,
  proxy: { '/local/v1': { target: `http://127.0.0.1:${apiPort}`, changeOrigin: false } },
} });
await server.listen();
for (const signal of ['SIGINT', 'SIGTERM']) process.once(signal, async () => { await server.close(); process.exit(0); });
console.log(`Frontend de teste: http://127.0.0.1:${webPort}/local/login`);
