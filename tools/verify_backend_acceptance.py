"""Reject a backend test report that silently skipped real integrations."""
import argparse
import json
from pathlib import Path

REQUIRED = {
    ('titansystem-backend/cmd/titan-fiscal', 'TestFiscalProductionConfigNativeCLI'),
    ('titansystem-backend/internal/onlinesessions', 'TestPostgresDurableRotationConcurrencyAndRollback'),
    ('titansystem-backend/internal/core/routes', 'TestPostgresCatalogCreationAndDiscountAtomicity'),
    *{('titansystem-backend/internal/localapi', name) for name in (
        'TestCashJournalFrontendBridge', 'TestDeliveryFrontendBridge',
        'TestMaintenanceFrontendBridge', 'TestFiscalCustomerFrontendBridge',
        'TestFiscalTaxRealHTTPRestartAndRestore',
        'TestHTTPFiscalProductionTaxQuotesBuildSignTransmitAndRecover',
        'TestHTTPFiscalProductionTaxDestinationDIFALAndFCP',
        'TestHTTPFiscalProductionTaxNativeLoopback',
        'TestHTTPFiscalProductionTaxDeferment',
        'TestHTTPFiscalProductionTaxSimpleCredit',
        'TestHTTPFiscalProductionTaxSimpleSTQuoteAndSaleGuard',
        'TestHTTPFiscalProductionTaxSimpleSTZeroAdditionalAmount',
        'TestHTTPFiscalTaxPricedSaleToIssuedXML',
        'TestHTTPFiscalTaxPricedSaleSessionGuardAndRecovery',
        'TestHTTPFiscalTaxPricedSaleNativeLoopback',
        'TestHTTPFiscalTaxPricedSaleIPIIncludedBeforePayment',
        'TestHTTPFiscalTaxPricedSaleRequiredAcrossWriters',
        'TestHTTPFiscalTaxPricedSaleModeRequiresExplicitOwnerDecision')},
}
ALLOWED_SKIPS = {
    ('titansystem-backend/internal/localapi', 'TestPilotCashProcess'),
    ('titansystem-backend/internal/localapi', 'TestPilotCashScreenFlow'),
    ('titansystem-backend/internal/localdb', 'TestMigrationCrashChild'),
}


def verify(path):
    passed, skipped, packages = set(), set(), set()
    with Path(path).open(encoding='utf-8') as report:
        for line in report:
            event = json.loads(line)
            action, package, test = event.get('Action'), event.get('Package'), event.get('Test')
            if action == 'fail':
                raise ValueError(f'Teste ou pacote reprovado: {package}/{test or ""}')
            if action == 'pass':
                if test:
                    passed.add((package, test))
                else:
                    packages.add(package)
            if action == 'skip' and test:
                skipped.add((package, test))
    missing = REQUIRED - passed
    unexpected = skipped - ALLOWED_SKIPS
    if missing or unexpected or any(package not in packages for package, _ in REQUIRED):
        raise ValueError(f'Integrações ausentes: {sorted(missing)}; ignorados não permitidos: {sorted(unexpected)}')
    return {'status': 'PASS', 'required_integrations': len(REQUIRED),
            'top_level_passed': sum('/' not in test for _, test in passed),
            'passed_including_subtests': len(passed), 'skipped': len(skipped),
            'skipped_tests': sorted(test for _, test in skipped)}


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('report', type=Path)
    args = parser.parse_args()
    try:
        print(json.dumps(verify(args.report), ensure_ascii=False, indent=2))
    except (OSError, ValueError, TypeError, AttributeError) as error:
        parser.exit(1, f'Aceite não comprovado: {error}\n')
