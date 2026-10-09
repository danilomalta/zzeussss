import importlib.util
import json
from pathlib import Path
import subprocess
import tempfile
import unittest

spec = importlib.util.spec_from_file_location('catalog_pg_runner', Path(__file__).resolve().parents[1] / 'test_online_postgres.py')
pg = importlib.util.module_from_spec(spec)
spec.loader.exec_module(pg)
TEST, PACKAGE, PATH, SOURCE, VARIABLE = pg.SUITES['catalog']


def events(action='pass'):
    return '\n'.join(json.dumps(event) for event in (
        {'Package': PACKAGE, 'Action': 'run', 'Test': TEST},
        {'Package': PACKAGE, 'Action': action, 'Test': TEST},
        {'Package': PACKAGE, 'Action': 'pass'}))


class CatalogPostgresRunnerTests(unittest.TestCase):
    def test_catalog_suite_cannot_accept_session_or_skipped_results(self):
        pg.require_pass(events(), TEST, PACKAGE)
        for output in (events('skip'), events('fail'), events().replace(TEST, pg.TEST), events().replace(PACKAGE, pg.PACKAGE)):
            with self.assertRaises(pg.TestFailure):
                pg.require_pass(output, TEST, PACKAGE)

    def test_catalog_uses_new_database_and_only_selected_url(self):
        with tempfile.TemporaryDirectory() as tmp:
            repo = Path(tmp)
            fixture = repo / SOURCE
            fixture.parent.mkdir(parents=True)
            fixture.write_text('fixture')
            calls, report = [], {}

            def run(args, **kwargs):
                calls.append((args, kwargs))
                return subprocess.CompletedProcess(args, 0, events())

            pg.execute(repo, 5432, report, runner=run, finder=lambda name: '/bin/'+name,
                       environment={'PATH': '/bin', 'TITAN_SESSION_TEST_DATABASE_URL': 'secret-old-url'}, suite='catalog')
            self.assertEqual(report['status'], 'PASSOU')
            self.assertEqual(report['suite'], 'catalog')
            args, kwargs = calls[-1]
            self.assertIn(PATH, args)
            self.assertIn('^'+TEST+'$', args)
            self.assertNotIn('TITAN_SESSION_TEST_DATABASE_URL', kwargs['env'])
            self.assertIn(VARIABLE, kwargs['env'])
            self.assertIn(report['database'], kwargs['env'][VARIABLE])

    def test_unknown_suite_stops_before_any_process(self):
        def forbidden(*args, **kwargs):
            self.fail('Unexpected command')
        with self.assertRaises(pg.TestFailure):
            pg.execute(Path('/unused'), 5432, {}, runner=forbidden, suite='foreign')

    def test_failure_records_only_source_line_without_raw_error(self):
        with tempfile.TemporaryDirectory() as tmp:
            repo = Path(tmp)
            fixture = repo / SOURCE
            fixture.parent.mkdir(parents=True)
            fixture.write_text('fixture')
            report = {}

            def run(args, **kwargs):
                if '-json' in args:
                    output = json.dumps({'Package': PACKAGE, 'Action': 'output', 'Output': 'catalog_postgres_test.go:73: password=PRIVATE SQL=SECRET'})
                    return subprocess.CompletedProcess(args, 1, output)
                return subprocess.CompletedProcess(args, 0, '')

            with self.assertRaises(pg.TestFailure):
                pg.execute(repo, 5432, report, runner=run, finder=lambda name: '/bin/'+name, environment={}, suite='catalog')
            self.assertEqual(report['failure_location'], 'catalog_postgres_test.go:73')
            self.assertNotIn('PRIVATE', json.dumps(report))
            self.assertNotIn('SECRET', json.dumps(report))


if __name__ == '__main__':
    unittest.main()
