package fiscalproduction

import "testing"

func TestFocusNFSENeverFallsBackToGoodsInvoiceWithoutMunicipalContract(t *testing.T) {
	for _, model := range []string{"nfse", "NFS-e", "service"} {
		_, err := newFocusProvider(TenantFiscalConfiguration{DocumentModel: model, Environment: "homologation", CertificateType: "A1"}, []byte("SyntheticOnlyToken"))
		if err == nil {
			t.Fatal("service accepted as goods invoice")
		}
	}
}
