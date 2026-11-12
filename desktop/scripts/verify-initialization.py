#!/usr/bin/env python3
"""Opt-in Linux installed acceptance; fresh directories and synthetic data only.
Never use an existing client root. Never print captured credentials or key output.
"""
import argparse, hashlib, importlib.util, json, os, pathlib, pty, secrets, select, signal, socket, sqlite3, subprocess, tempfile, time, urllib.request, urllib.error
ROOT=pathlib.Path(__file__).resolve().parents[2]
spec=importlib.util.spec_from_file_location('titan_install',ROOT/'tools/titan_install.py')
installer=importlib.util.module_from_spec(spec);spec.loader.exec_module(installer)
p=argparse.ArgumentParser();p.add_argument('--binaries',required=True);p.add_argument('--desktop',required=True);p.add_argument('--evidence',required=True);a=p.parse_args()
report=pathlib.Path(a.evidence).absolute()
if report.exists():raise SystemExit('New evidence path required')
base=pathlib.Path(tempfile.mkdtemp(prefix='titan-initial-install-'));os.chmod(base,0o700)
bins=pathlib.Path(a.binaries).absolute();destination=base/'customer-selected-location';bundle=base/'bundle'
assert not destination.exists()
installer.pack(bins,bundle,'clean-c75b2bc',1);manager=installer.Manager(destination);manager.install(bundle)
assert not manager.status()['initialized']
command=['python3',str(ROOT/'tools/titan_install.py'),'run','--root',str(destination)]
def stop_visual():
 # Only the process group recorded by this invocation's own disposable driver.
 lease=base/'acceptance-process.json'
 if not lease.exists():return
 info=json.loads(lease.read_text());pid=info['pid']
 try:
  if pid<=1 or os.getpgid(pid)!=pid or pathlib.Path(os.readlink('/proc/'+str(pid)+'/exe'))!=pathlib.Path(a.desktop).absolute():return
  os.killpg(pid,signal.SIGTERM)
  for _ in range(20):
   try:os.kill(pid,0)
   except ProcessLookupError:return
   time.sleep(.05)
  os.killpg(pid,signal.SIGKILL)
 except (ProcessLookupError,FileNotFoundError):pass

def show_phases(raw):
 for line in raw.decode(errors='replace').splitlines():
  try:item=json.loads(line)
  except ValueError:continue
  if set(item)=={'acceptancePhase'} or set(item)=={'acceptanceFailure','detail'}:print(json.dumps(item),flush=True)

def run(args,data=None,ok=True):
 visual=pathlib.Path(str(args[0])).name=='xvfb-run'
 child=subprocess.Popen([str(x) for x in args],stdin=subprocess.PIPE,stdout=subprocess.PIPE,stderr=subprocess.PIPE,start_new_session=True)
 try:
  try:stdout,stderr=child.communicate(data,timeout=60)
  except subprocess.TimeoutExpired:
   if visual:stop_visual()
   os.killpg(child.pid,signal.SIGKILL);stdout,stderr=child.communicate(timeout=5)
   show_phases(stdout+stderr)
   raise RuntimeError('Disposable command timed out') from None
  if visual:show_phases(stdout+stderr)
  if (child.returncode==0)!=ok:raise RuntimeError('Disposable command failed: '+pathlib.Path(str(args[0])).name)
  if visual:
   results=[]
   for line in stdout.decode().splitlines():
    try:item=json.loads(line)
    except ValueError:continue
    if item.get('packagedApp') is True:results.append(item)
   assert len(results)==1
   return json.dumps(results[0]).encode()
  return stdout
 finally:
  if visual:stop_visual()
password=secrets.token_urlsafe(32)
out=run([*command,'titan-local','init','--empresa','Synthetic clean company','--loja','Synthetic clean store','--dono','Synthetic test owner'],(password+'\n').encode())
owner=next(line.split(': ',1)[1] for line in out.decode().splitlines() if line.startswith('Dono (ID para login): '));out=b''
run([*command,'titan-local','check'])
# Repeating init must refuse and must not create a second owner or overwrite keys.
run([*command,'titan-local','init','--empresa','Other','--loja','Other','--dono','Other'],(password+'\n').encode(),False)
private=destination/'private';vault=private/'fiscal-vault.sqlite';master=private/'fiscal-master.key';copy=base/'customer-key-backup';copy.mkdir(mode=0o700)
# The existing CLI writes the one-time key to its private terminal. Consume the
# disposable PTY in memory, discard immediately; never attach it to any log.
pid,fd=pty.fork()
if pid==0:os.execv(str(bins/'titan-fiscal'),[str(bins/'titan-fiscal'),'vault-init','--vault',str(vault),'--key-file',str(master)])
deadline=time.monotonic()+30;exit_status=None
while time.monotonic()<deadline:
 if select.select([fd],[],[],0.1)[0]:
  try:os.read(fd,8192)
  except OSError:pass
 done,status=os.waitpid(pid,os.WNOHANG)
 if done:exit_status=status;break
os.close(fd)
if exit_status is None:
 os.kill(pid,signal.SIGTERM);os.waitpid(pid,0);raise RuntimeError('Disposable vault command timeout')
assert os.waitstatus_to_exitcode(exit_status)==0
run([bins/'titan-fiscal','vault-export-key','--key-file',master,'--out',copy/'fiscal-master.key'])
assert master.read_bytes()==(copy/'fiscal-master.key').read_bytes()
for f in [master,copy/'fiscal-master.key',vault,manager.db,manager.station]:assert f.stat().st_mode&0o777==0o600
# Prove that a copied customer key decrypts the actual vault control, and that
# missing keys remain unavailable. The existing read-only diagnostic is reused.
backup_copy=copy/'backup.key'
with backup_copy.open('xb') as out:
 os.chmod(backup_copy,0o600);out.write(manager.key.read_bytes())
