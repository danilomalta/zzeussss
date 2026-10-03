import { Link } from 'react-router-dom';
import { BrandLogo, ThemeToggle } from './BrandShell';

export default function FeaturePage({ eyebrow, title, description }: { eyebrow: string; title: string; description: string }) {
  return <div className="brand-page brand-app-page">
    <header className="brand-topbar"><BrandLogo /><ThemeToggle /></header>
    <main className="brand-app-content">
      <p className="brand-eyebrow">{eyebrow}</p><h1 className="brand-feature-title">{title}</h1>
      <section className="brand-panel brand-feature-panel"><span className="brand-feature-icon" aria-hidden="true">◇</span>
        <h2>Em integração</h2><p>{description}</p><Link className="brand-feature-link" to="/">Voltar à visão geral</Link>
      </section>
    </main>
  </div>;
}
