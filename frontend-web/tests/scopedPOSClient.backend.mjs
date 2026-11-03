// Explicit opt-in. Only the Go fixture starts HTTP and SQLite, both disposable.
import {mkdtempSync,writeFileSync,existsSync} from 'node:fs';
import {join,resolve,dirname} from 'node:path';
import {tmpdir} from 'node:os';
import {fileURLToPath} from 'node:url';
import {spawnSync} from 'node:child_process';
const [backend,go]=process.argv.slice(2);
if(!backend||!go)throw Error('Usage: node tests/scopedPOSClient.backend.mjs /absolute/repository /absolute/go');
const root=resolve(backend),tool=resolve(go),here=dirname(fileURLToPath(import.meta.url));
if(!existsSync(join(root,'backend/internal/localapi/cash_bags_test.go'))||!existsSync(tool))throw Error('Backend fixture or Go not found');
const work=mkdtempSync(join(tmpdir(),'titan-cash-bags-overlay-')),overlay=join(work,'overlay.json');
writeFileSync(overlay,JSON.stringify({Replace:{[join(root,'backend/internal/localapi/scoped_pos_frontend_overlay_test.go')]:join(here,'fixtures/scoped_pos_frontend_test.go')}}));
const env={...process.env,GOTOOLCHAIN:'local',GOPROXY:'off',GOSUMDB:'off',TITAN_SCOPED_POS_SCRIPT:join(here,'scopedPOSClient.bridge.mjs')};
for(const key of ['TITAN_SESSION_TEST_DATABASE_URL','TITAN_CATALOG_TEST_DATABASE_URL','TITAN_CASH_JOURNAL_BRIDGE','TITAN_DELIVERY_BRIDGE','TITAN_MAINTENANCE_BRIDGE'])delete env[key];
const result=spawnSync(tool,['test','-overlay='+overlay,'-count=1','-json','-run','^TestScopedPOSFrontendBridgeIsolated$','./internal/localapi'],{cwd:join(root,'backend'),env,stdio:'inherit'});
if(result.error)throw result.error;process.exitCode=result.status??1;
