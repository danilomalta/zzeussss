"""Print only fixed, non-business diagnostic labels for installed acceptance."""
import json
import re
import sys

TESTS = {'TestWindowsVaultSQLiteURIIdentity','TestConcurrentPrivatePublication','TestWindowsCustomerNativeInitialization', 'TestWindowsCustomerKeyLifecycle','TestWindowsPrivacyRejectsBroadGrantsAndFileReplacement', 'TestWindowsDefaultTempAndNoOverwritePublication','TestCreateReopenAndRepeatMigration','TestPilotInstalledCashPartialReceivingRestore', 'TestPilotInstalledCashClosingCustodyRestore'}
FIXED = (
    'production fiscal installation is not provisioned or current', 'explicit user-private temporary directory refused',
    'private customer key directory unavailable', 'native customer initialization/check result mismatch', 'native station proof failed', 'owner duplicated or lost', 'station key ACL is not private', 'not shown exactly once', 'backup overwritten', 'rotation lost secret', 'old backup no longer recoverable', 'missing key accepted',
    'private ACL refused', 'private parent refused', 'default user-private temporary directory refused', 'broad file grant accepted', 'broadly accessible parent accepted', 'existing destination overwritten', 'private file permissions unavailable',
    'sqlite failure', 'opening SQLite', 'reabrir banco', 'abrir banco local',
    'SQLite existente deve ser arquivo regular privado (0600)', 'banco não é arquivo regular 0600',
    'arquivo privado não é regular 0600',
    'explicit absolute installed binaries/controller required', 'installed acceptance input unavailable',
    'installed server spawn failed', 'installed server readiness failed', 'installed server exit failed',
    'installed server drain failed', 'lost final response was not exercised',
    'recovery repeated fiscal/commercial finalization POST', 'receiver observation login unavailable',
    'receiver observation login refused', 'original drawer missing', 'fixture station export failed',
    'fixture station restore failed', 'restored login unavailable', 'invalid authentication retry policy',
    'restored login refused after bounded wait', 'invalid restored authentication response',
    'installed workflow changed after CLI restore', 'installed outcome mismatch',
)

def diagnostics(path):
    with open(path, encoding='utf-8') as source:
      for line in source:
          event = json.loads(line)
          name = event.get('Test', '').split('/')[0]
          if name not in TESTS:
              continue
          if event.get('Action') in ('pass', 'skip', 'fail'):
              print(json.dumps({'test': name, 'action': event['Action'], 'elapsed': event.get('Elapsed')}))
          output = event.get('Output', '')
          location = re.search(r'^\s*([a-z_]+_test\.go):(\d{1,5}):', output)
          if location and location[1] in {'vault_lifecycle_test.go','customer_keys_windows_test.go','vault_uri_windows_test.go','customer_init_windows_test.go','pilot_installed_screen_test.go','localdb_test.go','privatefile_windows_test.go','publication_test.go'}:
              print(json.dumps({'test': name, 'source': location[1], 'line': int(location[2])}))
          code = re.search(r'vault sqlite code: (-?\d{1,5})\b', output)
          if code:
              print(json.dumps({'test': name, 'sqliteCode': int(code[1])}))
          for label in FIXED:
              if label in output:
                  print(json.dumps({'test': name, 'diagnostic': label}))
          phase = re.search(r'Screen (?:failure )?phase: ([\w -]{1,80})', output)
          if phase:
              value = phase[1].split(' detail')[0]
              # Stage labels are controller actions, never field values.
              print(json.dumps({'test': name, 'phase': value}, ensure_ascii=True))
          detail = re.search(r'detail: (screen missing: [a-z -]+|desktop [a-z_0-9-]+|assertion or inspection failed|application page unavailable|inspection timeout|screen action failed|disposable application did not close gracefully)', output)
          if detail:
              print(json.dumps({'test': name, 'diagnostic': detail[1]}))

if __name__ == '__main__':
    diagnostics(sys.argv[1])
