import { Link } from 'react-router-dom';
import { BrandShell } from '../../../shared/brand/BrandShell';

export default function PointOfSale() {
  return <BrandShell footer={false}><section className="brand-panel brand-feature-panel"><p className="brand-eyebrow">OPERAÇÕES</p><h1>Frente de caixa</h1><p>O PDV usa a sessão da instalação para conferir operador, aparelho e turno de caixa.</p><Link className="brand-feature-link" to="/local/pos">Entrar no PDV local</Link></section></BrandShell>;
}
