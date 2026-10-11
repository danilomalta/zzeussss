import sys
from pathlib import Path
import tempfile
import json
import unittest
from unittest.mock import patch
sys.path.insert(0,str(Path(__file__).resolve().parents[1]))
import titan_pair_lab as lab

class PairLabTests(unittest.TestCase):
    def test_public_fingerprints_require_both_valid_hashes(self):
        with patch.object(lab.local,'run',return_value='Ed25519 SHA-256: '+('a'*64)):
            with self.assertRaises(lab.local.guard.LabError): lab.prints('tool','file')

    def test_partial_state_is_preserved_without_new_build_or_approval(self):
        with tempfile.TemporaryDirectory() as tmp, patch.object(lab,'STATE',Path(tmp)/'state'):
            lab.STATE.mkdir(mode=0o700)
            marker=lab.STATE/'marker'; marker.write_text('preserved')
            with patch.object(lab.local,'metadata',return_value=({},'SECRET')), patch.object(lab.local,'run') as run:
                with self.assertRaises(OSError): lab.demo()
            run.assert_not_called()
            self.assertEqual(marker.read_text(),'preserved')

    def test_result_from_other_parent_is_rejected_before_database_read(self):
        with tempfile.TemporaryDirectory() as tmp, patch.object(lab,'STATE',Path(tmp)):
            lab.local.write_private(lab.STATE/'result.json',json.dumps({'root':str(lab.local.ROOT),'parent_device':'foreign'}))
            with patch.object(lab.local,'metadata',return_value=({'device_id':'local'},'SECRET')), patch.object(lab,'facts') as facts:
                with self.assertRaises(lab.local.guard.LabError): lab.verify()
            facts.assert_not_called()

    def test_approval_with_implicit_event_grant_cannot_claim_success(self):
        with tempfile.TemporaryDirectory() as tmp, patch.object(lab,'STATE',Path(tmp)):
            lab.local.write_private(lab.STATE/'result.json',json.dumps({'root':str(lab.local.ROOT),'parent_device':'parent','candidate_device':'12345678-1234-1234-1234-123456789012'}))
            bad={'status':'approved','events':['requested','proved','approved'],'event_grants':1,'transport_trust':0}
            with patch.object(lab.local,'metadata',return_value=({'device_id':'parent'},'SECRET')), patch.object(lab,'facts',return_value=bad):
                with self.assertRaises(lab.local.guard.LabError): lab.verify()

if __name__=='__main__': unittest.main()
