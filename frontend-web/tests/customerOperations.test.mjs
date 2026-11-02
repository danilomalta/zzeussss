import test from 'node:test';import assert from 'node:assert/strict';import fs from 'node:fs';
import {parseDiagnostic,diagnosticWarnings} from '../src/core/customerOperations/diagnostic.mjs';
const now=1800000000, report={version:1,build:'test',schema_version:1,observed_unix:now,backup_unix:now,backup_interval_seconds:3600,vault:'available',key_exported:true,tls:'loopback'};
test('local diagnostic validates its allow-list and refuses secrets and future times',()=>{
 assert.deepEqual(parseDiagnostic(JSON.stringify(report),now),report);
 for(const r of [{...report,token:'secret'},{...report,build:'<script>'},{...report,observed_unix:now+70},{...report,backup_unix:now+1},{...report,key_exported:'yes'},{...report,vault:'anything'},{...report,backup_interval_seconds:0}])assert.throws(()=>parseDiagnostic(JSON.stringify(r),now));
 assert.throws(()=>parseDiagnostic('x'.repeat(4097),now));
});
test('local diagnostic warns on unknown, old backup, old report and unexported key without presenting zero as success',()=>{
 assert.deepEqual(diagnosticWarnings(null,now),['unknown']);assert.deepEqual(diagnosticWarnings(report,now),[]);
 assert.deepEqual(diagnosticWarnings({...report,backup_unix:0,key_exported:false,vault:'unavailable',tls:'invalid'},now+3601),['staleReport','oldBackup','noKeyCopy','vaultUnavailable','tlsInvalid']);
});
test('local diagnostics have five complete dictionaries and no transmission or persistence',()=>{
 const dir=new URL('../src/core/customerOperations/',import.meta.url),pt=JSON.parse(fs.readFileSync(new URL('pt-BR.json',dir)));
 for(const lang of ['en','es','fr','zh-CN']){const words=JSON.parse(fs.readFileSync(new URL(lang+'.json',dir)));assert.deepEqual(Object.keys(words).sort(),Object.keys(pt).sort());for(const v of Object.values(words))assert.ok(v.length>0);}
 const source=fs.readFileSync(new URL('../src/modules/tenant/pages/CustomerOperations.tsx',import.meta.url),'utf8');
 assert.doesNotMatch(source,/localStorage|sessionStorage|fetch\(|XMLHttpRequest|sendBeacon|console\./);assert.match(source,/generation.current/);assert.match(source,/\[sessionKey\]/);
});
