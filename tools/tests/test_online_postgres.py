import importlib.util
import json
from pathlib import Path
import subprocess
import tempfile
import unittest

spec = importlib.util.spec_from_file_location(
    'test_online_postgres', Path(__file__).resolve().parents[1] / 'test_online_postgres.py')
pg = importlib.util.module_from_spec(spec)
spec.loader.exec_module(pg)


def events(action='pass'):
    return '\n'.join(json.dumps(row) for row in (
        {'Package': pg.PACKAGE, 'Action': 'run', 'Test': pg.TEST},
        {'Package': pg.PACKAGE, 'Action': action, 'Test': pg.TEST},
        {'Package': pg.PACKAGE, 'Action': 'pass'}))


class PostgresRunnerTests(unittest.TestCase):
    def fixture(self, root):
        path = Path(root) / 'backend/internal/onlinesessions/postgres_test.go'
        path.parent.mkdir(parents=True)
        path.write_text('fixture')
        return Path(root)

    def test_requires_actual_named_test_and_package_pass(self):
        pg.require_pass(events())
        for bad in (events('skip'), events('fail'), '', '{}', 'raw password',
                    events().replace(pg.TEST, 'DifferentTest'),
                    events().replace(pg.PACKAGE, 'different/package'), '[]',
                    '\n'.join(events().splitlines()[:-1])):
            with self.subTest(bad=bad), self.assertRaises(pg.TestFailure):
                pg.require_pass(bad)

    def test_excessive_output_is_rejected(self):
        with self.assertRaises(pg.TestFailure):
            pg.require_pass('x' * 2_000_001)

    def test_credentials_and_connection_overrides_are_not_inherited(self):
        cleaned = pg.clean_environment({'PATH': '/bin', 'GOTOOLCHAIN': 'go1.25.0',
                                       'PGPASSWORD': 'secret', 'PGSERVICE': 'foreign',
                                       'PGOPTIONS': 'foreign', 'DATABASE_URL': 'foreign',
                                       'TITAN_SESSION_TEST_DATABASE_URL': 'foreign'})
        self.assertEqual(cleaned, {'PATH': '/bin', 'GOTOOLCHAIN': 'go1.25.0'})

    def test_missing_tools_or_invalid_port_do_not_start_processes(self):
        with tempfile.TemporaryDirectory() as root:
            repo = self.fixture(root)
            def forbidden(*args, **kwargs):
                self.fail('Preflight must not launch anything')
            with self.assertRaises(pg.TestFailure):
                pg.execute(repo, 5432, {}, runner=forbidden, finder=lambda _: None)
            for port in (0, 65536, -1, True, '5432'):
                with self.assertRaises(pg.TestFailure):
                    pg.execute(repo, port, {}, runner=forbidden)

    def run_fixture(self, root, *, fail_stage=None, output=None):
        calls, report = [], {}
        def fake(args, **kwargs):
            calls.append((args, kwargs))
            number = len(calls)
            if fail_stage == number:
                return subprocess.CompletedProcess(args, 1, 'secret raw failure')
            return subprocess.CompletedProcess(args, 0, events() if output is None else output)
        pg.execute(self.fixture(root), 5432, report, runner=fake,
                   finder=lambda name: '/bin/' + name,
                   environment={'PATH': '/bin', 'PGPASSWORD': 'real-secret',
                                'TITAN_SESSION_TEST_DATABASE_URL': 'real-url'})
        return calls, report

    def test_success_uses_new_restricted_database_and_password_only_in_stdin_env(self):
        with tempfile.TemporaryDirectory() as root:
            calls, report = self.run_fixture(root)
            self.assertEqual(report['status'], 'PASSOU')
            self.assertEqual(len(calls), 6)
            role_sql = calls[3][1]['input']
            database_sql = calls[4][1]['input']
            self.assertRegex(report['role'], r'^titan_online_[a-f0-9]{24}$')
            self.assertEqual(report['database'], report['role'] + '_test')
            for restriction in ('NOSUPERUSER', 'NOCREATEDB', 'NOCREATEROLE',
                                'NOREPLICATION', 'NOBYPASSRLS', 'NOINHERIT', 'VALID UNTIL'):
                self.assertIn(restriction, role_sql)
            self.assertEqual(database_sql, f"CREATE DATABASE {report['database']} OWNER {report['role']} TEMPLATE template0;")
            password = role_sql.split("PASSWORD '")[1].split("'")[0]
            self.assertNotIn(password, json.dumps(report))
            for args, kwargs in calls:
                self.assertNotIn(password, ' '.join(args))
                self.assertNotIn('shell', kwargs)
                self.assertNotIn('real-secret', str(kwargs))
                self.assertNotIn('real-url', str(kwargs))
                self.assertNotIn('DROP', kwargs['input'] or '')
            test_call = calls[-1]
            self.assertIn('-json', test_call[0])
            self.assertIn('-count=1', test_call[0])
            self.assertIn('127.0.0.1:5432/' + report['database'],
                          test_call[1]['env']['TITAN_SESSION_TEST_DATABASE_URL'])
            self.assertIs(calls[0][1]['stderr'], subprocess.DEVNULL)

    def test_command_failure_stops_before_later_mutations_without_raw_error(self):
        for stage in range(1, 7):
            with tempfile.TemporaryDirectory() as root:
                with self.assertRaises(pg.TestFailure) as error:
                    self.run_fixture(root, fail_stage=stage)
                self.assertNotIn('secret', str(error.exception))

    def test_skip_cannot_claim_success_even_with_exit_zero(self):
        with tempfile.TemporaryDirectory() as root:
            with self.assertRaises(pg.TestFailure):
                self.run_fixture(root, output=events('skip'))

    def test_database_creation_failure_preserves_role_and_stops(self):
        with tempfile.TemporaryDirectory() as root:
            calls, report = [], {}
            def fake(args, **kwargs):
                calls.append((args, kwargs))
                return subprocess.CompletedProcess(args, 1 if len(calls) == 5 else 0, '')
            with self.assertRaises(pg.TestFailure):
                pg.execute(self.fixture(root), 5432, report, runner=fake,
                           finder=lambda name: '/bin/' + name, environment={})
            self.assertEqual(len(calls), 5)
            self.assertTrue(report['role_created'])
            self.assertFalse(report['database_created'])
            self.assertEqual(report['stage'], 'create_database')
            self.assertEqual(report['status'], 'NÃO EXECUTADO')
            self.assertNotIn('TITAN_SESSION_TEST_DATABASE_URL', calls[-1][1]['env'])
            self.assertIn('/var/run/postgresql', calls[-1][0])

    def test_timeout_preserves_failure_and_omits_process_details(self):
        with tempfile.TemporaryDirectory() as root:
            repo = self.fixture(root)
            def timeout(args, **kwargs):
                raise subprocess.TimeoutExpired(args, 1, output='secret')
            with self.assertRaises(pg.TestFailure) as error:
                pg.execute(repo, 5432, {}, runner=timeout, finder=lambda _: '/bin/tool')
            self.assertNotIn('secret', str(error.exception))

    def test_report_has_private_permissions_and_contains_only_metadata(self):
        with tempfile.TemporaryDirectory() as root:
            report = {'status': 'NÃO EXECUTADO', 'stage': 'preflight'}
            path = pg.save_report(report, root)
            self.assertEqual(path.stat().st_mode & 0o777, 0o600)
            self.assertEqual(path.parent.stat().st_mode & 0o777, 0o700)
            self.assertEqual(json.loads(path.read_text()), report)


if __name__ == '__main__':
    unittest.main()
