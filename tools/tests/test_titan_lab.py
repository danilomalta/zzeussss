import contextlib
import importlib.util
import io
import json
import os
from pathlib import Path
import stat
import subprocess
import tempfile
import unittest
from unittest.mock import patch, MagicMock

spec = importlib.util.spec_from_file_location('titan_lab', Path(__file__).resolve().parents[1] / 'titan_lab.py')
lab = importlib.util.module_from_spec(spec)
spec.loader.exec_module(lab)

def result(output='', code=0):
    return subprocess.CompletedProcess([], code, output, 'SECRET_RAW_ERROR')

class LabTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.state_patch = patch.object(lab, 'STATE', Path(self.tmp.name) / 'lab')
        self.state_patch.start()
        with contextlib.redirect_stdout(io.StringIO()):
            lab.initialize()
    def tearDown(self):
        self.state_patch.stop()
        self.tmp.cleanup()

    def test_credentials_are_private_random_and_restart_preserves_them(self):
        before = (lab.STATE / 'credentials.env').read_bytes()
        self.assertEqual(len(set(lab.credentials().values())), 4)
        self.assertEqual(stat.S_IMODE(lab.STATE.stat().st_mode), 0o700)
        for name in ('credentials.env', 'owner.json'):
            self.assertEqual(stat.S_IMODE((lab.STATE / name).stat().st_mode), 0o600)
        with contextlib.redirect_stdout(io.StringIO()) as out:
            lab.initialize()
        self.assertEqual(before, (lab.STATE / 'credentials.env').read_bytes())
        self.assertNotIn(lab.credentials()['LAB_DB_PASSWORD'], out.getvalue())

    def test_public_credentials_and_symlink_are_rejected(self):
        path = lab.STATE / 'credentials.env'
        path.chmod(0o644)
        with self.assertRaises(lab.LabError): lab.credentials()
        path.chmod(0o600)
        path.rename(lab.STATE / 'saved')
        path.symlink_to(lab.STATE / 'saved')
        with self.assertRaises(lab.LabError): lab.initialize()

    def test_credentials_from_other_checkout_are_rejected(self):
        path = lab.STATE / 'owner.json'
        owner = json.loads(path.read_text()); owner['root'] = '/other-checkout'
        path.write_text(json.dumps(owner))
        with self.assertRaises(lab.LabError): lab.credentials()

    def test_subprocess_removes_connection_overrides_and_masks_raw_errors(self):
        with patch.dict(os.environ, {'DATABASE_URL':'SECRET', 'PGPASSWORD':'SECRET', 'DOCKER_HOST':'tcp://remote', 'COMPOSE_FILE':'foreign'}), patch.object(lab.subprocess, 'run', return_value=result(code=1)) as run:
            with self.assertRaises(lab.LabError) as raised: lab.run(['docker'])
        self.assertNotIn('SECRET', str(raised.exception))
        env = run.call_args.kwargs['env']
        for key in ('DATABASE_URL','PGPASSWORD','DOCKER_HOST','COMPOSE_FILE'): self.assertNotIn(key, env)

    def test_timeout_does_not_expose_process_or_password(self):
        with patch.object(lab.subprocess, 'run', side_effect=subprocess.TimeoutExpired(['SECRET'], 1)):
            with self.assertRaises(lab.LabError) as raised: lab.run(['docker'])
        self.assertNotIn('SECRET', str(raised.exception))

    def test_remote_docker_is_blocked_before_other_commands(self):
        with patch.object(lab.shutil, 'which', return_value='/usr/bin/docker'), patch.object(lab, 'run', return_value=result(json.dumps([{'Endpoints':{'docker':{'Host':'tcp://remote:2375'}}}]))) as run:
            with self.assertRaises(lab.LabError): lab.docker()
        self.assertEqual(run.call_count, 1)

    def test_foreign_volume_is_never_changed(self):
        with patch.object(lab, 'run', side_effect=[result(lab.VOLUME), result(json.dumps([{'Labels':{'com.titansystem.lab-instance':'other'}}]))]) as run:
            with self.assertRaises(lab.LabError): lab.ownership()
        self.assertEqual(run.call_count, 2)

    def test_foreign_project_container_is_blocked(self):
        with patch.object(lab, 'run', side_effect=[result(), result('abcdef123456'), result('{}')]):
            with self.assertRaises(lab.LabError): lab.ownership()

    def test_port_conflict_prevents_build_and_start(self):
        sock = MagicMock(); sock.__enter__.return_value.bind.side_effect = OSError()
        with patch.object(lab, 'docker'), patch.object(lab, 'ownership'), patch.object(lab, 'compose', return_value=result()) as compose, patch.object(lab.socket, 'socket', return_value=sock):
            with self.assertRaises(lab.LabError): lab.start()
        self.assertFalse(any(c.args[0] in ('build','up','run') for c in compose.call_args_list))

    def test_migration_failure_prevents_api_start_and_preserves_database(self):
        calls = []
        def compose(*args, **kwargs):
            calls.append(args)
            if args[0] == 'run': raise lab.LabError('migration failed')
            return result('abcdef123456' if args[0] == 'ps' else '')
        with patch.object(lab, 'docker'), patch.object(lab, 'ownership'), patch.object(lab, 'wait_health'), patch.object(lab, 'compose', side_effect=compose):
            with self.assertRaises(lab.LabError): lab.start()
        self.assertIn(('up','-d','postgres'), calls)
        self.assertNotIn(('up','-d','--no-deps','api'), calls)
        self.assertFalse(any(c[0] in ('down','stop') for c in calls))

    def test_check_requires_real_tables_and_does_not_apply_migrations(self):
        with patch.object(lab, 'docker'), patch.object(lab, 'ownership'), patch.object(lab, 'wait_health'), patch.object(lab, 'compose', side_effect=[result(), result('products\n')]) as compose:
            with self.assertRaises(lab.LabError): lab.check()
        self.assertEqual(compose.call_args_list[0].args, ('run','--no-deps','-T','migrate','check'))

    def test_check_requires_http_health_after_tables(self):
        tables = '\n'.join(['tenants','users','products','discount_suggestions','online_sessions','online_purchase_documents','online_purchase_document_operations','online_purchase_document_outbox'])
        response = MagicMock(); response.__enter__.return_value.status = 200
        response.__enter__.return_value.read.return_value = b'{"status":"indisponivel"}'
        opener = MagicMock(); opener.open.return_value = response
        with patch.object(lab, 'docker'), patch.object(lab, 'ownership'), patch.object(lab, 'wait_health'), patch.object(lab, 'compose', side_effect=[result(),result(tables)]), patch.object(lab.urllib.request, 'build_opener', return_value=opener):
            with self.assertRaises(lab.LabError): lab.check()
        response.__enter__.return_value.read.return_value = b'{"status":"ativo"}'
        with patch.object(lab, 'docker'), patch.object(lab, 'ownership'), patch.object(lab, 'wait_health'), patch.object(lab, 'compose', side_effect=[result(),result(tables)]), patch.object(lab.urllib.request, 'build_opener', return_value=opener), contextlib.redirect_stdout(io.StringIO()) as out:
            lab.check()
        self.assertIn('PASSOU', out.getvalue())

    def test_diagnose_prints_only_state_metadata(self):
        raw = json.dumps([{'Service':'api','State':'running','Health':'healthy','ExitCode':0,'Environment':'SECRET','Command':'SECRET'}])
        with patch.object(lab, 'docker'), patch.object(lab, 'ownership'), patch.object(lab, 'compose', return_value=result(raw)), contextlib.redirect_stdout(io.StringIO()) as out:
            lab.diagnose()
        self.assertNotIn('SECRET', out.getvalue())
        self.assertEqual(json.loads(out.getvalue())['Health'], 'healthy')

    def test_stop_preserves_volume_and_has_no_removal_command(self):
        with patch.object(lab, 'docker'), patch.object(lab, 'ownership'), patch.object(lab, 'compose') as compose, contextlib.redirect_stdout(io.StringIO()): lab.stop()
        compose.assert_called_once_with('stop', timeout=90)

    def test_checker_script_never_runs_migrations_in_check_mode(self):
        # Exercise the actual shell driver with a recording maintenance executable.
        folder = Path(self.tmp.name)
        tool = folder / 'titan-online'
        tool.write_text('#!/bin/sh\nprintf "%s\\n" "$1" >> "$LAB_TEST_TRACE"\n')
        tool.chmod(0o700)
        trace = folder / 'trace'
        env = dict(os.environ, PATH=str(folder)+':'+os.environ['PATH'], LAB_TEST_TRACE=str(trace))
        script = lab.ROOT / 'deploy/lab52/migrate.sh'
        subprocess.run(['sh',str(script),'check'],env=env,check=True,stdout=subprocess.PIPE)
        commands = trace.read_text().splitlines()
        self.assertEqual(len(commands), 10)
        self.assertTrue(all(c.startswith('check-') for c in commands))
        trace.unlink()
        subprocess.run(['sh',str(script),'migrate'],env=env,check=True,stdout=subprocess.PIPE)
        self.assertEqual(len(trace.read_text().splitlines()), 20)

if __name__ == '__main__': unittest.main()
