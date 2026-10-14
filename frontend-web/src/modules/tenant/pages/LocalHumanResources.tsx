import { useEffect, useRef, useState } from 'react';
import { Link, Navigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { useLocalAccess } from '../../../core/local/LocalAccess';
import { useLocalSession } from '../../../core/local/useLocalSession';
import { staffSheetLink } from '../../../core/hr71/sheets';
import { useDashboard } from '../../../core/hr71/useDashboard';
import {
  canView,
  visibleAreas,
  areaState,
  initials,
  filterPeople,
  csv,
  currency,
} from '../../../core/hr71/policy.mjs';
import type { Area } from '../../../core/hr71/policy.mjs';
import type { Metric, Person, SourceState } from '../../../core/hr71/contracts';
import { wordsFor } from '../../../core/hr71/words';
import './humanResources71.css';
const pageSize = 25;
export default function LocalHumanResources() {
  const { i18n } = useTranslation(),
    locale = i18n.resolvedLanguage || i18n.language,
    w = wordsFor(locale),
    tr = (key: string) => (w as Record<string, string>)[key] || key;
  const { capabilities } = useLocalAccess(),
    token = useLocalSession((s) => s.token),
    session = useLocalSession((s) => s.session),
    tokenRef = useRef(token);
  tokenRef.current = token;
  const [minutes, setMinutes] = useState(5),
    [query, setQuery] = useState(''),
    [department, setDepartment] = useState(''),
    [jobTitle, setJobTitle] = useState(''),
    [status, setStatus] = useState(''),
    [sort, setSort] = useState('name'),
    [page, setPage] = useState(1),
    [selected, setSelected] = useState<string[]>([]),
    [metric, setMetric] = useState(''),
    [person, setPerson] = useState<Person>(),
    [help, setHelp] = useState(false),
    [busy, setBusy] = useState(false),
    [error, setError] = useState(''),
    [clock, setClock] = useState(Date.now());
  const alive = useRef(true),
    access = useRef({ allowed: false });
  useEffect(() => {
    alive.current = true;
    return () => {
      alive.current = false;
    };
  }, []);
  const { sources, refresh, mock } = useDashboard(minutes),
    search = useRef<HTMLInputElement>(null),
    dialog = useRef<HTMLDialogElement>(null),
    focus = useRef<HTMLElement | null>(null);
  const allowed = Boolean(token && session) && canView(capabilities),
    sheetLink = staffSheetLink(capabilities, mock);
  access.current = { allowed };
  useEffect(() => {
    const shortcut = (event: KeyboardEvent) => {
      const input =
        event.target instanceof HTMLElement &&
        (event.target.matches('input,textarea,select') || event.target.isContentEditable);
      if (
        ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'k') ||
        (event.key === '/' && !input)
      ) {
        event.preventDefault();
        search.current?.focus();
        search.current?.select();
      }
    };
    window.addEventListener('keydown', shortcut);
    const timer = setInterval(() => setClock(Date.now()), 30000);
    return () => {
      window.removeEventListener('keydown', shortcut);
      clearInterval(timer);
    };
  }, []);
  useEffect(() => {
    setPage(1);
    setSelected([]);
  }, [query, department, jobTitle, status, metric]);
  useEffect(() => {
    if (sources.staff.data)
      setSelected((old) => old.filter((id) => sources.staff.data!.some((p) => p.id === id)));
    else if (sources.staff.state === 'forbidden') setSelected([]);
  }, [sources.staff.data, sources.staff.state]);
  useEffect(() => {
    setDepartment('');
    setJobTitle('');
  }, [locale]);
  useEffect(() => {
    setSelected([]);
    setPerson(undefined);
    setQuery('');
    setMetric('');
    setError('');
  }, [token, session?.identity_id, session?.tenant_id]);
  useEffect(() => {
    if (person || help) {
      focus.current = document.activeElement as HTMLElement;
      dialog.current?.showModal();
    }
    return () => {
      dialog.current?.close();
      focus.current?.focus();
    };
  }, [person, help]);
  const people = (sources.staff.data || []).map((p) => ({
    ...p,
    jobTitle: p.jobTitle ? (mock ? tr(p.jobTitle) : p.jobTitle) : undefined,
    department: p.department ? (mock ? tr(p.department) : p.department) : undefined,
  }));
  const metrics: { key: string; data?: Metric; state: SourceState }[] = [
    { key: 'present', data: sources.attendance.data?.present, state: sources.attendance.state },
    { key: 'absent', data: sources.attendance.data?.absent, state: sources.attendance.state },
    { key: 'away', data: sources.leave.data?.away, state: sources.leave.state },
    { key: 'trial', data: sources.documents.data?.trial, state: sources.documents.state },
    { key: 'birthdays', data: sources.documents.data?.birthdays, state: sources.documents.state },
    { key: 'pending', data: sources.leave.data?.pending, state: sources.leave.state },
  ];
  const selectedMetric = metrics.find((value) => value.key === metric),
    rows = filterPeople(people, {
      query,
      department,
      jobTitle,
      status,
      sort,
      ids: selectedMetric?.data?.people,
    }),
    pages = Math.max(1, Math.ceil(rows.length / pageSize)),
    currentPage = Math.min(page, pages),
    visible = rows.slice((currentPage - 1) * pageSize, currentPage * pageSize),
    areas = visibleAreas(capabilities).filter(
      (area) =>
        !query ||
        (tr('area.' + area.id) + ' ' + tr('description.' + area.id))
          .toLocaleLowerCase()
          .includes(query.toLocaleLowerCase())
    ),
    empty = sources.staff.state === 'empty';
  const number = (value: number) => new Intl.NumberFormat(locale).format(value),
    date = (value?: string) => {
      if (!value) return '—';
      const parsed = new Date(value + 'T12:00:00');
      return Number.isNaN(parsed.valueOf())
        ? '—'
        : new Intl.DateTimeFormat(locale, { dateStyle: 'short' }).format(parsed);
    };
  const stateLabel = (state: SourceState) =>
    state === 'ready'
      ? w.active
      : state === 'not_licensed'
        ? w.notLicensed
        : state === 'forbidden'
          ? w.forbidden
          : state === 'loading'
            ? w.loading
            : state === 'error'
              ? w.failure
              : state === 'empty'
                ? w.emptySource
                : w.coming;
  const stateHelp = (state: SourceState) =>
    ['ready', 'empty'].includes(state)
      ? ''
      : state === 'forbidden'
        ? w.forbidden
        : state === 'not_licensed'
          ? w.notLicensed
          : state === 'error'
            ? w.failure
            : w.unavailable;
  const last = sources.staff.at,
    ago = last ? Math.max(0, Math.floor((clock - last) / 60000)) : undefined;
  function saveFile(blob: Blob, extension: string) {
    const url = URL.createObjectURL(blob),
      a = document.createElement('a');
    a.href = url;
    a.download = 'Titan-RH' + (mock ? '-DEMO' : '') + extension;
    a.click();
    setTimeout(() => URL.revokeObjectURL(url), 1000);
  }
  async function exportPeople(kind: 'csv' | 'xlsx', template = false) {
    if (!allowed || busy) return;
    setBusy(true);
    setError('');
    const identity = token;
    try {
      const headings = [w.name, w.profile, w.job, w.department, w.status, w.expiry];
      const exported = template
        ? []
        : selected.length
          ? people.filter((p) => selected.includes(p.id))
          : rows;
      const matrix = [
        headings,
        ...exported.map((p) => [
          p.name,
          tr('role.' + p.role),
          p.jobTitle || '',
          p.department || '',
          tr(p.status),
          date(p.nextExpiry) === '—' ? '' : date(p.nextExpiry),
        ]),
      ];
      if (kind === 'csv') {
        if (
          alive.current &&
          access.current.allowed &&
          useLocalSession.getState().token === identity &&
          tokenRef.current === identity
        )
          saveFile(
            new Blob([csv(matrix)], { type: 'text/csv;charset=utf-8' }),
            template ? '-modelo.csv' : '.csv'
          );
      } else {
        const { Workbook } = await import('exceljs');
        const book = new Workbook(),
          sheet = book.addWorksheet(w.csvName.slice(0, 31));
        sheet.addRows(matrix);
        sheet.getRow(1).font = { bold: true };
        sheet.columns.forEach((column) => {
          column.width = 24;
        });
        const bytes = await book.xlsx.writeBuffer();
        if (
          alive.current &&
          access.current.allowed &&
          useLocalSession.getState().token === identity &&
          tokenRef.current === identity
        )
          saveFile(
            new Blob([new Uint8Array(bytes).buffer], {
              type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
            }),
            template ? '-modelo.xlsx' : '.xlsx'
          );
      }
    } catch {
      if (
        alive.current &&
        access.current.allowed &&
        useLocalSession.getState().token === identity &&
        tokenRef.current === identity
      )
        setError('exportFailed');
    } finally {
      if (
        alive.current &&
        access.current.allowed &&
        useLocalSession.getState().token === identity &&
        tokenRef.current === identity
      )
        setBusy(false);
    }
  }
  function areaCard(area: Area) {
    const state = areaState(area, capabilities),
      title = tr('area.' + area.id),
      key =
        area.id === 'attendance'
          ? sources.attendance.data?.absent.count
          : area.id === 'leave'
            ? sources.leave.data?.pending.count
            : area.id === 'people'
              ? sources.staff.data?.length
              : area.id === 'schedules'
                ? sources.schedules.data?.pending
                : undefined;
    const contents = (
      <>
        <span className="hr71-icon" aria-hidden="true">
          {area.icon}
        </span>
        <div>
          <h3>{title}</h3>
          <p title={tr('description.' + area.id)}>{tr('description.' + area.id)}</p>
        </div>
        {area.major && (
          <strong className="hr71-key">
            {key === undefined ? '—' : number(key)}
            <small>
              {area.id === 'attendance'
                ? w.absent
                : area.id === 'people'
                  ? w.peopleCount
                  : w.pending}
            </small>
          </strong>
        )}
        <span className={'hr71-badge ' + (state === 'ready' ? 'success' : 'neutral')}>
          <span aria-hidden="true">{state === 'ready' ? '✓' : '◷'}</span>
          {stateLabel(state)}
        </span>
      </>
    );
    return state === 'ready' && area.route ? (
      <Link key={area.id} className="hr71-card hr71-area" to={area.route} aria-label={title}>
        {contents}
      </Link>
    ) : (
      <article
        key={area.id}
        className="hr71-card hr71-area hr71-disabled"
        aria-disabled="true"
        title={stateHelp(state)}
      >
        {contents}
        {state === 'not_licensed' && capabilities?.role === 'owner' && (
          <Link to="/local/subscription">{w.plans}</Link>
        )}
      </article>
    );
  }
  if (!allowed) return <Navigate to="/local/home" replace />;
  return (
    <main className="hr71-page">
      <header className="hr71-heading">
        <div>
          <h1>{w.title}</h1>
          <p>{w.subtitle}</p>
        </div>
        <div className="hr71-actions">
          <Link className="hr71-primary" to="/local/staff#staff-form">
            + {w.add}
          </Link>
          <button disabled title={w.importHelp}>
            {w.import} · {w.coming}
          </button>
        </div>
      </header>
      {mock && (
        <p className="hr71-demo" role="status">
          ◇ {w.demonstration}
        </p>
      )}
      <section className="hr71-toolbar">
        <label className="hr71-search">
          <span className="hr71-sr">{w.search}</span>
          <input
            ref={search}
            type="search"
            placeholder={w.searchHint}
            aria-label={w.search}
            value={query}
            maxLength={200}
            onChange={(e) => setQuery(e.target.value)}
          />
          <kbd>Ctrl K</kbd>
        </label>
        <div className="hr71-refresh">
          <span role="status">
            {last
              ? w.updated + ' · ' + (ago === 0 ? w.justNow : number(ago || 0) + ' ' + w.minutes)
              : '—'}
          </span>
          <label>
            {w.interval}
            <select value={minutes} onChange={(e) => setMinutes(Number(e.target.value))}>
              {[1, 2, 5, 10].map((value) => (
                <option key={value} value={value}>
                  {number(value)} {w.minutes}
                </option>
              ))}
            </select>
          </label>
          <button onClick={refresh} aria-label={w.refresh}>
            ↻ {w.refresh}
          </button>
        </div>
        <details className="hr71-about">
          <summary>{w.about}</summary>
          <p>{w.aboutBody}</p>
        </details>
      </section>
      {!empty && (
        <section className="hr71-metrics" aria-label={w.title}>
          {metrics.map((item) => (
            <button
              key={item.key}
              disabled={!item.data}
              className={'hr71-card ' + (metric === item.key ? 'hr71-selected' : '')}
              aria-label={
                tr(item.key) +
                ': ' +
                (item.data ? number(item.data.count) : '—') +
                ' ' +
                stateHelp(item.state)
              }
              title={stateHelp(item.state)}
              onClick={() => setMetric(item.key)}
            >
              <span>{tr(item.key)}</span>
              <strong>{item.data ? number(item.data.count) : '—'}</strong>
              {item.data?.delta !== undefined && (
                <small>
                  {w.change}: {item.data.delta > 0 ? '+' : ''}
                  {number(item.data.delta)}
                </small>
              )}
            </button>
          ))}
        </section>
      )}
      {error && (
        <p role="alert" className="hr71-error">
          {tr(error)}
        </p>
      )}
      {empty ? (
        <section className="hr71-card hr71-empty">
          <span className="hr71-empty-icon" aria-hidden="true">
            ♧
          </span>
          <h2>{w.emptyTitle}</h2>
          <p>{w.emptyText}</p>
          <div className="hr71-actions">
            <Link className="hr71-primary" to="/local/staff#staff-form">
              {w.add}
            </Link>
            <button disabled title={w.importHelp}>
              {w.import} · {w.coming}
            </button>
            <button onClick={() => setHelp(true)}>{w.help}</button>
          </div>
          <button disabled={busy} onClick={() => void exportPeople('xlsx', true)}>
            {w.template} · XLSX
          </button>
        </section>
      ) : (
        <div className="hr71-work">
          <section className="hr71-card hr71-people">
            <header>
              <h2>
                {w.people} {sources.staff.data && <small>({number(people.length)})</small>}
              </h2>
              {metric && (
                <button onClick={() => setMetric('')}>
                  {w.overview} › {tr(metric)} ×
                </button>
              )}
            </header>
            <div className="hr71-filters">
              <label>
                {w.department}
                <select
                  value={department}
                  disabled={!people.some((p) => p.department)}
                  title={people.some((p) => p.department) ? undefined : w.notProvided}
                  onChange={(e) => setDepartment(e.target.value)}
                >
                  <option value="">{w.all}</option>
                  {[...new Set(people.map((p) => p.department).filter(Boolean))].map((value) => (
                    <option key={value}>{value}</option>
                  ))}
                </select>
              </label>
              <label>
                {w.job}
                <select
                  value={jobTitle}
                  disabled={!people.some((p) => p.jobTitle)}
                  title={people.some((p) => p.jobTitle) ? undefined : w.notProvided}
                  onChange={(e) => setJobTitle(e.target.value)}
                >
                  <option value="">{w.all}</option>
                  {[...new Set(people.map((p) => p.jobTitle).filter(Boolean))].map((value) => (
                    <option key={value}>{value}</option>
                  ))}
                </select>
              </label>
              <label>
                {w.status}
                <select value={status} onChange={(e) => setStatus(e.target.value)}>
                  <option value="">{w.all}</option>
                  {[...new Set(people.map((p) => p.status))].map((value) => (
                    <option key={value} value={value}>
                      {tr(value)}
                    </option>
                  ))}
                </select>
              </label>
              <label>
                {w.sort}
                <select value={sort} onChange={(e) => setSort(e.target.value)}>
                  <option value="name">{w.ascending}</option>
                  <option value="department">{w.department}</option>
                  <option value="jobTitle">{w.job}</option>
                </select>
              </label>
              <button
                onClick={() => {
                  setQuery('');
                  setDepartment('');
                  setJobTitle('');
                  setStatus('');
                  setMetric('');
                }}
              >
                {w.clear}
              </button>
            </div>
            <div className="hr71-export">
              <span>
                {w.selected}: {number(selected.length)}
              </span>
              <button disabled={busy || !rows.length} onClick={() => void exportPeople('csv')}>
                {w.exportCSV}
              </button>
              <button disabled={busy || !rows.length} onClick={() => void exportPeople('xlsx')}>
                {w.exportXLSX}
              </button>
              {sheetLink.available ? (
                <Link title={w.sheetHelp} to={sheetLink.href}>
                  {w.sheets}
                </Link>
              ) : (
                <button disabled title={mock ? w.demonstration : w.sheetComing}>
                  {w.sheets} · {w.coming}
                </button>
              )}
            </div>
            {sources.staff.state === 'loading' ? (
              <p role="status">{w.loading}</p>
            ) : sources.staff.state === 'error' || sources.staff.state === 'forbidden' ? (
              <p role="alert">
                {stateLabel(sources.staff.state)} <button onClick={refresh}>{w.retry}</button>
              </p>
            ) : !rows.length ? (
              <p className="hr71-no-results">
                {w.noResults}
                {query && <> · {w.noDocuments}</>}
              </p>
            ) : (
              <table className="hr71-table">
                <thead>
                  <tr>
                    <th>
                      <label className="hr71-check">
                        <input
                          type="checkbox"
                          aria-label={w.selectAll}
                          checked={
                            visible.length > 0 && visible.every((p) => selected.includes(p.id))
                          }
                          onChange={(e) =>
                            setSelected(
                              e.target.checked
                                ? [...new Set([...selected, ...visible.map((p) => p.id)])]
                                : selected.filter((id) => !visible.some((p) => p.id === id))
                            )
                          }
                        />
                      </label>
                    </th>
                    {[w.name, w.job, w.department, w.status, w.expiry, w.actions].map((value) => (
                      <th key={value}>{value}</th>
                    ))}
                  </tr>
                </thead>
                <tbody>
                  {visible.map((p) => (
                    <tr key={p.id}>
                      <td data-label={w.selected}>
                        <label className="hr71-check">
                          <input
                            type="checkbox"
                            aria-label={w.selectPerson + ': ' + p.name}
                            checked={selected.includes(p.id)}
                            onChange={(e) =>
                              setSelected(
                                e.target.checked
                                  ? [...selected, p.id]
                                  : selected.filter((id) => id !== p.id)
                              )
                            }
                          />
                        </label>
                      </td>
                      <td data-label={w.name}>
                        <div className="hr71-person">
                          <span className="hr71-avatar" aria-hidden="true">
                            {initials(p.name)}
                          </span>
                          <div>
                            <strong>{p.name}</strong>
                            <small>{tr('role.' + p.role)}</small>
                          </div>
                        </div>
                      </td>
                      <td data-label={w.job} title={p.jobTitle ? undefined : w.notProvided}>
                        {p.jobTitle || '—'}
                      </td>
                      <td
                        data-label={w.department}
                        title={p.department ? undefined : w.notProvided}
                      >
                        {p.department || '—'}
                      </td>
                      <td data-label={w.status}>
                        <span
                          className={
                            'hr71-badge ' +
                            (p.status === 'access_active'
                              ? 'success'
                              : p.status === 'access_revoked'
                                ? 'neutral'
                                : 'warning')
                          }
                        >
                          <span aria-hidden="true">{p.status === 'access_active' ? '✓' : '◷'}</span>
                          {tr(p.status)}
                        </span>
                      </td>
                      <td data-label={w.expiry} title={p.nextExpiry ? undefined : w.notProvided}>
                        {date(p.nextExpiry)}
                      </td>
                      <td data-label={w.actions}>
                        <details className="hr71-row-menu">
                          <summary aria-label={w.actions + ': ' + p.name}>•••</summary>
                          <div>
                            <button onClick={() => setPerson(p)}>{w.view}</button>
                            {['record', 'request', 'communicate'].map((key) => (
                              <button key={key} disabled title={w.unavailable}>
                                {tr(key)} · {w.coming}
                              </button>
                            ))}
                          </div>
                        </details>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
            <footer className="hr71-pagination">
              <span>
                {w.pagination} {number(currentPage)} / {number(pages)}
              </span>
              <button disabled={currentPage <= 1} onClick={() => setPage(currentPage - 1)}>
                {w.previous}
              </button>
              <button disabled={currentPage >= pages} onClick={() => setPage(currentPage + 1)}>
                {w.next}
              </button>
            </footer>
          </section>
          <aside className="hr71-card hr71-alerts">
            <h2>{w.alerts}</h2>
            {sources.alerts.state === 'loading' ? (
              <p role="status">{w.loading}</p>
            ) : sources.alerts.data?.length ? (
              sources.alerts.data
                .slice()
                .sort(
                  (a, b) =>
                    ({ danger: 0, warning: 1, neutral: 2 })[a.severity] -
                    { danger: 0, warning: 1, neutral: 2 }[b.severity]
                )
                .map((alert) => (
                  <article key={alert.id}>
                    <span className={'hr71-badge ' + alert.severity}>
                      {alert.severity === 'danger' ? '!' : '◷'}{' '}
                      {tr(
                        {
                          trial: 'alertTrial',
                          missing_point: 'alertPoint',
                          document: 'alertDocument',
                          vacation: 'alertVacation',
                          exam: 'alertExam',
                        }[alert.kind]
                      )}
                    </span>
                    <strong>{alert.personName}</strong>
                    <time>{date(alert.due)}</time>
                    <button
                      onClick={() => {
                        const match = people.find((p) => p.id === alert.personId);
                        if (match) setPerson(match);
                      }}
                    >
                      {w.view}
                    </button>
                  </article>
                ))
            ) : (
              <p>{sources.alerts.state === 'empty' ? w.noAlerts : w.alertUnavailable}</p>
            )}
            {capabilities?.role === 'owner' && (
              <section className="hr71-finance">
                <h3>{w.totalPayroll}</h3>
                <strong title={stateHelp(sources.payroll.state)}>
                  {sources.payroll.data ? currency(sources.payroll.data.totalCents, locale) : '—'}
                </strong>
                <h3>{w.totalCost}</h3>
                <strong title={stateHelp(sources.finance.state)}>
                  {sources.finance.data ? currency(sources.finance.data.costCents, locale) : '—'}
                </strong>
                {sources.finance.state === 'not_licensed' && (
                  <>
                    <span className="hr71-badge neutral">{w.notLicensed}</span>
                    <Link to="/local/subscription">{w.plans}</Link>
                  </>
                )}
              </section>
            )}
          </aside>
        </div>
      )}
      <section className="hr71-areas">
        <h2>{w.moreUsed}</h2>
        <div className="hr71-grid">{areas.filter((area) => area.major).map(areaCard)}</div>
        <details className="hr71-other">
          <summary>{w.otherAreas}</summary>
          <div className="hr71-grid">{areas.filter((area) => !area.major).map(areaCard)}</div>
        </details>
      </section>
      {(person || help) && (
        <dialog
          ref={dialog}
          className="hr71-dialog"
          aria-label={person ? w.details : w.help}
          onCancel={() => {
            setPerson(undefined);
            setHelp(false);
          }}
        >
          <header>
            <h2>{person ? person.name : w.help}</h2>
            <button
              autoFocus
              onClick={() => {
                setPerson(undefined);
                setHelp(false);
              }}
            >
              {w.close}
            </button>
          </header>
          {person ? (
            <dl>
              {[
                [w.profile, tr('role.' + person.role)],
                [w.job, person.jobTitle || '—'],
                [w.department, person.department || '—'],
                [w.status, tr(person.status)],
                [w.expiry, date(person.nextExpiry)],
              ].map(([label, value]) => (
                <div key={label}>
                  <dt>{label}</dt>
                  <dd>{value}</dd>
                </div>
              ))}
            </dl>
          ) : (
            <>
              <ol>
                {[w.guideOne, w.guideTwo, w.guideThree, w.guideFour].map((text, index) => (
                  <li key={text}>
                    <span aria-hidden="true">{index === 0 && people.length ? '✓' : '◷'}</span>{' '}
                    {text}
                    {index > 0 && <span> · {w.coming}</span>}
                  </li>
                ))}
              </ol>
              <p>{w.importHelp}</p>
              <button disabled={busy} onClick={() => void exportPeople('xlsx', true)}>
                {w.template} · XLSX
              </button>
            </>
          )}
        </dialog>
      )}
    </main>
  );
}