diag_args=[bins/'titan-diagnostics','--db',manager.db,'--vault',vault,'--key-file',master,'--key-copy',copy/'fiscal-master.key','--backup-dir',destination/'backups','--backup-key',manager.key,'--backup-key-copy',backup_copy,*manager.scope()]
def diagnostic(key_file):
 args=diag_args.copy();args[args.index('--key-file')+1]=key_file
 return json.loads(run(args))
assert diagnostic(master)['vault']=='available'
assert diagnostic(copy/'fiscal-master.key')['vault']=='available'
assert diagnostic(private/'missing.key')['vault']=='unavailable'
# New profile explicitly selects the customer's local destination; no external bind.
s=socket.socket();s.bind(('127.0.0.1',0));port=s.getsockname()[1];s.close()
profile=private/'deployment.json';profile.write_text(json.dumps(dict(version=1,mode='local',listen='127.0.0.1:'+str(port),tls_cert='',tls_key='',allowed_networks=[])));profile.chmod(0o600)
url='http://127.0.0.1:'+str(port)
def request(method,path,body=None,token=None):
 headers={'Content-Type':'application/json'}
 if token:headers['Authorization']='Bearer '+token
 req=urllib.request.Request(url+path,data=None if body is None else json.dumps(body).encode(),method=method,headers=headers)
 try:
  with urllib.request.urlopen(req,timeout=5) as r:
   raw=r.read();return r.status,json.loads(raw) if raw else None
 except urllib.error.HTTPError as e:
  raw=e.read()
  try:body=json.loads(raw) if raw else None
  except ValueError:body=None
  return e.code,body
def start():
 child=subprocess.Popen([*command,'titan-local','serve','--deployment',str(profile)],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL,start_new_session=True)
 for _ in range(100):
  if child.poll() is not None:raise RuntimeError('Discarded server exited')
  try:
   if request('GET','/local/v1/health')[0]==200:return child
  except (OSError,ValueError):pass
  time.sleep(.1)
 stop(child);raise RuntimeError('Discarded server unavailable')
def stop(child):
 if child.poll() is None:
  os.killpg(child.pid,signal.SIGTERM)
  try:child.wait(timeout=10)
  except subprocess.TimeoutExpired:os.killpg(child.pid,signal.SIGKILL);child.wait(timeout=5)
server=None
try:
 server=start();status,auth=request('POST','/local/v1/login',dict(identity_id=owner,password=password));assert status==200;token=auth['token']
 result=json.loads(run(['xvfb-run','-a','node',ROOT/'desktop/scripts/verify-initialization.cjs'],json.dumps(dict(binary=str(pathlib.Path(a.desktop).absolute()),root=str(base),url=url,owner=owner,password=password,token=token)).encode()))
 before_registration=request('GET','/local/v1/company/registration',token=token)[1]
 assert before_registration['revision']==2
 # A server restart alone does not promise revocation of durable sessions.
 # Explicit logout must remain revoked across the restart. Desktop credentials
 # remain in memory only, so a reopened application must show login again.
 assert request('POST','/local/v1/logout',{},token)[0]==204
 assert request('GET','/local/v1/me',token=token)[0]==401
 stop(server);server=None;server=start()
 assert request('GET','/local/v1/me',token=token)[0]==401
 status,auth=request('POST','/local/v1/login',dict(identity_id=owner,password=password));assert status==200
 status,record=request('GET','/local/v1/company/registration',token=auth['token']);assert status==200 and record==before_registration
 resumed=json.loads(run(['xvfb-run','-a','node',ROOT/'desktop/scripts/verify-initialization.cjs'],json.dumps(dict(binary=str(pathlib.Path(a.desktop).absolute()),root=str(base),url=url,owner=owner,password=password,token=auth['token'],resume=True)).encode()))
 assert resumed['reopened'] is True
 result['appRestartLoginAndProfile']=True
 assert diagnostic(copy/'fiscal-master.key')['vault']=='available'
 result['vaultReopenedAfterRestart']=True
 result['entireRegistrationUnchanged']=True
 result['desktopLaunches']=2
 with sqlite3.connect('file:'+str(manager.db)+'?mode=ro',uri=True) as db:
  assert db.execute("SELECT COUNT(*) FROM memberships WHERE role='owner'").fetchone()[0]==1
  schema=db.execute('SELECT MAX(version) FROM schema_migrations').fetchone()[0]
 result.update(backendSourceCommit='c75b2bc2d9a5424ee58f83f472c38be8fae74df7',sourceBaseCommit='bd158b67e5ca3839231cba543b02167acc95d871',desktopArchiveSHA256=hashlib.sha256((pathlib.Path(a.desktop).absolute().parent/'resources/app.asar').read_bytes()).hexdigest(),disposableRoot=str(base),linuxInstalledBackend=True,customerSelectedDataLocation=True,loopbackOnly=True,stationKeyCreated=True,vaultKeyCreated=True,privateKeyExportVerified=True,exportedKeyOpensVault=True,missingVaultKeyUnavailable=True,backupKeyCopy=True,privatePermissions=True,secondInitRefused=True,oneOwner=True,restartPreservedRegistration=True,explicitLogoutRemainsRevoked=True,schemaObserved=schema,cleanOS=False,windows=False,servicesStopped=True,productionEnabled=False,clientCredentialsRequired=False)
 stop(server);server=None
 report.parent.mkdir(parents=True,exist_ok=True)
 with report.open('x') as out:json.dump(result,out,indent=2)
 print(json.dumps(result))
finally:
 if server is not None:stop(server)
 password='';token='';auth={}
