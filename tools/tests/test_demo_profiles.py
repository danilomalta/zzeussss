import importlib.util
from pathlib import Path
import unittest
spec = importlib.util.spec_from_file_location('demo_profiles', Path(__file__).resolve().parents[1]/'demo_pdv.py')
demo=importlib.util.module_from_spec(spec)
spec.loader.exec_module(demo)
class ProfileTests(unittest.TestCase):
    def test_hr_and_accounting_never_seed_stock_or_cash(self):
        class API:
            def __init__(self):self.calls=[]
            def request(self,path,token=None,body=None,method=None):
                self.calls.append(path)
                return {'/login':{'token':'token'},'/me':{'tenant_id':'t','store_id':'s','device_id':'d','identity_id':'i'},'/module-contracts':{'revision':1},'/logout':{}}[path]
        for profile in ['rh','contabilidade']:
            api=API()
            self.assertEqual(demo.seed(api,'i','test-password',{'tenant_id':'t','store_id':'s','device_id':'d'},{},profile),[])
            self.assertEqual(api.calls,['/login','/me','/module-contracts','/logout'])
if __name__=='__main__':unittest.main()
