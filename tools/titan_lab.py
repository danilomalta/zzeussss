"""Owned loopback laboratory. No system database, production worktree or remote Docker."""
import argparse
import json
import os
from pathlib import Path
import re
import secrets
import shutil
import socket
import stat
import subprocess
import time
import urllib.request

PROJECT = 'titansystem-lab52'
VOLUME = 'titansystem_lab52_pgdata'
ROOT = Path(__file__).resolve().parents[1]
STATE = Path.home() / '.local/share/titansystem/lab52'
KEYS = ('LAB_INSTANCE', 'LAB_ADMIN_PASSWORD', 'LAB_DB_PASSWORD', 'LAB_JWT_SECRET')

class LabError(Exception):
    pass

def run(args, timeout=60, allow_failure=False):
    env = {k: os.environ[k] for k in ('PATH', 'HOME', 'USER', 'XDG_RUNTIME_DIR') if k in os.environ}
    try:
        p = subprocess.run(args, env=env, stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                           text=True, timeout=timeout)
    except (OSError, subprocess.TimeoutExpired):
        raise LabError('Comando indisponível ou prazo excedido; dados preservados.') from None
    if len(p.stdout) + len(p.stderr) > 8_000_000:
        raise LabError('Saída excessiva; diagnóstico bruto não exibido.')
    if p.returncode and not allow_failure:
        raise LabError('Etapa falhou; execute diagnose. Logs brutos não exibidos.')
    return p

def private_path(path, directory=False):
    st = path.lstat()
    if path.is_symlink() or st.st_uid != os.getuid() or stat.S_IMODE(st.st_mode) & 0o077:
        raise LabError('Estado privado com proprietário/permissão inválidos.')
    if directory != stat.S_ISDIR(st.st_mode) or (not directory and not stat.S_ISREG(st.st_mode)):
        raise LabError('Tipo de arquivo privado inválido.')

