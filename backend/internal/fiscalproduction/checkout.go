package fiscalproduction

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"
	"titansystem-backend/internal/fiscalcore"
	"titansystem-backend/internal/localdb/fiscal"
)

var ErrCheckoutProduction = errors.New("production emission is disabled for checkout")

func (r *Runtime) ValidateCheckoutHomologation(ctx context.Context, a RequestContext, environment int) error {
	if environment != 2 {
		return ErrCheckoutProduction
	}
	if r == nil {
		return ErrUnavailable
	}
	p, err := fiscal.ReadProfile(ctx, a.DB, a.Actor, a.Device, r.binding.ProfileID, 0)
	if err != nil {
		return err
	}
	if p.Input.Environment != "homologation" || p.Input.Model != "65" {
		return ErrCheckoutProduction
	}
	if p.Revision != r.binding.ProfileRevision || a.Actor.TenantID != r.binding.TenantID || a.Actor.StoreID != r.binding.StoreID || a.Device.DeviceID != r.binding.DeviceID {
		return ErrUnavailable
	}
	return r.Validate()
}

type checkoutGuard struct {
	*fiscal.HomologationRecorder
	runtime *Runtime
}

func (g *checkoutGuard) BeforeSend(ctx context.Context, id string) error {
	if err := g.runtime.Validate(); err != nil {
		return err
	}
	return g.HomologationRecorder.BeforeSend(ctx, id)
}

// EmitCheckoutHomologation delegates numbering, mapping validation, signing,
// transmission and recovery to the existing implementations. Sale is committed
// before this function; failures append a separate durable checkout status.
func (r *Runtime) EmitCheckoutHomologation(ctx context.Context, a RequestContext, saleID string, in fiscalcore.Invoice) (state fiscal.CheckoutStatus, err error) {
	if in.Environment != 2 {
		return state, ErrCheckoutProduction
	}
	if r == nil {
		return state, ErrUnavailable
	}
	sum := sha256.Sum256([]byte(a.Actor.TenantID + "/" + a.Actor.StoreID + "/" + a.Device.DeviceID + "/" + saleID))
	op := "checkout-" + hex.EncodeToString(sum[:])
	state = fiscal.CheckoutStatus{State: "pending", OperationID: op}
	persist := func() error {
		c, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
		defer cancel()
		return fiscal.RecordCheckoutStatus(c, a.DB, a.Actor, a.Device, saleID, state)
	}
	if err = persist(); err != nil {
		state.State = "failed"
		state.ErrorCode = "fiscal_status_persistence_failed"
		return state, err
	}
	defer func() {
		if err != nil {
			state.State = "failed"
			state.ErrorCode = "fiscal_homologation_failed"
			if errors.Is(err, ErrCheckoutProduction) {
				state.ErrorCode = "fiscal_checkout_production_disabled"
			}
			if errors.Is(err, fiscalcore.ErrUncertain) || errors.Is(err, fiscalcore.ErrRecorded) {
				state.State = "recovery_required"
			}
		}
		if e := persist(); e != nil {
			state.ErrorCode = "fiscal_status_persistence_failed"
			err = e
		}
	}()
	intent, e := fiscal.ReadIntent(ctx, a.DB, a.Actor, a.Device, op, true)
	if errors.Is(e, fiscal.ErrNotFound) {
		if err = r.ValidateCheckoutHomologation(ctx, a, 2); err != nil {
			return
		}
		intent, err = fiscal.Prepare(ctx, a.DB, a.License, a.Actor, a.Device, fiscal.IntentInput{OperationID: op, SaleID: saleID, ProfileID: r.binding.ProfileID, ProfileRevision: r.binding.ProfileRevision}, a.Token)
	} else {
		err = e
	}
	if err != nil {
		return
	}
	state.IntentID = intent.ID
	a.IntentID = intent.ID
	if intent.Snapshot.Profile.Environment != "homologation" || intent.Snapshot.Profile.Model != "65" {
		err = ErrCheckoutProduction
		return
	}
	n, e := fiscal.ReadHomologationNumber(ctx, a.DB, a.Actor, a.Device, intent.ID, a.Token)
	if errors.Is(e, fiscal.ErrNotFound) {
		zone, e := time.LoadLocation(r.binding.Timezone)
		if e != nil {
			err = e
			return
		}
		n, err = fiscal.AllocateHomologationNumber(ctx, a.DB, a.License, a.Actor, a.Device, intent.ID, fiscal.NumberInput{OperationID: op, IssuedAt: time.Now().In(zone)}, a.Token)
	} else {
		err = e
	}
	if err != nil {
		return
	}
	state.Key = n.AccessKey
	recorder, e := fiscal.NewHomologationRecorder(ctx, a.DB, a.License, a.Actor, a.Device, intent.ID, r.binding.AuthorizationURL, n.AccessKey, a.Token)
	if e != nil {
		err = e
		return
	}
	recovered, e := fiscalcore.RecoverHomologationNFe(ctx, recorder)
	if !errors.Is(e, fiscal.ErrNotFound) {
		err = e
		state.State = recovered.State
		if e == nil && len(recovered.ProcessedXML) > 0 {
			state.State = "authorized_homologation"
		}
		return
	}
	if err = r.ValidateCheckoutHomologation(ctx, a, 2); err != nil {
		return
	}
	in, err = fiscal.CheckoutHomologationInvoice(ctx, a.DB, a.Actor, a.Device, intent.ID, a.Token, n, in)
	if err != nil {
		return
	}
	client, e := fiscalcore.NewClientWithServerTrust(r.binding.AuthorizationURL, recorder.Scope(), r.credential, &checkoutGuard{recorder, r}, r.network.ServerRoots, r.network.DialContext)
	if e != nil {
		err = e
		return
	}
	defer client.Close()
	evidence, e := client.IssueHomologationNFe(ctx, op, "1", in)
	err = e
	state.State = evidence.State
	if e == nil && len(evidence.ProcessedXML) > 0 {
		state.State = "authorized_homologation"
	}
	return
}
