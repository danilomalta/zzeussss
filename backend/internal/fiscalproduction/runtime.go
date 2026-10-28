// Package fiscalproduction connects trusted installation credentials to the
// authenticated production journal. It is disabled unless explicitly provisioned.
package fiscalproduction

import (
	"context"
	"crypto/x509"
	"database/sql"
	"errors"
	"net"
	"strconv"
	"strings"
	"time"

	"titansystem-backend/internal/fiscalcore"
	"titansystem-backend/internal/localdb/entitlementstore"
	"titansystem-backend/internal/localdb/fiscal"
	"titansystem-backend/internal/localdb/identity"
)

var ErrUnavailable = errors.New("production fiscal installation is not provisioned or current")

type NetworkTrust struct {
	ServerRoots *x509.CertPool
	DialContext func(context.Context, string, string) (net.Conn, error)
}
type Runtime struct {
	binding      fiscalcore.ProductionBinding
	credential   *fiscalcore.Credential
	crls         func() ([][]byte, error)
	qr           *fiscalcore.QRConfiguration
	consultation string
	network      NetworkTrust
}

// Provision receives process-owned inputs; none of them is accepted by the
// tenant's HTTP API. The CRL loader is rechecked immediately before each send.
func Provision(binding fiscalcore.ProductionBinding, credential *fiscalcore.Credential, crls func() ([][]byte, error), qr *fiscalcore.QRConfiguration, consultation string, network NetworkTrust) (*Runtime, error) {
	r := &Runtime{binding: binding, credential: credential, crls: crls, consultation: consultation, network: network}
	if qr != nil {
		copy := *qr
		r.qr = &copy
		sum := fiscalcore.ProductionSupplementSHA256(copy, consultation)
		if r.binding.SupplementSHA256 != "" && r.binding.SupplementSHA256 != sum {
			return nil, ErrUnavailable
		}
		r.binding.SupplementSHA256 = sum
	}
	if err := r.Validate(); err != nil {
		return nil, err
	}
	return r, nil
}
func (r *Runtime) Validate() error {
	if r == nil || r.credential == nil || r.crls == nil || !r.binding.Valid(time.Now()) || r.credential.Fingerprint() != r.binding.CertificateSHA256 || !fiscal.ValidCNPJ(r.binding.Issuer) {
		return ErrUnavailable
	}
	if r.binding.Model == "65" {
		if r.qr == nil || r.qr.Version != 2 && r.qr.Version != 3 || !fiscalcore.PublicFiscalConsultationURL(r.qr.BaseURL) || !fiscalcore.PublicFiscalConsultationURL(r.consultation) {
			return ErrUnavailable
		}
		if r.binding.SupplementSHA256 != fiscalcore.ProductionSupplementSHA256(*r.qr, r.consultation) {
			return ErrUnavailable
		}
		if r.qr.Version == 2 {
			if len(r.qr.CSC) < 16 || len(r.qr.CSC) > 128 || strings.TrimSpace(r.qr.CSC) != r.qr.CSC || len(r.qr.CSCID) < 1 || len(r.qr.CSCID) > 6 {
				return ErrUnavailable
			}
			n, e := strconv.Atoi(r.qr.CSCID)
			if e != nil || n < 1 {
				return ErrUnavailable
			}
		} else if r.qr.CSC != "" || r.qr.CSCID != "" {
			return ErrUnavailable
		}
	} else if r.qr != nil || r.consultation != "" || r.binding.SupplementSHA256 != "" {
		return ErrUnavailable
	}
	lists, err := r.crls()
	if err != nil || r.credential.CheckRevocation(lists, time.Now()) != nil {
		return ErrUnavailable
	}
	return nil
}
func (r *Runtime) Binding() fiscalcore.ProductionBinding {
	if r == nil {
		return fiscalcore.ProductionBinding{}
	}
	return r.binding
}

type RequestContext struct {
	DB       *sql.DB
	License  *entitlementstore.Store
	Actor    identity.Scope
	Device   identity.DeviceContext
	IntentID string
	Token    string
}

