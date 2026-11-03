// An operator-selected local snapshot is informational, never server authority.
// Strict allow-list: no records, IDs, paths, credentials or arbitrary messages.
export function parseDiagnostic(text, now = Date.now()/1000) {
 if(typeof text!=='string'||text.length>4096) throw new Error('invalid');
 const r=JSON.parse(text),keys=['version','build','schema_version','observed_unix','backup_unix','backup_interval_seconds','vault','key_exported','tls'];
 if(!r||Array.isArray(r)||Object.keys(r).length!==keys.length||keys.some(k=>!Object.hasOwn(r,k))||Object.keys(r).some(k=>!keys.includes(k)))throw new Error('invalid');
 if(r.version!==1||typeof r.build!=='string'||!/^[\w.+-]{1,80}$/.test(r.build)||!Number.isSafeInteger(r.schema_version)||r.schema_version<1||!Number.isSafeInteger(r.observed_unix)||r.observed_unix<1||r.observed_unix>now+60||!Number.isSafeInteger(r.backup_unix)||r.backup_unix<0||r.backup_unix>r.observed_unix||!Number.isSafeInteger(r.backup_interval_seconds)||r.backup_interval_seconds<60||r.backup_interval_seconds>86400||!['available','unavailable'].includes(r.vault)||typeof r.key_exported!=='boolean'||!['loopback','direct_tls','proxy_declared','invalid'].includes(r.tls))throw new Error('invalid');
 return Object.freeze({...r});
}
export function diagnosticWarnings(r, now = Date.now()/1000) {
 if(!r)return ['unknown'];
 const result=[];
 if(now-r.observed_unix>r.backup_interval_seconds)result.push('staleReport');
 if(!r.backup_unix||now-r.backup_unix>r.backup_interval_seconds)result.push('oldBackup');
 if(!r.key_exported)result.push('noKeyCopy');
 if(r.vault!=='available')result.push('vaultUnavailable');
 if(r.tls==='invalid')result.push('tlsInvalid');
 return result;
}
