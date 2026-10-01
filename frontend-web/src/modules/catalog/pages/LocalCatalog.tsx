import { useEffect, useState } from 'react';
import { LocalAPIError, formatLocalCents } from '../../../core/local/localClient.mjs';
import type { LocalProduct } from '../../../core/local/localClient.mjs';
import { localClient, localErrorMessage, useLocalSession } from '../../../core/local/useLocalSession';

export default function LocalCatalog() {
  const token = useLocalSession((state) => state.token);
  const session = useLocalSession((state) => state.session);
  const invalidate = useLocalSession((state) => state.invalidate);
  const endSession = useLocalSession((state) => state.endSession);
  const [items, setItems] = useState<LocalProduct[]>([]);
  const [offset, setOffset] = useState(0);
  const [reload, setReload] = useState(0);
  const [busy, setBusy] = useState(true);
  const [error, setError] = useState('');
  useEffect(() => {
    if (!token) return;
    let active = true; setBusy(true); setError(''); setItems([]);
    localClient.products(token, offset).then((result) => { if (active) setItems(result); })
      .catch((failure: unknown) => {
        if (!active) return;
        if (failure instanceof LocalAPIError && failure.status === 401) invalidate();
        else setError(localErrorMessage(failure));
      }).finally(() => { if (active) setBusy(false); });
    return () => { active = false; };
  }, [token, offset, reload, invalidate]);
  return <main className="h-screen flex flex-col bg-slate-900 text-slate-100 p-4 gap-4">
    <header className="flex flex-wrap items-center justify-between gap-3">
      <div><h1 className="text-2xl font-bold">Catálogo da instalação local</h1><p className="text-sm text-slate-400 break-all">Empresa: {session?.tenant_id} · Loja: {session?.store_id}</p></div>
      <button onClick={() => void endSession()} className="rounded border border-slate-600 px-4 py-2">Sair da sessão local</button>
    </header>
    <p className="text-sm text-slate-300">Consulta de produtos reais. Cadastro, estoque, caixa e conclusão de venda ainda precisam ser ligados a esta interface.</p>
    <div className="flex gap-3 items-center"><button disabled={busy} onClick={() => setReload((value) => value + 1)} className="rounded bg-indigo-600 px-4 py-2 disabled:opacity-50">Atualizar catálogo</button><span className="text-sm">Página {offset / 50 + 1}</span></div>
    {busy ? <p role="status">Consultando a API local…</p> : error ? <p role="alert" className="text-red-300">{error}</p> : items.length === 0 ? <p role="status">Nenhum produto nesta página.</p> :
      <section aria-label="Produtos" tabIndex={0} className="min-h-0 flex-1 overflow-auto rounded border border-slate-700">
        <table className="w-full text-left"><thead className="bg-slate-800"><tr><th className="p-3">SKU</th><th className="p-3">Produto</th><th className="p-3">Unidade</th><th className="p-3">Preço</th></tr></thead>
          <tbody>{items.map((item) => <tr key={item.id} className="border-t border-slate-700"><td className="p-3">{item.sku}</td><td className="p-3">{item.name}</td><td className="p-3">{item.unit}</td><td className="p-3 whitespace-nowrap">{formatLocalCents(item.price_cents)}</td></tr>)}</tbody>
        </table>
      </section>}
    <nav aria-label="Paginação do catálogo" className="flex gap-3">
      <button disabled={busy || offset === 0} onClick={() => setOffset((value) => Math.max(0, value - 50))} className="rounded border border-slate-600 px-4 py-2 disabled:opacity-50">Anterior</button>
      <button disabled={busy || !!error || items.length < 50} onClick={() => setOffset((value) => value + 50)} className="rounded border border-slate-600 px-4 py-2 disabled:opacity-50">Próxima</button>
    </nav>
  </main>;
}
