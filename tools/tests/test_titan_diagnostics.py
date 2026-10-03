import importlib.util
from pathlib import Path
import subprocess
import tempfile
import unittest

spec = importlib.util.spec_from_file_location('titan_diagnostics', Path(__file__).resolve().parents[1] / 'titan_diagnostics.py')
diag = importlib.util.module_from_spec(spec)
spec.loader.exec_module(diag)

class DiagnosticTests(unittest.TestCase):
    def test_missing_tools_cannot_report_success(self):
        result = diag.run_checks(finder=lambda _: None)
        self.assertTrue(all(row[1] == 'NÃO EXECUTADO' for row in result))

    def test_tests_do_not_capture_or_render_raw_logs(self):
        calls = []
        def fake(command, **kwargs):
            calls.append((command, kwargs))
            return subprocess.CompletedProcess(command, 1)
        result = diag.run_checks(runner=fake, finder=lambda _: '/bin/tool')
        self.assertTrue(all(row[1] == 'FALHOU' for row in result))
        self.assertTrue(all(k['stdout'] is subprocess.DEVNULL and k['stderr'] is subprocess.DEVNULL for _,k in calls))
        self.assertNotIn('shell', calls[0][1])

    def test_private_report_escapes_text(self):
        with tempfile.TemporaryDirectory() as tmp:
            path = diag.save_report([('<script>','FALHOU','test')], Path(tmp))
            self.assertEqual(path.stat().st_mode & 0o777, 0o600)
            self.assertEqual(path.parent.stat().st_mode & 0o777, 0o700)
            self.assertIn('&lt;script&gt;', path.read_text())
            self.assertNotIn('<h2><script>', path.read_text())

if __name__ == '__main__':
    unittest.main()
