"""Run the existing online-session integration test in a fresh local database.

No application credentials, .env, historical migrations or existing databases
are used. PostgreSQL resources are preserved for inspection, never dropped.
"""
import argparse
from datetime import datetime, timedelta, timezone
import json
import os
from pathlib import Path
import secrets
import shutil
import subprocess
import tempfile

TEST = 'TestPostgresDurableRotationConcurrencyAndRollback'
PACKAGE = 'titansystem-backend/internal/onlinesessions'


class TestFailure(Exception):
    pass


def clean_environment(source):
    # Do not inherit application/test credentials or libpq connection overrides.
    return {k: v for k, v in source.items()
            if not k.startswith('PG') and not k.startswith('TITAN_')
            and k not in ('DATABASE_URL', 'DB_PASSWORD', 'DB_HOST', 'DB_NAME')}


def require_pass(output):
    if not isinstance(output, str) or len(output.encode()) > 2_000_000:
        raise TestFailure('Saída do teste inválida ou excessiva.')
    started = passed = package_passed = False
    for line in output.splitlines():
        try:
            event = json.loads(line)
        except (ValueError, TypeError):
            raise TestFailure('O teste não produziu o relatório JSON esperado.') from None
        if not isinstance(event, dict):
            raise TestFailure('Evento de teste inválido.')
        if event.get('Package') != PACKAGE:
            continue
        action, test = event.get('Action'), event.get('Test')
        if action in ('fail', 'skip'):
            raise TestFailure('Teste falhou ou foi ignorado; PostgreSQL não validado.')
        if test == TEST:
            started |= action == 'run'
            passed |= action == 'pass'
        if test is None and action == 'pass':
            package_passed = True
    if not (started and passed and package_passed):
        raise TestFailure('Não houve execução e aprovação do teste PostgreSQL esperado.')


def execute(repo, port, report, runner=subprocess.run, finder=shutil.which,
            environment=None):
    repo = Path(repo)
    if not isinstance(port, int) or isinstance(port, bool) or not 1 <= port <= 65535:
        raise TestFailure('Porta PostgreSQL inválida.')
    if not (repo / 'backend/internal/onlinesessions/postgres_test.go').is_file():
        raise TestFailure('Execute a ferramenta dentro do projeto atualizado.')
    binaries = {name: finder(name) for name in ('go', 'psql', 'sudo')}
    if not all(binaries.values()):
        raise TestFailure('Instale Go, o cliente PostgreSQL e sudo antes de executar.')
    env = clean_environment(os.environ if environment is None else environment)
    report.update(port=port, status='NÃO EXECUTADO', stage='preflight')

    def command(args, *, sql=None, timeout=30, command_env=None, interactive=False):
        try:
            result = runner(args, input=sql, text=True, errors='replace',
                            stdout=subprocess.PIPE,
                            stderr=None if interactive else subprocess.DEVNULL,
                            env=env if command_env is None else command_env,
                            cwd=repo / 'backend', timeout=timeout, check=False)
        except (OSError, subprocess.TimeoutExpired):
            raise TestFailure('Comando indisponível ou prazo excedido; consulte a etapa no relatório.') from None
        if result.returncode != 0:
            raise TestFailure('Comando falhou; consulte a etapa no relatório. Logs brutos foram omitidos.')
        return result.stdout

    # Download/check the selected toolchain before any PostgreSQL mutation.
    command([binaries['go'], 'version'], timeout=180)
    command([binaries['sudo'], '-v'], interactive=True)
    psql = [binaries['sudo'], '-n', '-u', 'postgres', binaries['psql'],
            '-X', '-q', '-w', '-h', '/var/run/postgresql', '-p', str(port),
            '-d', 'postgres', '-v', 'ON_ERROR_STOP=1']
    command(psql, sql='SELECT 1;')
    suffix = secrets.token_hex(12)
    role = 'titan_online_' + suffix
    database = role + '_test'
    password = secrets.token_hex(32)
    expires = (datetime.now(timezone.utc) + timedelta(hours=1)).strftime('%Y-%m-%d %H:%M:%S+00')
    report.update(role=role, database=database, role_created=False,
                  database_created=False, credentials_expire_at=expires)
    # All interpolated SQL identifiers/values are generated here from hex/dates.
    # Password travels via stdin, never argv, report or terminal output.
    report['stage'] = 'create_role'
    command(psql, sql=(f"CREATE ROLE {role} LOGIN NOSUPERUSER NOCREATEDB "
                      f"NOCREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT "
                      f"CONNECTION LIMIT 12 PASSWORD '{password}' VALID UNTIL '{expires}';"))
    report['role_created'] = True
    report['stage'] = 'create_database'
    command(psql, sql=f'CREATE DATABASE {database} OWNER {role} TEMPLATE template0;')
    report['database_created'] = True
    report['stage'] = 'postgres_test'
    test_env = dict(env)
    test_env['TITAN_SESSION_TEST_DATABASE_URL'] = (
        f'postgres://{role}:{password}@127.0.0.1:{port}/{database}?sslmode=disable')
    output = command([binaries['go'], 'test', '-json', '-count=1',
                      './internal/onlinesessions', '-run', '^' + TEST + '$'],
                     command_env=test_env, timeout=240)
    require_pass(output)
    report.update(status='PASSOU', stage='completed')


def save_report(report, parent=None):
    directory = Path(tempfile.mkdtemp(prefix='titansystem-postgres-test-', dir=parent))
    directory.chmod(0o700)
    path = directory / 'result.json'
    descriptor = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    with os.fdopen(descriptor, 'w', encoding='utf-8') as file:
        json.dump(report, file, ensure_ascii=False, indent=2)
        file.write('\n')
    return path


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--pg-port', type=int, default=5432)
    args = parser.parse_args()
    report = {'status': 'NÃO EXECUTADO', 'stage': 'preflight'}
    error = None
    try:
        if os.geteuid() == 0:
            raise TestFailure('Execute como seu usuário normal, sem sudo no Python.')
        execute(Path(__file__).resolve().parents[1], args.pg_port, report)
    except TestFailure as failure:
        error = str(failure)
        report['status'] = 'FALHOU / NÃO VALIDADO'
    except KeyboardInterrupt:
        error = 'Execução interrompida; recursos eventualmente criados foram preservados.'
        report['status'] = 'INTERROMPIDO / NÃO VALIDADO'
    try:
        path = save_report(report)
    except OSError:
        print('Não foi possível salvar o relatório. Recursos de teste não foram removidos.')
        return 1
    if error:
        print('Pare: ' + error)
    else:
        print('PASSOU: migrações, concorrência, revogação e rollback em PostgreSQL real.')
    print('Relatório privado:', path)
    if 'role' in report:
        print('Recursos de teste eventualmente criados foram preservados; senha temporária válida por uma hora.')
    return 1 if error else 0


if __name__ == '__main__':
    raise SystemExit(main())
