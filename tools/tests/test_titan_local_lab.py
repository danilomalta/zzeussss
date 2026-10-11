import importlib.util
import sys
from pathlib import Path
import json
import tempfile
import unittest
from unittest.mock import patch
import subprocess

sys.path.insert(0,str(Path(__file__).resolve().parents[1]))
import titan_local_lab as lab

class LocalLabTests(unittest.TestCase):
    def test_environment_does_not_inherit_online_credentials_or_proxy(self):
        with patch.dict(lab.os.environ,{'DATABASE_URL':'SECRET','HTTP_PROXY':'SECRET','PGPASSWORD':'SECRET'}):
            self.assertFalse(set(lab.env()) & {'DATABASE_URL','HTTP_PROXY','PGPASSWORD'})

    def test_raw_errors_and_passwords_are_not_shown(self):
        p=subprocess.CompletedProcess([],1,'SECRET','SECRET')
        with patch.object(lab.subprocess,'run',return_value=p):
            with self.assertRaises(lab.guard.LabError) as err: lab.run(['command'],password='SECRET')
        self.assertNotIn('SECRET',str(err.exception))

    def test_failed_initialization_is_not_overwritten(self):
        with tempfile.TemporaryDirectory() as tmp, patch.object(lab,'STATE',Path(tmp)/'state'):
            lab.STATE.mkdir(mode=0o700)
            marker=lab.STATE/'password.txt'
            marker.write_text('preserved')
            with self.assertRaises(OSError): lab.initialize()
            self.assertEqual(marker.read_text(),'preserved')

    def test_metadata_rejects_public_permissions_before_reading_secrets(self):
        with tempfile.TemporaryDirectory() as tmp, patch.object(lab,'STATE',Path(tmp)):
            lab.STATE.chmod(0o755)
            with self.assertRaises(lab.guard.LabError): lab.metadata()

    def test_peer_export_does_not_overwrite_other_public_file(self):
        with tempfile.TemporaryDirectory() as tmp, patch.object(lab,'STATE',Path(tmp)), patch.object(lab,'PUBLIC',Path(tmp)/'export.json'):
            for name,body in [('peer.encryption','private'),('peer-public.json','public'),('export.json','different')]:
                lab.write_private(Path(tmp)/name,body)
            with patch.object(lab,'metadata',return_value=({'owner_id':'owner'},'SECRET')), patch.object(lab,'binary',return_value='titan-peer'), patch.object(lab,'run',return_value='public fingerprints'):
                with self.assertRaises(lab.guard.LabError): lab.peer()
            self.assertEqual(lab.PUBLIC.read_text(),'different')

    def test_session_checks_context_and_revocation_without_printing_token(self):
        value={k:k+'-value' for k in lab.IDS}
        me={k:value['owner_id' if k=='identity_id' else k] for k in ('identity_id','tenant_id','store_id','device_id')}
        with patch.object(lab,'http',side_effect=[{'status':'local'},{'token':'SECRET'},me,None,None]) as http:
            lab.session_probe(value,'PASSWORD')
        self.assertEqual(http.call_args_list[-1].kwargs,{'token':'SECRET','expected':401})

    def test_foreign_identity_still_logs_out_and_is_rejected(self):
        value={k:k+'-value' for k in lab.IDS}
        with patch.object(lab,'http',side_effect=[{'status':'local'},{'token':'SECRET'},{'tenant_id':'foreign'},None]) as http:
            with self.assertRaises(lab.guard.LabError): lab.session_probe(value,'PASSWORD')
        self.assertEqual(http.call_args_list[-1].args,('logout',{},'SECRET',204))

if __name__=='__main__': unittest.main()