def initialize():
    if STATE.exists() or STATE.is_symlink():
        credentials()
        print('Estado privado existente preservado.')
        return
    # Reject symlink ancestors; never replace an existing state or follow a redirection.
    for p in STATE.parents:
        if p.is_symlink():
            raise LabError('Diretório de dados não pode atravessar symlink.')
    STATE.mkdir(parents=True, mode=0o700)
    values = {k: secrets.token_hex(32) for k in KEYS}
    fd = os.open(STATE / 'credentials.env', os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    with os.fdopen(fd, 'w') as f:
        f.write(''.join(k + '=' + v + '\n' for k, v in values.items()))
    fd = os.open(STATE / 'owner.json', os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    with os.fdopen(fd, 'w') as f:
        json.dump({'project': PROJECT, 'root': str(ROOT), 'instance': values['LAB_INSTANCE']}, f)
    print('Credenciais aleatórias criadas em diretório privado; nenhuma senha exibida.')

def credentials():
    try:
        private_path(STATE, True)
        for name in ('credentials.env', 'owner.json'):
            private_path(STATE / name)
        lines = (STATE / 'credentials.env').read_text().splitlines()
        values = dict(line.split('=', 1) for line in lines)
        if len(lines) != len(KEYS) or set(values) != set(KEYS) or any(not re.fullmatch('[0-9a-f]{64}', v) for v in values.values()):
            raise ValueError()
        owner = json.loads((STATE / 'owner.json').read_text())
        if owner != {'project': PROJECT, 'root': str(ROOT), 'instance': values['LAB_INSTANCE']}:
            raise ValueError()
        return values
    except (OSError, ValueError, TypeError):
        raise LabError('Estado ausente/incompleto. Não sobrescreva; use init ou preserve para diagnóstico.') from None

def docker():
    if shutil.which('docker') is None:
        raise LabError('Docker não encontrado. Instale Docker Engine e o plugin Compose antes de start.')
    try:
        info = json.loads(run(['docker', '--context', 'default', 'context', 'inspect', 'default']).stdout)
        host = info[0]['Endpoints']['docker']['Host']
        if not host.startswith('unix://'):
            raise ValueError()
    except (ValueError, KeyError, IndexError, TypeError):
        raise LabError('Somente Docker local por socket Unix é permitido.') from None
    if run(['docker', '--context', 'default', 'info', '--format', '{{.OSType}}']).stdout.strip() != 'linux':
        raise LabError('É necessário Docker Engine local com containers Linux.')
    run(['docker', '--context', 'default', 'compose', 'version'])

def compose(*args, timeout=60, allow_failure=False):
    return run(['docker', '--context', 'default', 'compose', '--project-name', PROJECT,
                '--project-directory', str(ROOT / 'deploy/lab52'), '--env-file', str(STATE / 'credentials.env'),
                '-f', str(ROOT / 'deploy/lab52/compose.yaml'), *args], timeout, allow_failure)

def ownership():
    values = credentials()
    present = run(['docker', '--context', 'default', 'volume', 'ls', '--format', '{{.Name}}']).stdout.splitlines()
    for kind, name in [('volume', VOLUME)]:
        if name in present:
            p = run(['docker', '--context', 'default', kind, 'inspect', name])
            try:
                if json.loads(p.stdout)[0]['Labels']['com.titansystem.lab-instance'] != values['LAB_INSTANCE']:
                    raise ValueError()
            except (ValueError, KeyError, IndexError, TypeError):
                raise LabError('Volume existente não pertence a este laboratório. Nada alterado.') from None
    p = run(['docker', '--context', 'default', 'ps', '-a', '--filter', 'label=com.docker.compose.project='+PROJECT, '--format', '{{.ID}}'])
    for cid in p.stdout.splitlines():
        if not re.fullmatch('[0-9a-f]{12,64}', cid):
            raise LabError('Inventário Docker inválido.')
        result = run(['docker', '--context', 'default', 'inspect', '--format', '{{json .Config.Labels}}', cid])
        if json.loads(result.stdout).get('com.titansystem.lab-instance') != values['LAB_INSTANCE']:
            raise LabError('Container existente não pertence a este laboratório. Nada alterado.')

def wait_health(service, deadline=120):
    end = time.monotonic() + deadline
    while time.monotonic() < end:
        p = compose('ps', '-q', service)
        cid = p.stdout.strip()
        if cid and re.fullmatch('[0-9a-f]{12,64}', cid):
            status = run(['docker', '--context', 'default', 'inspect', '--format', '{{.State.Health.Status}}', cid]).stdout.strip()
            if status == 'healthy':
                return
            if status == 'unhealthy':
                break
        time.sleep(2)
    raise LabError('Saúde de '+service+' não confirmada; recursos preservados.')

def start():
    credentials(); docker(); ownership(); compose('config', '--quiet')
    # Refuse a conflicting host port before creating or restarting API containers.
    cid = compose('ps', '-q', 'postgres').stdout.strip()
    if not cid:
        with socket.socket() as s:
            try:
                s.bind(('127.0.0.1', 18080))
            except OSError:
                raise LabError('Porta 18080 ocupada. Nenhum container iniciado.') from None
    print('Compilando a API completa e a ferramenta de migração...', flush=True)
    compose('build', 'api', 'migrate', timeout=900)
    compose('up', '-d', 'postgres'); wait_health('postgres')
    print('Aplicando e verificando somente as migrações do laboratório...', flush=True)
    compose('run', '--no-deps', '-T', 'migrate', timeout=180)
    compose('up', '-d', '--no-deps', 'api'); wait_health('api')
    check()

def check():
    credentials(); docker(); ownership()
    wait_health('postgres', 15); wait_health('api', 15)
    compose('run', '--no-deps', '-T', 'migrate', 'check', timeout=180)
    query = "SELECT tablename FROM pg_tables WHERE schemaname='public' ORDER BY tablename"
    p = compose('exec', '-T', 'postgres', 'psql', '-U', 'titan_app', '-d', 'titan_lab52', '-At', '-c', query)
    tables = p.stdout.splitlines()
    required = {'tenants','users','products','discount_suggestions','online_sessions','online_purchase_documents','online_purchase_document_operations','online_purchase_document_outbox'}
    if not required.issubset(tables) or any(not re.fullmatch('[a-z_][a-z_0-9]*', n) for n in tables):
        raise LabError('Inventário de tabelas incompleto ou inválido.')
    try:
        # Do not inherit HTTP proxies for this strictly local check.
        client = urllib.request.build_opener(urllib.request.ProxyHandler({}))
        with client.open('http://127.0.0.1:18080/api/v1/saude', timeout=5) as response:
            if response.status != 200 or json.loads(response.read(4096)) != {'status':'ativo'}:
                raise ValueError()
    except (OSError, ValueError):
        raise LabError('API HTTP local não validada.') from None
    print('PASSOU: PostgreSQL saudável, migrações/checksums e API HTTP reais.')
    print('Tabelas presentes: ' + ', '.join(tables))
    print('API: http://127.0.0.1:18080/api/v1/saude')

def diagnose():
    credentials(); docker(); ownership()
    # State metadata only: never compose config, inspect env or application raw logs.
    p = compose('ps', '-a', '--format', 'json')
    try:
        rows = json.loads(p.stdout) if p.stdout.strip().startswith('[') else [json.loads(v) for v in p.stdout.splitlines()]
        for row in rows:
            service = row.get('Service', '')
            if service in ('postgres','migrate','api'):
                print(json.dumps({k: row.get(k) for k in ('Service','State','Health','ExitCode')}, ensure_ascii=False))
    except (ValueError, TypeError, AttributeError):
        raise LabError('Metadados Docker indisponíveis; nenhuma credencial exibida.') from None

def stop():
    credentials(); docker(); ownership(); compose('stop', timeout=90)
    print('Containers parados. Volume PostgreSQL e credenciais preservados.')

def main():
    parser = argparse.ArgumentParser(description='Laboratório Titan isolado; não é instalação comercial ou sincronização.')
    parser.add_argument('action', choices=['init','start','check','diagnose','stop'])
    action = parser.parse_args().action
    try:
        {'init':initialize,'start':start,'check':check,'diagnose':diagnose,'stop':stop}[action]()
        return 0
    except LabError as e:
        print('Pare: '+str(e))
        return 1
    except (OSError, ValueError, TypeError):
        print('Pare: estado ou metadados indisponíveis. Arquivos e dados preservados.')
        return 1
if __name__ == '__main__':
    raise SystemExit(main())
