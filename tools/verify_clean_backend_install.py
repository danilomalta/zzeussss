"""CI only: fresh Linux installation using real binaries and discarded data."""
import argparse
import json
from pathlib import Path
import secrets
import subprocess
import sys
import tempfile
import titan_install

p=argparse.ArgumentParser()
p.add_argument('--binaries',required=True)
p.add_argument('--report',required=True)
args=p.parse_args()
source=Path(args.binaries).absolute()
report=Path(args.report).absolute()
if report.exists():raise SystemExit('New report path required')
with tempfile.TemporaryDirectory(prefix='titan-clean-install-') as temporary:
    base=Path(temporary)
    bundle=base/'bundle'
    destination=base/'installation'
    assert not destination.exists()
    titan_install.pack(source,bundle,'ci-pilot',1)
    manager=titan_install.Manager(destination)
    manager.install(bundle)
    assert manager.status()['initialized'] is False
    command=[sys.executable,str(Path(titan_install.__file__).absolute()),'run','--root',str(destination)]
    password=secrets.token_urlsafe(32).encode()+b'\n'
    subprocess.run([*command,'titan-local','init','--empresa','Synthetic CI company','--loja','Synthetic CI store','--dono','Synthetic CI owner'],input=password,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL,check=True,timeout=60)
    subprocess.run([*command,'titan-local','check'],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL,check=True,timeout=30)
    status=manager.status()
    assert status['initialized'] is True and status['enabled'] is True
    # A second process reads the same installed station after the first exits.
    subprocess.run([*command,'titan-local','check'],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL,check=True,timeout=30)
    value={'platform':'linux','real_binaries':True,'fresh_installation':True,'initialized':True,'second_process_check':True,'server_started':False,'commercial_data':False,'windows_backend_proven':False}
    with report.open('x') as out:json.dump(value,out,indent=2)
    print(json.dumps(value))
