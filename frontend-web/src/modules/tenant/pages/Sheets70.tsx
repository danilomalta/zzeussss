import {sheetFinanceWords} from '../../../core/sheets70/financeWords.mjs';
import {sheetStockLocations} from '../../../core/sheets70/stockSnapshot.mjs';
import {sheetStockWords} from '../../../core/sheets70/stockWords';
import { useEffect, useRef, useState } from 'react';
import type { ICellData, IWorkbookData } from '@univerjs/presets';
import { useSearchParams } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { LocalAPIError } from '../../../core/local/localClient.mjs';
import { useLocalSession } from '../../../core/local/useLocalSession';
import { useLocalAccess } from '../../../core/local/LocalAccess';
import {
  namespace,
  cleanSnapshot,
  checkSpace,
  sourceAllowed,
  tableCSV,
  usedMatrix,
  validateImport,
  trackStructure,
  ensureNoOverlap,
  selectionMatrix,
} from '../../../core/sheets70/model.mjs';
import type { Block } from '../../../core/sheets70/model.mjs';
import { sheetStorage } from '../../../core/sheets70/persistence';
import type { Document } from '../../../core/sheets70/store.mjs';
import { templateBook } from '../../../core/sheets70/templates.mjs';
import { sources } from '../../../core/sheets70/sources';
import type { DataSet, Filters, Cell } from '../../../core/sheets70/contracts';
import pt from '../../../i18n/sheets70/pt-BR.json';
import en from '../../../i18n/sheets70/en.json';
import es from '../../../i18n/sheets70/es.json';
import fr from '../../../i18n/sheets70/fr.json';
import zh from '../../../i18n/sheets70/zh-CN.json';
import './sheets70.css';
export const dictionaries70: Record<string, typeof pt> = {
  'pt-BR': pt,
  pt,
  en,
  es,
  fr,
  'zh-CN': zh,
  zh,
};
interface Props {
  ready: boolean;
  name: string;
  get: () => IWorkbookData;
  load: (book: IWorkbookData) => void;
  saved: () => void;
  selection: () => { sheetId: string; row: number; col: number; height?: number; width?: number };
  write: (matrix: Cell[][], block: Block, previous?: Block, newSheet?: boolean) => Block;
  tools: () => void;
  download: (kind: 'titan' | 'xlsx' | 'csv') => void;
  open: () => void;
  create: () => void;
  summary: () => void;
  freeze: (active: boolean) => void;
  cache: (source: string, data: DataSet) => void;
  clearCache: () => void;
  activate: (sheetId: string, row: number, col: number) => void;
}
export default function Sheets70(props: Props) {
  const { i18n } = useTranslation(),
    w = dictionaries70[i18n.resolvedLanguage || i18n.language] || pt;
  const [parameters] = useSearchParams();
  const token = useLocalSession((s) => s.token),
    session = useLocalSession((s) => s.session),
    { capabilities } = useLocalAccess();
  const scope = session ? namespace(session) : '';
  const latest = useRef(props);
  latest.current = props;
  const doc = useRef<Document | null>(null),
    blocks = useRef<Block[]>([]),
    serial = useRef(''),
    queue = useRef(false),
    pendingRefresh=useRef(''),
    mounted = useRef(true),
    controller = useRef<AbortController>();
  const [blockErrors,setBlockErrors]=useState<Record<string,string>>({});
  const [stockLocations,setStockLocations]=useState<{id:string;name:string}[]>([]);
  const financeWords=sheetFinanceWords(i18n.resolvedLanguage||i18n.language);
  const stockWords=sheetStockWords(i18n.resolvedLanguage||i18n.language);
  const [panel, setPanel] = useState(parameters.has('source') ? 'data' : ''),
    [status, setStatus] = useState(''),
    [files, setFiles] = useState<Document[]>([]),
    [query, setQuery] = useState(''),
    [error, setError] = useState(''),
    [source, setSource] = useState(
      Object.keys(sources).includes(parameters.get('source') || '')
        ? parameters.get('source')!
        : 'catalog'
    ),
    [filters, setFilters] = useState<Filters>({
      q: parameters.get('q')?.slice(0, 240) || '',
      from: parameters.get('from') || '',
      to: parameters.get('to') || '',
    }),
    [data, setData] = useState<DataSet>(),
    [columns, setColumns] = useState<string[]>([]),
    [target, setTarget] = useState('selection'),
    [loading, setLoading] = useState(false),
    [blockList, setBlockList] = useState<Block[]>([]),
    [editBlock, setEditBlock] = useState<Block>(),
    [importKind, setImportKind] = useState('prices'),
    [mapping, setMapping] = useState<string[]>([]),
    [previewRows, setPreviewRows] = useState<Cell[][]>([]),
    [searchText, setSearchText] = useState(''),
    [replacement, setReplacement] = useState(''),
    [sendScope, setSendScope] = useState('sheet'),
    [destinationRow, setDestinationRow] = useState(1),
    [destinationCol, setDestinationCol] = useState(1);
  const auth = useRef({ token, capabilities, scope });
  auth.current = { token, capabilities, scope };
  const dialog = useRef<HTMLDialogElement>(null),
    focusReturn = useRef<HTMLElement | null>(null);
  useEffect(() => {
    if (panel) {
      focusReturn.current = document.activeElement as HTMLElement;
      dialog.current?.showModal();
    }
    return () => {
      dialog.current?.close();
      focusReturn.current?.focus();
    };
  }, [panel]);
  const contextRef = useRef(scope);
  contextRef.current = scope;
  useEffect(() => {
    const track = (event: Event) => {
      blocks.current = trackStructure(blocks.current, (event as CustomEvent).detail);
      setBlockList([...blocks.current]);
    };
    window.addEventListener('titan-sheets70-command', track);
    return () => window.removeEventListener('titan-sheets70-command', track);
  }, []);
  const label = (key: string) => financeWords[key] || (stockWords as Record<string,string>)[key] || (w as Record<string, string>)[key] || key;
  const time = (at: number) =>
    new Intl.DateTimeFormat(i18n.resolvedLanguage || 'pt-BR', {
      dateStyle: 'short',
      timeStyle: 'short',
    }).format(at);
  function activity(action: string) {
    if (doc.current)
      doc.current.activity = [...doc.current.activity, { at: Date.now(), action }].slice(-100);
  }
  async function persist(force = false) {
    if (!latest.current.ready || queue.current || !scope) return false;
    queue.current = true;
    try {
      const live = latest.current.get();
      if (doc.current && doc.current.workbook.id !== live.id) {
        doc.current = null;
        blocks.current = [];
        setBlockList([]);setBlockErrors({});
        serial.current = '';
      }
      const safe = cleanSnapshot(live, blocks.current),
        content = JSON.stringify([safe, blocks.current, latest.current.name]);
      if (content === serial.current && !force) {
        latest.current.saved();
        return true;
      }
      if (content.length > 4000000) throw Error('limit');
      setStatus('saving');
      const current = doc.current || {
        id: crypto.randomUUID(),
        name: latest.current.name,
        workbook: safe,
        blocks: [],
        activity: [{ at: Date.now(), action: 'create' }],
        revision: 0,
        updated: 0,
        versions: [],
      };
      const saved = await sheetStorage().save(
        scope,
        {
          ...current,
          name: latest.current.name,
          workbook: safe,
          blocks: structuredClone(blocks.current),
        },
        current.revision
      );
      if (!mounted.current || contextRef.current !== scope) return;
      doc.current = saved;
      serial.current = content;
      setStatus('saved:' + saved.updated);
      if (
        JSON.stringify([
          cleanSnapshot(latest.current.get(), blocks.current),
          blocks.current,
          latest.current.name,
        ]) === content
      )
        latest.current.saved();
      return true;
    } catch {
      if (mounted.current) {
        setStatus('saveError');
        setError('saveError');
      }
      return false;
    } finally {
      queue.current = false;
    }
  }
  useEffect(() => {
    mounted.current = true;
    doc.current = null;
    blocks.current = [];
    serial.current = '';
    pendingRefresh.current='';
    setBlockErrors({});
    let bootstrap: number | undefined;
    void sheetStorage()
      .list(scope)
      .then((saved) => {
        if (parameters.has('report') || !mounted.current || contextRef.current !== scope || !saved.length) return;
        bootstrap = window.setTimeout(() => {
          if (!mounted.current || doc.current) return;
          const file = saved[0];
          doc.current = file;
          blocks.current = file.blocks || [];
          setBlockList(blocks.current);
          pendingRefresh.current=file.workbook.id;
          latest.current.load(file.workbook);
        }, 300);
      })
      .catch(() => setError('saveError'));
    const timer = setInterval(() => void persist(), 2000);
    return () => {
      mounted.current = false;
      clearTimeout(bootstrap);
      clearInterval(timer);
      controller.current?.abort();
    };
  }, [scope]);
  useEffect(() => {
    const keyboard = (event: KeyboardEvent) => {
      if (!(event.ctrlKey || event.metaKey)) return;
      const key = event.key.toLowerCase();
      if(!latest.current.ready){if(['s','f','h'].includes(key)){event.preventDefault();event.stopImmediatePropagation();}return;}
      if (key === 's') {
        event.preventDefault();
        event.stopImmediatePropagation();
        void persist(true);
      }
      if (key === 'f' || key === 'h') {
        event.preventDefault();
        setPanel(key === 'f' ? 'find' : 'replace');
      }
    };
    window.addEventListener('keydown', keyboard, true);
    return () => window.removeEventListener('keydown', keyboard, true);
  });
  useEffect(() => {
    const revoke = blocks.current.filter((b) => !sourceAllowed(b.source, capabilities));
    if (!revoke.length) return;
    try {
      const safe = cleanSnapshot(latest.current.get(), revoke);
      latest.current.clearCache();
      latest.current.load(safe);
      setError('permission');
    } catch {
      /* Runtime is mounting. */
    }
  }, [capabilities]);
  async function show(name: string) {
    setError('');
    setPanel(name);
    if (name === 'files' || name === 'copy')
      try {
        setFiles(await sheetStorage().list(scope));
      } catch {
        setError('saveError');
      }
    if (name === 'send' || name === 'share')
      try {
        setPreviewRows(usedMatrix(props.get(), props.selection().sheetId));
      } catch {
        setError('empty');
      }
  }
  async function loadFile(file: Document) {
    if (!(await persist()) && !window.confirm(w.discard)) return;
    controller.current?.abort();
    doc.current = file;
    blocks.current = file.blocks || [];
    setBlockList(blocks.current);
    serial.current = '';
    latest.current.clearCache();
    pendingRefresh.current=file.workbook.id;
    setBlockErrors({});
    latest.current.load(file.workbook);
    setPanel('');
  }
  function scrubSource(source: string, status: number) {
    try {
      latest.current.clearCache();
      latest.current.load(
        cleanSnapshot(
          latest.current.get(),
          blocks.current.filter((b) => b.source === source)
        )
      );
      setData(undefined);
      setError('permission');
      if (status === 401) useLocalSession.getState().invalidate();
    } catch {
      setError('permission');
    }
  }
  function sourceContext(selected:string,signal:AbortSignal){
    if(!token||!session||!capabilities)throw Error('permission');
    const origin=[session.tenant_id,session.store_id,session.identity_id,session.device_id];
    const sameSession=()=>{const current=useLocalSession.getState();return mounted.current&&current.token===token&&origin.every((value,index)=>value===[current.session?.tenant_id,current.session?.store_id,current.session?.identity_id,current.session?.device_id][index]);};
    return {token,session,capabilities,signal,locale:i18n.resolvedLanguage||i18n.language,sameSession,isCurrent:()=>sameSession()&&sourceAllowed(selected,auth.current.capabilities)};
  }
  useEffect(()=>{
    setStockLocations([]);if(panel!=='data'||source!=='stock')return;
    const ctrl=new AbortController();
    try{void sheetStockLocations(sourceContext('stock',ctrl.signal)).then(value=>{if(!ctrl.signal.aborted)setStockLocations(value);}).catch(()=>{if(!ctrl.signal.aborted)setError('unavailable');});}catch{setError('permission');}
    return()=>ctrl.abort();
  },[panel,source,token,session,capabilities]);
  async function fetchData() {
    controller.current?.abort();
    const ctrl = new AbortController();
    controller.current = ctrl;
    setLoading(true);
    setError('');
    setData(undefined);
    let attempt:ReturnType<typeof sourceContext>|undefined;
    try {
      if (!token || !capabilities || !sourceAllowed(source, capabilities))
        throw Error('permission');
      attempt=sourceContext(source,ctrl.signal);
      const result = await sources[source](attempt, filters);
      if (
        ctrl.signal.aborted ||
        !attempt.sameSession() ||
        !mounted.current ||
        auth.current.token !== token ||
        contextRef.current !== scope ||
        !sourceAllowed(source, auth.current.capabilities)
      )
        return;
      setData(result);
      setColumns(result.columns);
      props.cache(source, result);
    } catch (e) {
      if(attempt&&!attempt.sameSession())return;
      if (e instanceof LocalAPIError && (e.status === 401 || e.status === 403))
        scrubSource(source, e.status);
      if(e instanceof Error&&['financeSheetRange','financeSheetLimit','financeSheetRows'].includes(e.message)&&!ctrl.signal.aborted){setError(e.message);return;}
      if (!ctrl.signal.aborted)
        setError(
          (e instanceof LocalAPIError && [401, 403].includes(e.status)) ||
            (e instanceof Error && e.message === 'permission')
            ? 'permission'
            : 'unavailable'
        );
    } finally {
      if (!ctrl.signal.aborted) setLoading(false);
    }
  }
  async function fetchBlock(block: Block) {
    if (!latest.current.ready || !token || !capabilities || !sourceAllowed(block.source, capabilities)) return;
    let attempt:ReturnType<typeof sourceContext>|undefined;
    let workbookID='';
    const sameDocument=()=>{try{return latest.current.ready&&latest.current.get().id===workbookID&&blocks.current.some(value=>value.id===block.id);}catch{return false;}};
    try {
      workbookID=latest.current.get().id;
      const ctrl = new AbortController();
      attempt=sourceContext(block.source,ctrl.signal);
      const result = await sources[block.source](
        attempt,
        block.filters || { q: '', from: '', to: '' }
      );
      if (
        !attempt.sameSession() ||
        !sameDocument() ||
        !mounted.current ||
        auth.current.token !== token ||
        !sourceAllowed(block.source, auth.current.capabilities)
      )
        return;
      const selectedColumns = block.columns || result.columns,
        indexes = selectedColumns.map((c) => result.columns.indexOf(c));
      if (indexes.includes(-1)) throw Error('permission');
      const matrix = [
        selectedColumns.map(label),
        ...result.rows.map((row) => indexes.map((i) => row[i])),
      ];
      const checked = checkSpace(props.get(), block, matrix, block);
      ensureNoOverlap(checked, blocks.current);
      const updated = props.write(matrix, { ...checked, updated: result.at, partial: result.partial }, block);
      blocks.current = blocks.current.map((b) => (b.id === block.id ? updated : b));
      setBlockList([...blocks.current]);
      props.cache(block.source, result);
      activity('refresh');
      setBlockErrors(previous=>{const next={...previous};delete next[block.id];return next;});
    } catch (error) {
      if(attempt&&!attempt.sameSession()||!sameDocument())return;
      const message=error instanceof LocalAPIError&&[401,403].includes(error.status)?'permission':error instanceof Error&&['financeSheetRange','financeSheetLimit','financeSheetRows'].includes(error.message)?error.message:'offline';
      setBlockErrors(previous=>({...previous,[block.id]:message}));
      if (error instanceof LocalAPIError && (error.status === 401 || error.status === 403))
        scrubSource(block.source, error.status);

    }
  }

  useEffect(() => {
    const timer = setInterval(() => {
      for (const b of blocks.current)
        if (!b.detached && b.interval > 0 && Date.now() - b.updated > b.interval * 60000)
          void fetchBlock(b);
    }, 60000);
    return () => clearInterval(timer);
  }, [token, capabilities]);
  useEffect(()=>{
    if(!props.ready||!pendingRefresh.current)return;
    try{if(latest.current.get().id!==pendingRefresh.current)return;}catch{return;}
    pendingRefresh.current='';
    for(const block of blocks.current.filter(value=>!value.detached))void fetchBlock(block);
  },[props.ready]);
  function insert() {
    if (!data) return;
    if (!sourceAllowed(source, auth.current.capabilities)) {
      setError('permission');
      return;
    }
    try {
      const indexes = columns.map((c) => data.columns.indexOf(c)),
        matrix = [columns.map(label), ...data.rows.map((r) => indexes.map((i) => r[i]))];
      if (!matrix[0].length) throw Error('empty');
      const selected = props.selection(),
        base = editBlock || {
          id: crypto.randomUUID(),
          source,
          ...selected,
          height: 0,
          width: 0,
          updated: 0,
          interval: 0,
        };
      const checked =
        target === 'newSheet' && !editBlock
          ? { ...base, height: matrix.length, width: matrix[0].length }
          : checkSpace(props.get(), base, matrix, editBlock);
      if (target !== 'newSheet') ensureNoOverlap(checked, blocks.current);
      const result = props.write(
        matrix,
        { ...checked, updated: data.at, partial: data.partial, filters: { ...filters }, columns: [...columns] },
        editBlock,
        target === 'newSheet'
      );
      blocks.current = editBlock
        ? blocks.current.map((b) => (b.id === result.id ? result : b))
        : [...blocks.current, result];
      setBlockList([...blocks.current]);
      activity('link');
      setPanel('');
      setEditBlock(undefined);
    } catch {
      setError('conflict');
    }
  }
  async function template(kind: string) {
    if (!(await persist())) return;
    const book = templateBook(props.get(), kind, label);
    doc.current = null;
    blocks.current = [];
    setBlockList([]);setBlockErrors({});
    serial.current = '';
    props.clearCache();
    props.load(book);
    setPanel('');
  }

  function find(all = false) {
    try {
      if (!searchText) return;
      const book = structuredClone(props.get());
      let hits = 0;
      for (const sheet of Object.values(book.sheets))
        for (const [r, row] of Object.entries(sheet.cellData || {}))
          for (const [c, cell] of Object.entries(row) as [string, ICellData][]) {
            if (typeof cell.v === 'string' && cell.v.includes(searchText)) {
              hits++;
              if (!all && hits === 1) {
                props.activate(sheet.id || '', Number(r), Number(c));
                setPanel('');
              }
              if (all) cell.v = cell.v.split(searchText).join(replacement);
            }
          }
      if (all) {
        props.load(book);
        activity('replace');
      }
      setStatus(String(hits));
    } catch {
      setError('empty');
    }
  }
  async function copySelection(file: Document) {
    if (!window.confirm(w.confirm)) return;
    try {
      const matrix = selectionMatrix(props.get(), props.selection()),
        book = structuredClone(file.workbook),
        sheetId = book.sheetOrder[0];
      if (
        !Number.isInteger(destinationRow) ||
        destinationRow < 1 ||
        !Number.isInteger(destinationCol) ||
        destinationCol < 1
      )
        throw Error();
      const area = {
        id: crypto.randomUUID(),
        source: 'manual',
        sheetId,
        row: destinationRow - 1,
        col: destinationCol - 1,
        height: matrix.length,
        width: matrix[0].length,
        updated: Date.now(),
        interval: 0,
      };
      checkSpace(book, area, matrix);
      ensureNoOverlap(area, file.blocks);
      const sheet = book.sheets[sheetId];
      sheet.cellData ||= {};
      matrix.forEach((row, r) => {
        sheet.cellData![area.row + r] ||= {};
        row.forEach((v, c) => {
          sheet.cellData![area.row + r][area.col + c] = { v };
        });
      });
      const linked = blocks.current.filter((b) => b.sheetId === props.selection().sheetId);
      const provenance = linked.map((b) => ({
        ...area,
        id: crypto.randomUUID(),
        source: b.source,
        detached: true,
      }));
      await sheetStorage().save(
        scope,
        {
          ...file,
          workbook: cleanSnapshot(book, provenance),
          blocks: [...file.blocks, ...provenance],
          activity: [...file.activity, { at: Date.now(), action: 'copy' }],
        },
        file.revision
      );
      setPanel('');
    } catch {
      setError('conflict');
    }
  }
  function externalShare() {
    void (async () => {
      if (!window.confirm(w.sensitive)) return;
      const file = new File([tableCSV(previewRows)], 'Titan.csv', { type: 'text/csv' });
      try {
        if (navigator.canShare?.({ files: [file] })) await navigator.share({ files: [file] });
        else props.download('xlsx');
      } catch {
        setError('unavailable');
      }
    })();
  }
  return (
    <>
      <div className="sheet70-bar">
        <details>
          <summary>{w.file}</summary>
          <div className="sheet70-menu">
            <button
              onClick={() => {
                void persist().then((ok) => {
                  if (!ok) return;
                  doc.current = null;
                  blocks.current = [];
                  setBlockList([]);setBlockErrors({});
                  serial.current = '';
                  props.create();
                });
              }}
            >
              {label('newSheet')}
            </button>
            <button onClick={props.open}>{w.openFile}</button>
            <button onClick={() => void show('copy')}>{w.copyFile}</button>
            {(['titan', 'xlsx', 'csv'] as const).map((k) => (
              <button key={k} onClick={() => props.download(k)}>
                {k.toUpperCase()}
              </button>
            ))}
            <button onClick={() => void show('files')}>{w.files}</button>
            <button onClick={() => void show('templates')}>{w.templates}</button>
            <button onClick={() => void show('versions')}>{w.versions}</button>
            <button onClick={props.summary}>{label('preview')}</button>
            <button disabled>
              {w.pivot} · {w.coming}
            </button>
          </div>
        </details>
        <button onClick={props.tools}>{w.tools}</button>
        <button onClick={() => props.freeze(true)}>{w.freeze}</button>
        <button onClick={() => props.freeze(false)}>{w.unfreeze}</button>
        <button onClick={() => void show('data')}>{w.insert}</button>
        <button onClick={() => void show('share')}>{w.share}</button>
        <button onClick={() => void show('send')}>{w.send}</button>
        <button onClick={() => void show('activity')}>{w.activity}</button>
        {status === 'saveError' && <button onClick={() => void persist(true)}>{w.saveNow}</button>}
        <span role="status">
          {status.startsWith('saved:')
            ? w.saved + ' ' + time(Number(status.slice(6)))
            : label(status || 'local')}
        </span>
      </div>
      <div className="sheet70-links">
        {blockList.map((b) => (
          <details key={b.id}>
            <summary>
              ↗ {label(b.source)} · {time(b.updated)}{b.source==='stock'&&b.filters?.location_id?' · '+b.filters.location_id:''}
            </summary>
            {b.partial&&<p>{w.partial}</p>}
            {b.source==='stock'&&<p>{stockWords.help}</p>}
            {b.source==='finance'&&<p>{financeWords.financeHelp}</p>}
            <button disabled={b.detached} onClick={() => void fetchBlock(b)}>
              {w.refresh}
            </button>
            <label>
              {w.interval}
              <input
                type="number"
                min="0"
                max="1440"
                value={b.interval}
                onChange={(e) => {
                  b.interval = Math.max(0, Math.min(1440, Number(e.target.value)));
                  setBlockList([...blocks.current]);
                }}
              />
            </label>
            <button
              onClick={() => {
                b.detached = true;
                setBlockList([...blocks.current]);
                activity('unlink');
              }}
            >
              {w.unlink}
            </button>
            <button
              onClick={() => {
                setSource(b.source);
                setFilters(b.filters || { q: '', from: '', to: '' });
                setEditBlock(b);
                setPanel('data');
              }}
            >
              {w.origin}
            </button>
          </details>
        ))}
      </div>
      <div className="sheet70-source-errors">{blockList.filter(block=>blockErrors[block.id]).map(block=><p role="alert" key={block.id}>{label(block.source)}: {label(blockErrors[block.id])}</p>)}</div>
      {panel && (
        <dialog
          ref={dialog}
          className="sheet70-dialog"
          onCancel={() => setPanel('')}
          aria-modal="true"
          aria-label={label(panel)}
          onKeyDown={(e) => {
            if (e.key === 'Escape') setPanel('');
          }}
        >
          <header>
            <h2>{label(panel)}</h2>
            <button autoFocus onClick={() => setPanel('')}>
              {w.close}
            </button>
          </header>
          {error && <p role="alert">{label(error)}</p>}
          {panel === 'files' && (
            <>
              <input
                aria-label={w.search}
                placeholder={w.search}
                value={query}
                onChange={(e) => setQuery(e.target.value)}
              />
              {files
                .filter((f) => f.name.toLocaleLowerCase().includes(query.toLocaleLowerCase()))
                .map((f) => (
                  <article key={f.id}>
                    <button onClick={() => void loadFile(f)}>{f.name}</button>
                    <time>{time(f.updated)}</time>
                    <button
                      onClick={() => {
                        const name = window.prompt(w.rename, f.name);
                        if (name)
                          void sheetStorage()
                            .save(scope, { ...f, name }, f.revision)
                            .then(() => show('files'))
                            .catch(() => setError('saveError'));
                      }}
                    >
                      {w.rename}
                    </button>
                    <button
                      onClick={() =>
                        void sheetStorage()
                          .save(
                            scope,
                            {
                              ...f,
                              id: crypto.randomUUID(),
                              name: f.name + ' (2)',
                              revision: 0,
                              versions: [],
                            },
                            0
                          )
                          .then(() => show('files'))
                      }
                    >
                      {w.duplicate}
                    </button>
                    <button
                      onClick={() => {
                        if (window.confirm(w.deleteConfirm))
                          void sheetStorage()
                            .remove(scope, f.id)
                            .then(() => show('files'));
                      }}
                    >
                      {w.delete}
                    </button>
                  </article>
                ))}
              {!files.length && <p>{w.noFiles}</p>}
              <p>{w.local}</p>
            </>
          )}
          {panel === 'versions' && (
            <>
              <p>{w.versionHelp}</p>
              {doc.current?.versions.map((v) => (
                <article key={v.at}>
                  <time>{time(v.at)}</time>
                  <button
                    onClick={() => {
                      if (window.confirm(w.confirm)) {
                        blocks.current = v.blocks;
                        setBlockList(v.blocks);
                        props.load(v.workbook);
                        serial.current = '';
                        activity('restore');
                        setPanel('');
                      }
                    }}
                  >
                    {w.restore}
                  </button>
                </article>
              ))}
            </>
          )}
          {panel === 'templates' && (
            <>
              <p>{w.templateHelp}</p>
              <div className="sheet70-template-grid">
                {[
                  'catalog',
                  'stock',
                  'cashflow',
                  'pricing',
                  'schedule',
                  'closing',
                  'payable',
                  'receivable',
                ].map((k) => (
                  <button key={k} onClick={() => void template(k)}>
                    {label(k)}
                  </button>
                ))}
              </div>
            </>
          )}
          {panel === 'data' && (
            <>
              <p>{w.linkHelp}</p>
              <label>
                {w.source}
                <select
                  value={source}
                  onChange={(e) => {
                    controller.current?.abort();setSource(e.target.value);
                    setData(undefined);setLoading(false);
                  }}
                >
                  {Object.keys(sources).map((s) => (
                    <option key={s} value={s} disabled={!sourceAllowed(s, capabilities)}>
                      {label(s)}
                    </option>
                  ))}
                </select>
              </label>
              {source==='finance'&&<><p>{financeWords.financeHelp}</p><label>{financeWords.finance_as_of}<input type="date" value={filters.as_of||''} onChange={e=>{controller.current?.abort();setLoading(false);setData(undefined);setFilters({...filters,as_of:e.target.value});}}/></label></>}
              {source==='stock'&&<><p>{stockWords.help}</p><label>{stockWords.location}<select value={filters.location_id||''} onChange={e=>{controller.current?.abort();setLoading(false);setData(undefined);setFilters({...filters,location_id:e.target.value});}}><option value="">{stockWords.choose}</option>{stockLocations.map(item=><option key={item.id} value={item.id}>{item.name}</option>)}</select></label></>}
              <label>
                {w.search}
                <input
                  maxLength={240}
                  value={filters.q}
                  onChange={(e) => {controller.current?.abort();setLoading(false);setData(undefined);setFilters({ ...filters, q: e.target.value });}}
                />
              </label>
              <label>
                {w.from}
                <input
                  type="date"
                  disabled={!['sales', 'purchases','finance'].includes(source)}
                  value={filters.from}
                  onChange={(e) => {controller.current?.abort();setLoading(false);setData(undefined);setFilters({ ...filters, from: e.target.value });}}
                />
              </label>
              <label>
                {w.to}
                <input
                  type="date"
                  disabled={!['sales', 'purchases','finance'].includes(source)}
                  value={filters.to}
                  onChange={(e) => {controller.current?.abort();setLoading(false);setData(undefined);setFilters({ ...filters, to: e.target.value });}}
                />
              </label>
              <button disabled={loading||source==='finance'&&(!filters.from||!filters.to||!filters.as_of)||source==='stock'&&!stockLocations.some(item=>item.id===filters.location_id)} onClick={() => void fetchData()}>
                {w.preview}
              </button>
              {data && (
                <>
                  {data.mock && <strong>{w.mock}</strong>}
                  {data.partial && <p>{w.partial}</p>}
                  <fieldset>
                    <legend>{w.columns}</legend>
                    {data.columns.map((c) => (
                      <label key={c}>
                        <input
                          type="checkbox"
                          checked={columns.includes(c)}
                          onChange={(e) =>
                            setColumns(
                              e.target.checked ? [...columns, c] : columns.filter((x) => x !== c)
                            )
                          }
                        />
                        {label(c)}
                      </label>
                    ))}
                  </fieldset>
                  <div className="sheet70-table">
                    <table>
                      <thead>
                        <tr>
                          {data.columns.map((c) => (
                            <th key={c}>{label(c)}</th>
                          ))}
                        </tr>
                      </thead>
                      <tbody>
                        {data.rows.slice(0, 20).map((r, i) => (
                          <tr key={i}>
                            {r.map((v, j) => (
                              <td key={j}>
                                {typeof v === 'number'
                                  ? new Intl.NumberFormat(i18n.resolvedLanguage || 'pt-BR', {
                                      maximumFractionDigits: 2,
                                    }).format(v)
                                  : String(v)}
                              </td>
                            ))}
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                  <select
                    aria-label={w.selection}
                    value={target}
                    onChange={(e) => setTarget(e.target.value)}
                  >
                    <option value="selection">{w.selection}</option>
                    <option value="newSheet">{w.newSheet}</option>
                  </select>
                  <button disabled={!data.rows.length} onClick={insert}>
                    {w.confirm}
                  </button>
                </>
              )}
            </>
          )}
          {panel === 'send' && (
            <>
              <label>
                {w.selection}
                <select
                  value={sendScope}
                  onChange={(e) => {
                    setSendScope(e.target.value);
                    setPreviewRows(
                      e.target.value === 'selection'
                        ? selectionMatrix(props.get(), props.selection())
                        : usedMatrix(props.get(), props.selection().sheetId)
                    );
                  }}
                >
                  <option value="sheet">{w.wholeSheet}</option>
                  <option value="selection">{w.selection}</option>
                </select>
              </label>
              <label>
                {w.send}
                <select value={importKind} onChange={(e) => setImportKind(e.target.value)}>
                  {['prices', 'inventory', 'products', 'employees'].map((k) => (
                    <option key={k} value={k}>
                      {label(k)}
                    </option>
                  ))}
                </select>
              </label>
              <p>{w.mapping}</p>
              {(previewRows[0] || []).map((v, i) => (
                <label key={i}>
                  {String(v)}
                  <select
                    value={mapping[i] || ''}
                    onChange={(e) => {
                      const next = [...mapping];
                      next[i] = e.target.value;
                      setMapping(next);
                    }}
                  >
                    <option value="">—</option>
                    {['sku', 'name', 'price', 'quantity', 'role'].map((k) => (
                      <option key={k} value={k}>
                        {label(k)}
                      </option>
                    ))}
                  </select>
                </label>
              ))}
              <div className="sheet70-table">
                <table>
                  <thead>
                    <tr>
                      <th>{w.preview}</th>
                      <th>{w.status}</th>
                    </tr>
                  </thead>
                  <tbody>
                    {validateImport(previewRows, mapping, importKind)
                      .slice(0, 100)
                      .map((row) => (
                        <tr key={row.line}>
                          <td>{row.line}</td>
                          <td>{row.errors.map(label).join(', ') || '✓'}</td>
                        </tr>
                      ))}
                  </tbody>
                </table>
              </div>
              <p>{w.dataUnavailable}</p>
              <button disabled>
                {w.apply} · {w.coming}
              </button>
              <button
                onClick={() => {
                  const errors = validateImport(previewRows, mapping, importKind).filter(
                      (r) => r.errors.length
                    ),
                    url = URL.createObjectURL(
                      new Blob(
                        [
                          tableCSV([
                            [w.preview, w.status],
                            ...errors.map((r) => [r.line, r.errors.map(label).join(', ')]),
                          ]),
                        ],
                        { type: 'text/csv' }
                      )
                    ),
                    link = document.createElement('a');
                  link.href = url;
                  link.download = 'Titan-errors.csv';
                  link.click();
                  setTimeout(() => URL.revokeObjectURL(url), 1000);
                }}
              >
                {w.report}
              </button>
            </>
          )}
          {panel === 'copy' && (
            <>
              <p>{w.copyHelp}</p>
              <label>
                {w.row}
                <input
                  type="number"
                  min="1"
                  max="10000"
                  value={destinationRow}
                  onChange={(e) => setDestinationRow(Number(e.target.value))}
                />
              </label>
              <label>
                {w.column}
                <input
                  type="number"
                  min="1"
                  max="200"
                  value={destinationCol}
                  onChange={(e) => setDestinationCol(Number(e.target.value))}
                />
              </label>
              {files.map((file) => (
                <article key={file.id}>
                  <span>{file.name}</span>
                  <button
                    disabled={file.id === doc.current?.id}
                    onClick={() => void copySelection(file)}
                  >
                    {w.copy}
                  </button>
                </article>
              ))}
            </>
          )}
          {panel === 'share' && (
            <>
              <p>{w.sensitive}</p>
              <label>
                {w.person}
                <input />
              </label>
              <select aria-label={w.share}>
                <option>{w.read}</option>
                <option>{w.edit}</option>
              </select>
              <p>{w.shareUnavailable}</p>
              <button disabled>{w.share}</button>
              <button onClick={externalShare}>{w.shareFile}</button>
              <p>{w.attachHelp}</p>
              <a
                href={
                  'mailto:?subject=' +
                  encodeURIComponent(props.name) +
                  '&body=' +
                  encodeURIComponent(w.attachHelp)
                }
              >
                {w.email}
              </a>{' '}
              ·{' '}
              <a
                target="_blank"
                rel="noopener noreferrer"
                href={
                  'https://wa.me/?text=' + encodeURIComponent(props.name + ' — ' + w.attachHelp)
                }
              >
                WhatsApp
              </a>
            </>
          )}
          {(panel === 'find' || panel === 'replace') && (
            <>
              <label>
                {w.find}
                <input value={searchText} onChange={(e) => setSearchText(e.target.value)} />
              </label>
              {panel === 'replace' && (
                <label>
                  {w.replace}
                  <input value={replacement} onChange={(e) => setReplacement(e.target.value)} />
                </label>
              )}
              <button onClick={() => find(false)}>{w.find}</button>
              {panel === 'replace' && (
                <button
                  onClick={() => {
                    if (window.confirm(w.confirm)) find(true);
                  }}
                >
                  {w.replaceAll}
                </button>
              )}
            </>
          )}
          {panel === 'activity' && (
            <>
              {doc.current?.activity.map((a, i) => (
                <p key={i}>
                  {time(a.at)} · {label(a.action)}
                </p>
              ))}
            </>
          )}
        </dialog>
      )}
    </>
  );
}
