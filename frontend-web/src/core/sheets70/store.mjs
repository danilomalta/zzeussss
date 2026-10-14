const DB = 'titan-sheets70';
function valid(document) {
  if (
    !document ||
    typeof document.id !== 'string' ||
    document.id.length > 128 ||
    typeof document.name !== 'string' ||
    document.name.length > 120 ||
    !document.workbook ||
    !Array.isArray(document.workbook.sheetOrder) ||
    document.workbook.sheetOrder.length > 12 ||
    !Array.isArray(document.blocks) ||
    document.blocks.length > 100 ||
    document.blocks.some(
      (b) =>
        !b ||
        !['row', 'col', 'height', 'width', 'interval'].every(
          (k) => Number.isInteger(b[k]) && b[k] >= 0
        ) ||
        b.row + b.height > 10000 ||
        b.col + b.width > 200
    ) ||
    JSON.stringify(document).length > 12000000
  )
    throw Error('invalid');
  return document;
}

let connection;
async function open() {
  if (!connection)
    connection = new Promise((resolve, reject) => {
      const request = indexedDB.open(DB, 1);
      request.onupgradeneeded = () =>
        request.result.createObjectStore('documents', { keyPath: 'key' });
      request.onsuccess = () => resolve(request.result);
      request.onerror = () => reject(request.error);
    });
  return connection;
}
export async function listDocuments(scope) {
  const db = await open();
  return new Promise((resolve, reject) => {
    const tx = db.transaction('documents', 'readonly'),
      request = tx.objectStore('documents').getAll();
    request.onsuccess = () =>
      resolve(
        request.result
          .filter((x) => x && x.scope === scope)
          .map(valid)
          .sort((a, b) => b.updated - a.updated)
      );
    request.onerror = () => reject(request.error);
  });
}
export async function saveDocument(scope, document, expectedRevision) {
  valid({ ...document, versions: [] });
  const db = await open();
  return new Promise((resolve, reject) => {
    const tx = db.transaction('documents', 'readwrite'),
      store = tx.objectStore('documents'),
      key = scope + ':' + document.id,
      request = store.get(key);
    let saved;
    request.onsuccess = () => {
      const old = request.result;
      if ((old?.revision || 0) !== expectedRevision) {
        tx.abort();
        reject(Error('conflict'));
        return;
      }
      const now = Date.now(),
        versionDue = !old || now - old.versionAt >= 300000;
      const versions = versionDue
        ? [
            ...(old?.versions || []),
            {
              at: now,
              workbook: structuredClone(document.workbook),
              blocks: structuredClone(document.blocks || []),
            },
          ].slice(-20)
        : old.versions;
      saved = {
        ...document,
        key,
        scope,
        revision: expectedRevision + 1,
        updated: now,
        versionAt: versionDue ? now : old.versionAt,
        versions,
      };
      while (JSON.stringify(saved).length > 12000000 && saved.versions.length > 1)
        saved.versions.shift();
      valid(saved);
      store.put(saved);
    };
    tx.oncomplete = () => resolve(saved);
    tx.onerror = () => reject(tx.error);
    tx.onabort = () => reject(tx.error || Error('conflict'));
  });
}
export async function deleteDocument(scope, id) {
  const db = await open();
  return new Promise((resolve, reject) => {
    const tx = db.transaction('documents', 'readwrite');
    tx.objectStore('documents').delete(scope + ':' + id);
    tx.oncomplete = () => resolve();
    tx.onerror = () => reject(tx.error);
  });
}
