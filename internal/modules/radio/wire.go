package radio

import (
	"context"
	"infradesktop/internal/modules/equipment"
	"infradesktop/internal/modules/inventory"
	"infradesktop/internal/modules/monitoring"
	"infradesktop/internal/modules/pppoe"
	"time"
)

type compositeService struct {
	inv *inventory.Service
	pp  *pppoe.Service
	rad *Service
	mon *monitoring.Service
}

func NewComposite(
	inv *inventory.Service, pp *pppoe.Service, rad *Service, mon *monitoring.Service,
) equipment.Handler {
	return &compositeService{inv: inv, pp: pp, rad: rad, mon: mon}
}

func (c *compositeService) Handle(ctx context.Context, q Command) (View, Code) {
	return c.inv.Handle(ctx, q)
}

func (c *compositeService) HandlePP(ctx context.Context, q Command, in PPInput) (PPView, Code) {
	return c.pp.HandlePP(ctx, q, in)
}

func (c *compositeService) HandleRadio(
	ctx context.Context, q Command, in RadioInput,
) (RadioView, Code) {
	return c.rad.HandleRadio(ctx, q, in)
}

func (c *compositeService) VaultAvailable() bool {
	return c.inv.VaultAvailable()
}

func (c *compositeService) Close() {
	c.inv.Close()
}

func init() {
	equipment.DefaultFactory = func(
		auth Authorizer, store Store, vault Vault, remote Remote, ids IDs, now func() time.Time,
	) equipment.Handler {
		mon := monitoring.New(auth, store)
		inv := inventory.New(auth, store, vault, remote, ids, now)
		pp := pppoe.New(inv, mon)
		rad := New(inv)
		return NewComposite(inv, pp, rad, mon)
	}
	equipment.ComparePP = pppoe.ComparePP
	equipment.ReconcilePP = pppoe.ReconcilePP
	equipment.ReconcileCatalogAbsences = pppoe.ReconcileCatalogAbsences
	equipment.ComparableConcentratorReadings = pppoe.ComparableConcentratorReadings
	equipment.AssociatePPTunnels = pppoe.AssociatePPTunnels
	equipment.MatchPPRead = pppoe.MatchPPRead
	equipment.PPBandwidth = pppoe.PPBandwidth
	equipment.SortPPConsumption = pppoe.SortPPConsumption
	equipment.PPReadScope = pppoe.PPReadScope
	equipment.PPReadBudget = pppoe.PPReadBudget
	equipment.PPReadWithinBudget = pppoe.PPReadWithinBudget
	equipment.PPMonitoringContinuous = monitoring.PPMonitoringContinuous
	equipment.PPMonitoringDrop = monitoring.PPMonitoringDrop
	equipment.MonitoringRetention = monitoring.MonitoringRetention
	equipment.DetectedDeviceReboot = monitoring.DetectedDeviceReboot
	equipment.PortNegotiationChanged = monitoring.PortNegotiationChanged
	equipment.StampRadio = StampRadio
}