func (r *Runtime) Prepare(ctx context.Context, a RequestContext, operation string, expected int, in fiscalcore.Invoice) (fiscal.ProductionPreparation, error) {
	if err := r.Validate(); err != nil {
		return fiscal.ProductionPreparation{}, err
	}
	return fiscal.PrepareProduction(ctx, a.DB, a.License, a.Actor, a.Device, a.IntentID, a.Token, operation, expected, in, r.binding)
}
func (r *Runtime) Approve(ctx context.Context, a RequestContext, operation string, revision int, sha, reference string) error {
	if err := r.Validate(); err != nil {
		return err
	}
	return fiscal.ApproveProduction(ctx, a.DB, a.License, a.Actor, a.Device, a.IntentID, a.Token, operation, revision, sha, reference, r.binding)
}

type guardedRecorder struct {
	*fiscal.ProductionRecorder
	runtime *Runtime
}

func (g *guardedRecorder) BeforeSend(ctx context.Context, id string) error {
	if err := g.runtime.Validate(); err != nil {
		return err
	}
	return g.ProductionRecorder.BeforeSend(ctx, id)
}
func (g *guardedRecorder) ValidateProduction(ctx context.Context, in fiscalcore.Invoice, fingerprint, endpoint string) error {
	if err := g.runtime.Validate(); err != nil {
		return err
	}
	return g.ProductionRecorder.ValidateProduction(ctx, in, fingerprint, endpoint)
}

func (r *Runtime) client(ctx context.Context, a RequestContext, revision int, kind string) (*fiscalcore.Client, *guardedRecorder, error) {
	if err := r.Validate(); err != nil {
		return nil, nil, err
	}
	recorder, err := fiscal.NewProductionRecorder(ctx, a.DB, a.License, a.Actor, a.Device, a.IntentID, a.Token, kind, revision, r.binding)
	if err != nil {
		return nil, nil, err
	}
	guard := &guardedRecorder{recorder, r}
	endpoint := r.binding.AuthorizationURL
	if kind == "query" {
		endpoint = r.binding.ConsultationURL
	}
	client, err := fiscalcore.NewClientWithServerTrust(endpoint, recorder.Scope(), r.credential, guard, r.network.ServerRoots, r.network.DialContext)
	return client, guard, err
}

func (r *Runtime) Emit(ctx context.Context, a RequestContext, operation string, revision int, reviewSHA string) (fiscalcore.ProductionEvidence, error) {
	// Replays read the existing receipt before requiring a live signer. A new
	// operation never reuses this path as authority for another HTTP POST.
	status, err := fiscal.ReadProduction(ctx, a.DB, a.Actor, a.Device, a.IntentID, a.Token)
	if err != nil {
		return fiscalcore.ProductionEvidence{}, err
	}
	if status.Preparation.Revision != revision || status.Preparation.ReviewSHA256 != reviewSHA || !status.Approved {
		return fiscalcore.ProductionEvidence{}, fiscal.ErrConflict
	}
	if status.Evidence.OperationID != "" {
		if status.Evidence.OperationID == operation && status.EvidenceRevision == revision {
			return status.Evidence, nil
		}
		if status.Evidence.State != "rejected_requires_review" && status.Evidence.State != "batch_rejected_requires_review" {
			return status.Evidence, fiscalcore.ErrRecorded
		}
	}
	client, _, err := r.client(ctx, a, revision, "issue")
	if err != nil {
		return fiscalcore.ProductionEvidence{}, err
	}
	defer client.Close()
	return client.IssueProductionNFe(ctx, operation, "1", status.Preparation.Invoice, r.qr, r.consultation)
}
func (r *Runtime) Query(ctx context.Context, a RequestContext, operation string, revision int) (fiscal.ProductionStatus, error) {
	client, _, err := r.client(ctx, a, revision, "query")
	if err != nil {
		return fiscal.ProductionStatus{}, err
	}
	defer client.Close()
	status, err := fiscal.ReadProduction(ctx, a.DB, a.Actor, a.Device, a.IntentID, a.Token)
	if err != nil {
		return status, err
	}
	key, err := fiscalcore.AccessKey(status.Preparation.Invoice.Key)
	if err != nil {
		return status, err
	}
	_, err = client.ConsultProductionNFe(ctx, operation, key)
	if err != nil && !errors.Is(err, fiscalcore.ErrRecorded) && !errors.Is(err, fiscalcore.ErrUncertain) {
		return status, err
	}
	return fiscal.ReadProduction(ctx, a.DB, a.Actor, a.Device, a.IntentID, a.Token)
}
