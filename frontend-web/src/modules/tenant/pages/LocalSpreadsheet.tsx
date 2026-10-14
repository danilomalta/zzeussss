import { useEffect, useRef, useState } from 'react';
import type { ChangeEvent } from 'react';
import { Navigate, useBlocker } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { useLocalAccess } from '../../../core/local/LocalAccess';
import { allowedAreas } from '../../../core/local/accessModel.mjs';
import { useLocalSession } from '../../../core/local/useLocalSession';
import { useTheme } from '../../../shared/brand/theme';
import {
  emptySnapshot,
  readTitanFile,
  writeTitanFile,
  csvToSnapshot,
  snapshotCSV,
  snapshotToXLSX,
  xlsxToSnapshot,
  pivotRows,
  limits68,
} from '../../../core/local/sheetFiles68.mjs';
import { applyStructure69, previewMatrix69 } from '../../../core/local/sheetActions69.mjs';
import { mountSheetEditor } from './sheetRuntime70';
import pt from '../../../i18n/sheets/pt-BR.json';
import en from '../../../i18n/sheets/en.json';
import es from '../../../i18n/sheets/es.json';
import fr from '../../../i18n/sheets/fr.json';
import zh from '../../../i18n/sheets/zh-CN.json';
import './spreadsheet.css';
import Sheets70, { dictionaries70 } from './Sheets70';
import type { Block } from '../../../core/sheets70/model.mjs';
import { lookup, sourceAllowed } from '../../../core/sheets70/model.mjs';
import type { Cell, DataSet } from '../../../core/sheets70/contracts';
const translations: Record<string, typeof pt> = { 'pt-BR': pt, pt, en, es, fr, 'zh-CN': zh, zh };
type Runtime = ReturnType<typeof mountSheetEditor>;
export default function LocalSpreadsheet() {
  const { i18n } = useTranslation(),
    words = translations[i18n.resolvedLanguage || i18n.language] || pt,
    { theme } = useTheme();
  const { capabilities } = useLocalAccess();
  const session = useLocalSession((s) => s.session),
    token = useLocalSession((s) => s.token);
  const caps70 = useRef(capabilities);
  caps70.current = capabilities;
  const permitted = allowedAreas(capabilities).includes('sheets');
  const scope = [
    session?.tenant_id,
    session?.store_id,
    session?.device_id,
    session?.identity_id,
    token,
  ].join(':');
  const host = useRef<HTMLDivElement>(null),
    runtime = useRef<Runtime | null>(null),
    snapshot = useRef(emptySnapshot()),
    scopeRef = useRef(scope),
    fileRef = useRef<HTMLInputElement>(null),
    skipSave = useRef(false);
  const words70 = dictionaries70[i18n.resolvedLanguage || i18n.language] || dictionaries70['pt-BR'];
  const [printHeader, setPrintHeader] = useState(''),
    [printFooter, setPrintFooter] = useState('');
  const printDialog = useRef<HTMLDialogElement>(null);
  const [expanded, setExpanded] = useState(false),
    [tool, setTool] = useState('rows'),
    [rowHeight, setRowHeight] = useState(28),
    [columnWidth, setColumnWidth] = useState(120),
    [address, setAddress] = useState('A1'),
    [printArea, setPrintArea] = useState('selection'),
    [orientation, setOrientation] = useState('landscape'),
    [preview, setPreview] = useState<(string | number | boolean)[][]>([]);
  const [revision, setRevision] = useState(0),
    [dirty, setDirty] = useState(false),
    [busy, setBusy] = useState(false),
    [error, setError] = useState(''),
    [message, setMessage] = useState(''),
    [name, setName] = useState('Titan'),
    [analysis, setAnalysis] = useState(false),
    [keyCol, setKeyCol] = useState(0),
    [valueCol, setValueCol] = useState(1),
    [groups, setGroups] = useState<{ label: string; sum: number; count: number }[]>([]);
  const blocker = useBlocker(
    ({ currentLocation, nextLocation }) =>
      permitted && dirty && currentLocation.pathname !== nextLocation.pathname
  );
  useEffect(() => {
    if (blocker.state === 'blocked') {
      if (window.confirm(words.leaveConfirm)) blocker.proceed();
      else blocker.reset();
    }
  }, [blocker, words.leaveConfirm]);
  useEffect(() => {
    if (!dirty) return;
    const warn = (event: BeforeUnloadEvent) => {
      event.preventDefault();
      event.returnValue = '';
    };
    window.addEventListener('beforeunload', warn);
    return () => window.removeEventListener('beforeunload', warn);
  }, [dirty]);
  useEffect(() => {
    if (scopeRef.current !== scope) {
      snapshot.current = emptySnapshot();
      lookup70.current = {};
      scopeRef.current = scope;
      setDirty(false);
      setName('Titan');
      setGroups([]);
      setAnalysis(false);
      setError('');
      setMessage('');
    }
    if (!permitted || !host.current) return;
    let listener: { dispose: () => void } | undefined;
    let instance: Runtime | null = null;
    const container = document.createElement('div');
    container.style.cssText = 'width:100%;height:100%';
    host.current.append(container);
    const frame = requestAnimationFrame(() => {
      try {
        instance = mountSheetEditor(
          container,
          i18n.resolvedLanguage || i18n.language,
          theme === 'dark'
        );
        runtime.current = instance;
        const workbook = instance.univerAPI.createWorkbook(snapshot.current);
        instance.univerAPI.getFormula().registerFunction(
          'TITAN',
          (source, field, key) => {
            const s = String(source);
            if (!sourceAllowed(s, caps70.current)) return '#N/A';
            return lookup(lookup70.current, s, String(field), key) as string | number | boolean;
          },
          words70.lookupHelp
        );
        listener = workbook.onCommandExecuted((command) => {
          if (command.id.includes('.mutation.')) {
            window.dispatchEvent(new CustomEvent('titan-sheets70-command', { detail: command }));
            setDirty(true);
          }
        });
      } catch {
        setError(words.error);
      }
    });
    return () => {
      cancelAnimationFrame(frame);
      listener?.dispose();
      container.remove();
      if (instance) {
        if (!skipSave.current && scopeRef.current === scope) {
          const workbook = instance.univerAPI.getActiveWorkbook();
          if (workbook) snapshot.current = workbook.save();
        }
        const old = instance;
        queueMicrotask(() => old.univer.dispose());
        if (runtime.current === instance) runtime.current = null;
      }
      skipSave.current = false;
    };
  }, [scope, permitted, revision]);
  useEffect(() => {
    runtime.current?.univerAPI.toggleDarkMode(theme === 'dark');
  }, [theme]);
  useEffect(() => {
    runtime.current?.univerAPI.setLocale(
      (
        {
          pt: 'ptBR',
          'pt-BR': 'ptBR',
          en: 'enUS',
          es: 'esES',
          fr: 'frFR',
          zh: 'zhCN',
          'zh-CN': 'zhCN',
        } as Record<string, string>
      )[i18n.resolvedLanguage || i18n.language] || 'ptBR'
    );
  }, [i18n.resolvedLanguage, i18n.language]);
  function live() {
    const workbook = runtime.current?.univerAPI.getActiveWorkbook();
    if (!workbook) throw new Error();
    return workbook.save();
  }
  function download(data: BlobPart, type: string, extension: string) {
    const url = URL.createObjectURL(new Blob([data], { type })),
      link = document.createElement('a');
    link.href = url;
    link.download = (name.replace(/[^\p{L}\p{N}_-]/gu, '_').slice(0, 80) || 'Titan') + extension;
    link.click();
    setTimeout(() => URL.revokeObjectURL(url), 1000);
    setMessage(words.saved);
  }
  async function save(kind: 'titan' | 'xlsx' | 'csv') {
    const context = scopeRef.current;
    setBusy(true);
    setError('');
    try {
      const book = live();
      if (kind === 'titan') {
        download(writeTitanFile(book), 'application/json', '.titan.json');
        setDirty(false);
      } else if (kind === 'xlsx') {
        const bytes = await snapshotToXLSX(book);
        if (context !== scopeRef.current) return;
        download(
          new Uint8Array(bytes).buffer,
          'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
          '.xlsx'
        );
      } else {
        const id = runtime.current?.univerAPI.getActiveWorkbook()?.getActiveSheet().getSheetId();
        download(snapshotCSV(book, id), 'text/csv;charset=utf-8', '.csv');
      }
    } catch {
      setError(words.error);
    } finally {
      if (context === scopeRef.current) setBusy(false);
    }
  }
  useEffect(() => {
    if (!permitted) return;
    const keyboard = (event: KeyboardEvent) => {
      if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 's') {
        event.preventDefault();
        void save('titan');
      }
    };
    window.addEventListener('keydown', keyboard);
    return () => window.removeEventListener('keydown', keyboard);
  });
  function replace(book: ReturnType<typeof emptySnapshot>) {
    if (dirty && !window.confirm(words.confirmNew)) return;
    skipSave.current = true;
    snapshot.current = book;
    setName(book.name);
    setDirty(false);
    setGroups([]);
    setError('');
    setMessage('');
    setRevision((r) => r + 1);
  }
  async function open(event: ChangeEvent<HTMLInputElement>) {
    const file = event.target.files?.[0],
      context = scopeRef.current;
    event.target.value = '';
    if (!file) return;
    setBusy(true);
    setError('');
    try {
      if (file.size > limits68.bytes) throw new Error();
      const title =
        file.name.replace(/\.(titan\.)?(json|xlsx|csv|tsv)$/i, '').slice(0, 120) || 'Titan';
      const book = /\.xlsx$/i.test(file.name)
        ? await xlsxToSnapshot(await file.arrayBuffer(), title)
        : /\.(csv|tsv)$/i.test(file.name)
          ? csvToSnapshot(await file.text(), title)
          : readTitanFile(await file.text());
      if (context !== scopeRef.current) return;
      replace(book);
      if (/\.xlsx$/i.test(file.name)) setMessage(words.xlsxNotice);
    } catch {
      if (context === scopeRef.current) setError(words.error);
    } finally {
      if (context === scopeRef.current) setBusy(false);
    }
  }
  function selected() {
    const workbook = runtime.current?.univerAPI.getActiveWorkbook(),
      sheet = workbook?.getActiveSheet(),
      range = workbook?.getActiveRange();
    if (!workbook || !sheet || !range) throw new Error();
    return { workbook, sheet, range };
  }
  function structure(operation: string, size?: number) {
    try {
      const { sheet, range } = selected();
      if (operation.startsWith('delete') && !window.confirm(words.deleteSelection69)) return;
      applyStructure69(sheet, range, operation, size);
      setDirty(true);
      setError('');
    } catch {
      setError(words.operationError69);
    }
  }
  function go() {
    try {
      if (!/^[A-Z]{1,3}[1-9]\d{0,4}(?::[A-Z]{1,3}[1-9]\d{0,4})?$/.test(address.toUpperCase()))
        throw new Error();
      const { workbook, sheet } = selected(),
        range = sheet.getRange(address.toUpperCase());
      if (
        range.getLastRow() >= sheet.getMaxRows() ||
        range.getLastColumn() >= sheet.getMaxColumns()
      )
        throw new Error();
      workbook.setActiveRange(range);
      range.activate();
      setError('');
    } catch {
      setError(words.operationError69);
    }
  }
  function format69(action: string) {
    try {
      const { range } = selected();
      if (action === 'bold') range.setFontWeight('bold');
      if (action === 'normal') range.setFontWeight('normal');
      if (action === 'wrap') range.setWrap(true);
      if (action === 'unwrap') range.setWrap(false);
      if (action === 'merge') range.merge();
      if (action === 'unmerge') range.breakApart();
      if (action === 'currency') range.setNumberFormat('\"R$\" #,##0.00');
      if (action === 'percent') range.setNumberFormat('0.00%');
      if (action === 'borders') {
        const api = runtime.current!.univerAPI;
        range.setBorder(
          api.Enum.BorderType.ALL,
          api.Enum.BorderStyleTypes.THIN,
          theme === 'dark' ? '#cccccc' : '#444444'
        );
      }
      setDirty(true);
      setError('');
    } catch {
      setError(words.operationError69);
    }
  }
  const lookup70 = useRef<Record<string, DataSet>>({});
  function writeBlock70(matrix: Cell[][], block: Block, previous?: Block, newSheet?: boolean) {
    const workbook = runtime.current?.univerAPI.getActiveWorkbook();
    if (!workbook) throw Error();
    if (newSheet && workbook.save().sheetOrder.length >= limits68.sheets) throw Error();
    const sheet = newSheet ? workbook.insertSheet() : workbook.getSheetBySheetId(block.sheetId);
    if (!sheet) throw Error();
    const result = {
      ...block,
      sheetId: sheet.getSheetId(),
      row: newSheet ? 0 : block.row,
      col: newSheet ? 0 : block.col,
    };
    if (result.row + matrix.length > sheet.getMaxRows())
      sheet.insertRowsAfter(
        sheet.getMaxRows() - 1,
        result.row + matrix.length - sheet.getMaxRows()
      );
    if (result.col + matrix[0].length > sheet.getMaxColumns())
      sheet.insertColumnsAfter(
        sheet.getMaxColumns() - 1,
        result.col + matrix[0].length - sheet.getMaxColumns()
      );
    if (previous) {
      for (let r = previous.row; r < previous.row + previous.height; r++)
        for (let c = previous.col; c < previous.col + previous.width; c++)
          if (r >= result.row + result.height || c >= result.col + result.width)
            sheet.getRange(r, c, 1, 1).setValue('');
    }
    sheet.getRange(result.row, result.col, matrix.length, matrix[0].length).setValues(matrix);
    const api = runtime.current!.univerAPI;
    sheet
      .getRange(result.row, result.col, matrix.length, matrix[0].length)
      .setBorder(api.Enum.BorderType.OUTSIDE, api.Enum.BorderStyleTypes.THIN, '#a57b36');
    sheet.getRange(result.row, result.col, 1, matrix[0].length).setFontWeight('bold');
    if (matrix.length > 1)
      block.columns?.forEach((column, i) => {
        if (['price', 'cost', 'total', 'amount'].includes(column))
          sheet
            .getRange(result.row + 1, result.col + i, matrix.length - 1, 1)
            .setNumberFormat('\"R$\" #,##0.00');
      });
    setDirty(true);
    return result;
  }
  function showPrint() {
    try {
      const { workbook, sheet, range } = selected();
      setPreview(
        previewMatrix69(
          workbook.save(),
          sheet.getSheetId(),
          printArea === 'selection' ? range.getRange() : undefined
        )
      );
      setError('');
    } catch {
      setError(words.printLimit69);
    }
  }
  useEffect(() => {
    if (preview.length && !printDialog.current?.open) printDialog.current?.showModal();
  }, [preview]);
  function calculate() {
    try {
      const book = live(),
        id = runtime.current?.univerAPI.getActiveWorkbook()?.getActiveSheet().getSheetId();
      setGroups(pivotRows(book, id, keyCol, valueCol));
      setError('');
    } catch {
      setError(words.error);
    }
  }
  function insertSummary() {
    try {
      const workbook = runtime.current?.univerAPI.getActiveWorkbook();
      if (!workbook || !groups.length || workbook.save().sheetOrder.length >= limits68.sheets)
        throw new Error();
      const sheet = workbook.insertSheet();
      sheet
        .getRange(0, 0, groups.length + 1, 3)
        .setValues([
          [words.category, words.sum, words.cells],
          ...groups.map((g) => [g.label, g.sum, g.count]),
        ]);
      setDirty(true);
    } catch {
      setError(words.error);
    }
  }
  if (!permitted) return <Navigate to="/local/home" replace />;
  const peak = Math.max(1, ...groups.map((g) => Math.abs(g.sum))),
    format = (value: number) =>
      new Intl.NumberFormat(i18n.resolvedLanguage || 'pt-BR', { maximumFractionDigits: 2 }).format(
        value
      );
  return (
    <main className="brand-app-content sheet-page sheet68-page sheet70-page">
      <header className="sheet68-heading">
        <div>
          <h1>{words.title}</h1>
        </div>
        <label>
          {words.name}
          <input
            maxLength={120}
            value={name}
            onChange={(event) => {
              setName(event.target.value);
              runtime.current?.univerAPI.getActiveWorkbook()?.setName(event.target.value);
              setDirty(true);
            }}
          />
        </label>
      </header>
      {busy && <span role="status">{words.working}</span>}
      <Sheets70
        name={name}
        get={live}
        load={(book) => {
          skipSave.current = true;
          snapshot.current = book;
          setName(book.name);
          setDirty(true);
          setRevision((r) => r + 1);
        }}
        saved={() => setDirty(false)}
        selection={() => {
          const { sheet, range } = selected(),
            r = range.getRange();
          return {
            sheetId: sheet.getSheetId(),
            row: r.startRow,
            col: r.startColumn,
            height: r.endRow - r.startRow + 1,
            width: r.endColumn - r.startColumn + 1,
          };
        }}
        write={writeBlock70}
        tools={() => setExpanded((v) => !v)}
        download={(kind) => void save(kind)}
        open={() => fileRef.current?.click()}
        create={() => replace(emptySnapshot())}
        summary={() => setAnalysis((v) => !v)}
        freeze={(active) => {
          const { sheet } = selected();
          sheet.setFrozenRows(active ? 1 : 0);
          sheet.setFrozenColumns(active ? 1 : 0);
        }}
        clearCache={() => {
          lookup70.current = {};
        }}
        activate={(id, row, col) => {
          const book = runtime.current?.univerAPI.getActiveWorkbook(),
            sheet = book?.getSheetBySheetId(id);
          if (sheet) {
            book?.setActiveSheet(sheet);
            sheet.getRange(row, col, 1, 1).activate();
          }
        }}
        cache={(source, data) => {
          lookup70.current[source] = data;
          runtime.current?.univerAPI.getFormula().executeCalculation();
        }}
      />
      <section hidden={!expanded} className="sheet69-ribbon" aria-label={words.tools69}>
        <nav>
          {(['rows', 'columns', 'format', 'print'] as const).map((tab) => (
            <button key={tab} aria-pressed={tool === tab} onClick={() => setTool(tab)}>
              {
                words[
                  tab === 'rows'
                    ? 'rows69'
                    : tab === 'columns'
                      ? 'columns69'
                      : tab === 'format'
                        ? 'format69'
                        : 'print69'
                ]
              }
            </button>
          ))}
        </nav>
        <div className="sheet69-tools">
          <label>
            {words.range69}
            <input
              value={address}
              onChange={(e) => setAddress(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter') go();
              }}
              placeholder="A1:C10"
              maxLength={24}
            />
          </label>
          <button onClick={go}>{words.select69}</button>
          {tool === 'rows' ? (
            <>
              <button onClick={() => structure('insertRows')}>{words.insertRows69}</button>
              <button onClick={() => structure('deleteRows')}>{words.deleteRows69}</button>
              <button onClick={() => structure('hideRows')}>{words.hide69}</button>
              <button onClick={() => structure('showRows')}>{words.show69}</button>
              <label>
                {words.height69}
                <input
                  type="number"
                  min={16}
                  max={400}
                  value={rowHeight}
                  onChange={(e) => setRowHeight(Number(e.target.value))}
                />
              </label>
              <button onClick={() => structure('height', rowHeight)}>{words.apply69}</button>
              <button onClick={() => structure('autoHeight')}>{words.autoHeight69}</button>
            </>
          ) : tool === 'columns' ? (
            <>
              <button onClick={() => structure('insertColumns')}>{words.insertColumns69}</button>
              <button onClick={() => structure('deleteColumns')}>{words.deleteColumns69}</button>
              <button onClick={() => structure('hideColumns')}>{words.hide69}</button>
              <button onClick={() => structure('showColumns')}>{words.show69}</button>
              <label>
                {words.width69}
                <input
                  type="number"
                  min={30}
                  max={800}
                  value={columnWidth}
                  onChange={(e) => setColumnWidth(Number(e.target.value))}
                />
              </label>
              <button onClick={() => structure('width', columnWidth)}>{words.apply69}</button>
              <button onClick={() => structure('showAll')}>{words.showAll69}</button>
            </>
          ) : tool === 'format' ? (
            <>
              {(
                [
                  'bold',
                  'normal',
                  'wrap',
                  'unwrap',
                  'merge',
                  'unmerge',
                  'currency',
                  'percent',
                  'borders',
                ] as const
              ).map((action) => (
                <button key={action} onClick={() => format69(action)}>
                  {words[(action + '69') as keyof typeof words]}
                </button>
              ))}
            </>
          ) : (
            <>
              <label>
                {words.printArea69}
                <select value={printArea} onChange={(e) => setPrintArea(e.target.value)}>
                  <option value="selection">{words.selection69}</option>
                  <option value="used">{words.used69}</option>
                </select>
              </label>
              <label>
                {words.orientation69}
                <select value={orientation} onChange={(e) => setOrientation(e.target.value)}>
                  <option value="portrait">{words.portrait69}</option>
                  <option value="landscape">{words.landscape69}</option>
                </select>
              </label>
              <label>
                {words70.printHeader}
                <input value={printHeader} onChange={(e) => setPrintHeader(e.target.value)} />
              </label>
              <label>
                {words70.printFooter}
                <input value={printFooter} onChange={(e) => setPrintFooter(e.target.value)} />
              </label>
              <button onClick={showPrint}>{words.preview69}</button>
            </>
          )}
        </div>
        <p>{words.selectionHelp69}</p>
      </section>
      {error && (
        <div className="sheet-message error" role="alert">
          {error}
          <button onClick={() => setError('')}>{words.close}</button>
        </div>
      )}
      {message && (
        <div className="sheet-message" role="status">
          {message}
          <button onClick={() => setMessage('')}>{words.close}</button>
        </div>
      )}
      <div className="sheet68-workspace">
        <div ref={host} className="sheet68-engine" aria-label={words.title} />
        {analysis && (
          <aside className="sheet68-analysis">
            <h2>{words.analysis}</h2>
            <p>{words.analysisHelp}</p>
            <label>
              {words.category}
              <select value={keyCol} onChange={(e) => setKeyCol(Number(e.target.value))}>
                {Array.from({ length: 50 }, (_, i) => (
                  <option key={i} value={i}>
                    {i + 1}
                  </option>
                ))}
              </select>
            </label>
            <label>
              {words.valueColumn}
              <select value={valueCol} onChange={(e) => setValueCol(Number(e.target.value))}>
                {Array.from({ length: 50 }, (_, i) => (
                  <option key={i} value={i}>
                    {i + 1}
                  </option>
                ))}
              </select>
            </label>
            <button onClick={calculate}>{words.refreshAnalysis}</button>
            <button disabled={!groups.length} onClick={insertSummary}>
              {words.insertSummary}
            </button>
            {groups.length ? (
              <>
                <table>
                  <thead>
                    <tr>
                      <th>{words.category}</th>
                      <th>{words.sum}</th>
                      <th>{words.cells}</th>
                    </tr>
                  </thead>
                  <tbody>
                    {groups.slice(0, 200).map((g) => (
                      <tr key={g.label}>
                        <td>{g.label}</td>
                        <td>{format(g.sum)}</td>
                        <td>{g.count}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
                <div className="sheet68-bars">
                  {groups.slice(0, 12).map((g) => (
                    <div key={g.label}>
                      <span>{g.label}</span>
                      <i style={{ width: `${(Math.abs(g.sum) / peak) * 100}%` }} />
                      <strong>{format(g.sum)}</strong>
                    </div>
                  ))}
                </div>
              </>
            ) : (
              <p>{words.chartEmpty}</p>
            )}
          </aside>
        )}
      </div>
      <details className="sheet68-help">
        <summary>{words.compatibility}</summary>
        <p>{words.xlsxNotice}</p>
        <p>{words.limits68}</p>
        <p>{words.formulas68}</p>
        <p>{words70.lookupHelp}</p>
      </details>
      <dialog ref={printDialog} className="sheet69-printdialog" onClose={() => setPreview([])}>
        <header>
          <h2>{words.preview69}</h2>
          <button onClick={() => window.print()}>{words.printPdf69}</button>
          <button onClick={() => printDialog.current?.close()}>{words.close}</button>
        </header>
        <p>
          {printHeader || name} · {words.printValues69}
        </p>
        <style>{`@media print{@page{size:A4 ${orientation};margin:12mm}}`}</style>
        <table>
          <tbody>
            {preview.map((row, r) => (
              <tr key={r}>
                {row.map((v, c) => (
                  <td key={c}>{typeof v === 'number' ? format(v) : String(v)}</td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
        <footer>{printFooter}</footer>
      </dialog>
      <input
        ref={fileRef}
        type="file"
        accept=".json,.xlsx,.csv,.tsv"
        hidden
        onChange={(event) => void open(event)}
      />
    </main>
  );
}
