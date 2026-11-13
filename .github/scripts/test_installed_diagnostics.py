import contextlib
import importlib.util
import io
import json
from pathlib import Path
import tempfile
import unittest

spec = importlib.util.spec_from_file_location('installed', Path(__file__).with_name('installed_diagnostics.py'))
installed = importlib.util.module_from_spec(spec)
spec.loader.exec_module(installed)

class InstalledDiagnosticsTests(unittest.TestCase):
    def test_only_fixed_labels_and_stages_no_raw_credentials(self):
        events = [
            {'Test': 'TestWindowsCustomerKeyLifecycle', 'Output': '    vault_lifecycle_test.go:21: PRIVATE_CREDENTIAL'},
            {'Test': 'TestWindowsCustomerKeyLifecycle', 'Output': '    private_credential_test.go:11: PRIVATE_PERSONAL_VALUE'},
            {'Test': 'TestPilotInstalledCashClosingCustodyRestore', 'Output': 'Screen failure phase: login detail: screen missing: authenticated home'},
            {'Test': 'TestPilotInstalledCashClosingCustodyRestore', 'Output': 'token=PRIVATE_CREDENTIAL CPF=PRIVATE_PERSONAL_VALUE'},
            {'Test': 'OtherTest', 'Output': 'Screen phase: unrelated'},
            {'Test': 'TestPilotInstalledCashClosingCustodyRestore', 'Output': 'installed server drain failed PRIVATE_CREDENTIAL'},
            {'Test': 'TestPilotInstalledCashClosingCustodyRestore', 'Action': 'fail', 'Elapsed': 3.79},
        ]
        with tempfile.TemporaryDirectory() as root:
            p = Path(root)/'events.jsonl'
            p.write_text('\n'.join(json.dumps(e) for e in events))
            out = io.StringIO()
            with contextlib.redirect_stdout(out):
                installed.diagnostics(p)
            text = out.getvalue()
            self.assertNotIn('PRIVATE_', text)
            self.assertNotIn('unrelated', text)
            self.assertNotIn('private_credential_test.go', text)
            self.assertIn('vault_lifecycle_test.go', text)
            values = [json.loads(l) for l in text.splitlines()]
            self.assertTrue(any(v.get('diagnostic') == 'screen missing: authenticated home' for v in values))
            self.assertTrue(any(v.get('action') == 'fail' for v in values))

if __name__ == '__main__':
    unittest.main()
