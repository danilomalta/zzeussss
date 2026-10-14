import { Link, useLocation } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { useLocalAccess } from '../../core/local/LocalAccess';
import { sourceAllowed } from '../../core/sheets70/model.mjs';
import { dictionaries70 } from '../../modules/tenant/pages/Sheets70';
export default function OpenInSheets70() {
  const location = useLocation(),
    { capabilities } = useLocalAccess(),
    { i18n } = useTranslation();
  const source: Record<string, string> = {
    '/local/catalog': 'catalog',
    '/local/stock': 'stock',
    '/local/pos': 'sales',
    '/local/cash': 'sales',
    '/local/orders': 'purchases',
    '/local/staff': 'staff',
    '/local/hr': 'staff',
    '/local/accounting': 'finance',
  };
  const selected = source[location.pathname];
  if (!selected || !sourceAllowed(selected, capabilities)) return null;
  const parameters = new URLSearchParams(location.search),
    query = new URLSearchParams({ source: selected });
  for (const key of ['q', 'from', 'to'])
    if (parameters.has(key)) query.set(key, parameters.get(key)!.slice(0, 240));
  const words = dictionaries70[i18n.resolvedLanguage || i18n.language] || dictionaries70['pt-BR'];
  return (
    <Link className="brand-secondary" to={'/local/sheets?' + query}>
      {words.openSheets}
    </Link>
  );
}
