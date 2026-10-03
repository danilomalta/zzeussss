import { useEffect, useState } from 'react';
import { LocalAPIError, formatLocalCents } from '../../../core/local/localClient.mjs';
import type { LocalProduct } from '../../../core/local/localClient.mjs';
import { localClient, localErrorMessage, useLocalSession } from '../../../core/local/useLocalSession';
import { BrandLogo, ThemeToggle } from '../../../shared/brand/BrandShell';

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

  return <div className="brand-page brand-app-page">
    <header className="brand-topbar"><BrandLogo /><ThemeToggle /></header>
    <main className="brand-app-content">
      <div className="brand-app-heading">
        <div><p className="brand-eyebrow">INSTALAÇÃO LOCAL</p><h1>Catálogo de produtos</h1>
          <p className="brand-subtitle">Empresa {session?.tenant_id} · Loja {session?.store_id}</p></div>
        <button type="button" className="brand-secondary" onClick={() => void endSession()}>Sair da sessão</button>
      </div>
      <section className="brand-panel">
        <div className="brand-panel-top">
          <div><h2>Produtos cadastrados</h2><p>Dados reais desta instalação.</p></div>
          <button type="button" disabled={busy} onClick={() => setReload((value) => value + 1)} className="brand-small-primary">Atualizar catálogo</button>
        </div>
        {busy ? <p className="brand-state" role="status">Consultando a API local…</p> :
          error ? <p className="brand-message error" role="alert">{error}</p> :
            items.length === 0 ? <div className="brand-empty" role="status"><span aria-hidden="true">◇</span><h3>Nenhum produto nesta página</h3><p>O catálogo mostrará os itens assim que forem cadastrados nesta instalação.</p></div> :
              <div className="brand-table-scroll"><table className="brand-table"><thead><tr><th>SKU</th><th>Produto</th><th>Unidade</th><th>Preço</th></tr></thead>
                <tbody>{items.map((item) => <tr key={item.id}><td>{item.sku}</td><td>{item.name}</td><td>{item.unit}</td><td>{formatLocalCents(item.price_cents)}</td></tr>)}</tbody></table></div>}
        <nav className="brand-pagination" aria-label="Paginação do catálogo">
          <button type="button" className="brand-secondary" disabled={busy || offset === 0} onClick={() => setOffset((value) => Math.max(0, value - 50))}>Anterior</button>
          <span>Página {offset / 50 + 1}</span>
          <button type="button" className="brand-secondary" disabled={busy || !!error || items.length < 50} onClick={() => setOffset((value) => value + 50)}>Próxima</button>
        </nav>
      </section>
    </main>
  </div>;
}
