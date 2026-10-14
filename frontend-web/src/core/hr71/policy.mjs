import { allowedAreas } from '../local/accessModel.mjs';
export const areas = [
  { id: 'attendance', icon: '◷', major: true },
  { id: 'leave', icon: '☀', major: true },
  { id: 'people', icon: '♧', major: true, route: '/local/staff' },
  { id: 'schedules', icon: '▦', major: true },
  { id: 'recruitment', icon: '◇' },
  { id: 'lifecycle', icon: '✓' },
  { id: 'payroll', icon: '▤', sensitive: true },
  { id: 'benefits', icon: '♡', sensitive: true },
  { id: 'documents', icon: '▧' },
  { id: 'performance', icon: '↗' },
  { id: 'reports', icon: '▥' },
  { id: 'portal', icon: '✉' },
  { id: 'security', icon: '⌑' },
];
export function canView(capabilities) {
  return allowedAreas(capabilities).includes('hr');
}
export function sourceAccess(name, capabilities) {
  if (!capabilities?.license?.modules.includes('staff')) return 'not_licensed';
  if (!canView(capabilities)) return 'forbidden';
  if (['payroll', 'finance'].includes(name) && capabilities.role !== 'owner') return 'forbidden';
  if (
    name === 'finance' &&
    !capabilities.license.modules.includes('finance') &&
    !capabilities.license.modules.includes('accounting')
  )
    return 'not_licensed';
  return 'ready';
}
export function visibleAreas(capabilities) {
  if (!canView(capabilities)) return [];
  return areas.filter((area) => !area.sensitive || capabilities.role === 'owner');
}
export function areaState(area, capabilities) {
  const state = sourceAccess(area.id, capabilities);
  return state !== 'ready' ? state : area.route ? 'ready' : 'unavailable';
}
export function initials(name) {
  return name
    .trim()
    .split(/\s+/)
    .slice(0, 2)
    .map((part) => Array.from(part)[0] || '')
    .join('')
    .toLocaleUpperCase();
}
export function filterPeople(
  people,
  { query = '', department = '', jobTitle = '', status = '', ids, sort = 'name' }
) {
  const search = query.trim().toLocaleLowerCase();
  return people
    .filter(
      (person) =>
        (!search ||
          [person.name, person.jobTitle, person.department].some((v) =>
            v?.toLocaleLowerCase().includes(search)
          )) &&
        (!department || person.department === department) &&
        (!jobTitle || person.jobTitle === jobTitle) &&
        (!status || person.status === status) &&
        (!ids || ids.includes(person.id))
    )
    .sort((a, b) => String(a[sort] || '').localeCompare(String(b[sort] || '')));
}
export function csv(rows) {
  return (
    '\uFEFF' +
    rows
      .map((row) =>
        row
          .map((v) => {
            let value = String(v ?? '');
            if (/^[=+@-]/.test(value)) value = "'" + value;
            return '"' + value.replaceAll('"', '""') + '"';
          })
          .join(',')
      )
      .join('\r\n')
  );
}
export function currency(cents, locale) {
  if (!Number.isSafeInteger(cents) || cents < 0) return '—';
  return new Intl.NumberFormat(locale, { style: 'currency', currency: 'BRL' }).format(cents / 100);
}
