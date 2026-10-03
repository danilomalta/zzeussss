import { useState } from 'react';
import { NavLink, Outlet, useLocation } from 'react-router-dom';
import { Navigate } from 'react-router-dom';
import { useLocalAccess } from '../../core/local/LocalAccess';
import { allowedAreas, areaForRoute } from '../../core/local/accessModel.mjs';
import { BrandLogo, ThemeToggle } from './BrandShell';
import { useLocalSession } from '../../core/local/useLocalSession';
import './workspace.css';

const modules = [
 { id: 'home', name: 'Minha área', icon: '◇', path: '/local/home' },
 { id: 'pos', name: 'PDV', icon: '▦', path: '/local/pos' },
 { id: 'catalog', name: 'Catálogo de produtos', icon: '▤', path: '/local/catalog' },
 { id: 'stock', name: 'Estoque e locais', icon: '▥', path: '/local/stock' },
 { id: 'cash', name: 'Fechamento de caixa', icon: '◈', path: '/local/pos?panel=cash' },
 { id: 'sos', name: 'SOS', icon: '⊕' },
 { id: 'production', name: 'Produção e receitas', icon: '◇' },
 { id: 'orders', name: 'Pedidos e fornecedores', icon: '⇄', path: '/local/orders' },
 { id: 'fleet', name: 'Frota e rotas', icon: '↗' },
 { id: 'staff', name: 'RH e funcionários', icon: '♧', path: '/local/staff' },
 { id: 'point', name: 'Meu ponto', icon: '◷' },
 { id: 'accounting', name: 'Contabilidade', icon: '▧' },
 { id: 'prices', name: 'Comparador de preços', icon: '≍' },
 { id: 'payments', name: 'Maquininha, Pix e delivery', icon: '▭' },
 { id: 'plans', name: 'Assinatura e faturas', icon: '⚙', path: '/local/subscription' },
];
function TileIcon({id}:{id:string}) {
 const paths:Record<string,string>={
  home:'M3 10l9-7 9 7v11h-6v-7H9v7H3z',
  point:'M12 2a10 10 0 1 0 0 20 10 10 0 0 0 0-20 M12 6v6l4 2',
  pos:'M3 3h7v7H3z M14 3h7v7h-7z M3 14h7v7H3z M14 14h7v7h-7z',
  catalog:'M5 3h14v18H5z M8 7h8 M8 11h8 M8 15h5',
  stock:'M3 8l9-5 9 5v10l-9 4-9-4z M3 8l9 5 9-5 M12 13v9',
  cash:'M3 6h18v14H3z M3 10h18 M7 15h4 M17 15h1',
  sos:'M12 3l10 18H2z M12 9v5 M12 17h.01',
  production:'M3 21V9l6 3V6l6 6V3h5v18z',
  orders:'M3 7h16 M15 3l4 4-4 4 M21 17H5 M9 13l-4 4 4 4',
  fleet:'M2 6h12v12H2z M14 10h4l4 4v4h-8 M4 18v3h4v-3 M16 18v3h4v-3',
  staff:'M8 10a4 4 0 1 0 0-8 4 4 0 0 0 0 8 M2 22v-5c0-6 12-6 12 0v5 M16 5h6 M19 2v6',
  accounting:'M5 2h14v20H5z M8 6h8 M8 10h2 M14 10h2 M8 14h2 M14 14h2 M8 18h2 M14 18h2',
  prices:'M3 16l6-6 4 4 8-10 M15 4h6v6',
  payments:'M3 5h18v14H3z M3 10h18 M7 15h4',
  plans:'M3 6h18 M3 12h18 M3 18h18 M7 3v6 M17 9v6 M10 15v6',
 };
 return <svg viewBox="0 0 24 24" width="24" height="24" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true"><path d={paths[id]}/></svg>;
}
function readShortcuts(): string[] {
 try { const ids = JSON.parse(localStorage.getItem('titan-shortcuts') || 'null');
  if (Array.isArray(ids) && ids.length >= 1 && ids.length <= 3 && new Set(ids).size === ids.length && ids.every(id => modules.some(m => m.id === id))) return ids;
 } catch { /* Preferences stay in memory if storage is unavailable. */ }
 return ['pos', 'catalog', 'cash'];
}
export default function LocalWorkspace() {
 const location = useLocation();
 const {capabilities} = useLocalAccess();
 const allowed = allowedAreas(capabilities);
 const visible = modules.filter(m=>allowed.includes(m.id));
 const [expanded, setExpanded] = useState(false);
 const [shortcuts, setShortcuts] = useState(readShortcuts);
 const [notice, setNotice] = useState('');
 const endSession = useLocalSession(s => s.endSession);
 function choose(slot: number, id: string) {
  const available = [...shortcuts.filter(id=>visible.some(m=>m.id===id)),...visible.map(m=>m.id).filter(id=>!shortcuts.includes(id))].slice(0,3);
  const next = [...available]; const existing = next.indexOf(id);
  if (existing !== -1) next[existing] = next[slot]; next[slot] = id;
  setShortcuts(next); try { localStorage.setItem('titan-shortcuts', JSON.stringify(next)); } catch { setNotice('Atalhos alterados nesta sessão; o navegador não permitiu salvar a preferência.'); }
 }
 const effectiveShortcuts = [...shortcuts.filter(id=>visible.some(m=>m.id===id)),...visible.map(m=>m.id).filter(id=>!shortcuts.includes(id))].slice(0,3);
 if (!allowed.includes(areaForRoute(location.pathname))) return <Navigate to="/local/home" replace />;
 return <div className={`brand-page workspace ${expanded ? 'workspace-expanded' : ''}`}>
  <aside className="workspace-rail" aria-label="Navegação dos módulos">
   <button className="workspace-menu" aria-expanded={expanded} aria-label={expanded ? 'Recolher menu' : 'Abrir menu de módulos'} onClick={() => setExpanded(!expanded)}>☰</button>
   {expanded && <><BrandLogo /><p className="brand-state">Módulos da empresa</p></>}
   <nav>{(expanded ? visible : effectiveShortcuts.map(id => visible.find(m => m.id === id)!)).map(m => m.path ?
    <NavLink key={m.id} to={m.path} title={m.name} aria-label={m.name} className={() => `workspace-tile ${(m.id==='pos' && location.pathname==='/local/pos' && !location.search.includes('panel=cash')) || (m.id==='cash' && location.pathname==='/local/pos' && location.search.includes('panel=cash')) || (m.id==='catalog' && location.pathname==='/local/catalog') || (m.id==='stock' && location.pathname==='/local/stock') || (m.id==='orders' && location.pathname==='/local/orders') ? 'selected' : ''}`} onClick={() => setExpanded(false)}>
     <span className="workspace-squircle" aria-hidden="true"><TileIcon id={m.id}/></span><span className="workspace-label">{m.name}</span>
    </NavLink> : <button key={m.id} className="workspace-tile" title={`${m.name} — em preparação`} aria-label={m.name} onClick={() => setNotice(`${m.name}: módulo planejado. Esta etapa disponibiliza a navegação; a função ainda não está integrada.`)}><span className="workspace-squircle" aria-hidden="true"><TileIcon id={m.id}/></span><span className="workspace-label">{m.name}<small>Em preparação</small></span></button>)}</nav>
   {expanded && <section className="workspace-shortcuts"><h2>Seus 3 atalhos rápidos</h2>{effectiveShortcuts.map((id, slot) => <label key={slot}>Atalho {slot+1}<select value={id} onChange={e => choose(slot, e.target.value)}>{visible.map(m => <option key={m.id} value={m.id}>{m.name}</option>)}</select></label>)}<button className="brand-secondary" onClick={() => void endSession()}>Sair da sessão</button></section>}
  </aside>
  <section className="workspace-body"><header className="workspace-top"><BrandLogo /><ThemeToggle /></header>
   {notice && <div className="workspace-notice" role="status">{notice}<button className="brand-secondary" onClick={() => setNotice('')}>Fechar aviso</button></div>}
   <div className="workspace-scroll"><Outlet /></div>
  </section>
 </div>;
}
