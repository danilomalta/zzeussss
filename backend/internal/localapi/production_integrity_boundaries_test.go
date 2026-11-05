package localapi

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"

	"github.com/gofiber/fiber/v2"
	"titansystem-backend/internal/apicontract"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/localauth"
	"titansystem-backend/internal/localdb/production"
)

// The admission seam exists only here: it models logout/context replacement
// after HTTP middleware resolved a session but before the command gets its lock.
func TestProductionStaleAdmissionCannotMutate(t *testing.T) {
	for _, action := range []string{"reserve", "consume", "release", "quality"} {
		for _, mode := range []string{"logout", "foreign-token"} {
			t.Run(action+"/"+mode, func(t *testing.T) {
				var f *httpContractFixture
				var body, path string
				var handler fiber.Handler
				if action == "quality" {
					var in production.QualityInput
					f, in, _ = qualityFixture(t)
					path = qualityPath
					body = orderBody(t, in)
					handler = productionBoundaryServer(f).recordProductionQuality
				} else {
					var in production.ReserveInput
					f, in = materialsFixture(t)
					path = materialsPath
					body = orderBody(t, in)
					handler = productionBoundaryServer(f).reserveProductionMaterials
					if action != "reserve" {
						requireMaterials(t, f, "POST", materialsPath, in, 201)
						path = materialsPath + "/state"
						body = orderBody(t, production.MaterialChangeInput{OperationID: "p16-change", ReservationID: in.ReservationID, Action: action, Reason: "Synthetic"})
						handler = productionBoundaryServer(f).changeProductionMaterials
					}
				}
				snapshot := func() []int64 {
					t.Helper()
					var out []int64
					for _, table := range []string{"stock_movements", "production_material_reservations", "production_material_events", "production_quality_reviews", "outbox"} {
						var n int64
						if e := f.db.QueryRow("SELECT count(*) FROM " + table).Scan(&n); e != nil {
							t.Fatal(e)
						}
						out = append(out, n)
					}
					return out
				}
				before := snapshot()
				captured, e := localauth.Resolve(context.Background(), f.db, f.token)
				if e != nil {
					t.Fatal(e)
				}
				if mode == "logout" {
					if e = localauth.Logout(context.Background(), f.db, f.token); e != nil {
						t.Fatal(e)
					}
				} else {
					captured.Token = httpContractSetup(t).token
				}
				app := fiber.New(fiber.Config{ErrorHandler: apicontract.FrameworkError})
				app.Post(path, func(c *fiber.Ctx) error { c.Locals("session", captured); return handler(c) })
				if code, _ := request(t, app, "POST", path, body, f.token); code != 401 {
					t.Errorf("stale admitted %s: %d", action, code)
				}
				if !reflect.DeepEqual(before, snapshot()) {
					t.Error("stale command changed materials, stock, quality or outbox")
				}
			})
		}
	}
}

func productionBoundaryServer(f *httpContractFixture) *Server {
	return &Server{DB: f.db, Device: f.device, contracts: retailServiceFor(f).Contracts}
}

func TestProductionOtherWritesRejectStaleAdmission(t *testing.T) {
	for _, action := range []string{"publish", "recipe-state", "order", "order-state", "stage-plan", "stage-state", "result", "lot", "void-lot", "loss", "void-loss"} {
		for _, mode := range []string{"logout", "foreign-token"} {
			t.Run(action+"/"+mode, func(t *testing.T) {
				var f *httpContractFixture
				var input any
				var path string
				var handler fiber.Handler
				switch action {
				case "publish":
					var in production.PublishInput
					f, in = recipeFixture(t)
					input = in
					path = recipePath
					handler = productionBoundaryServer(f).publishRecipe
				case "recipe-state":
					f, _ = orderFixture(t)
					input = production.RecipeStateInput{OperationID: "deactivate", RecipeID: "bread-recipe", ExpectedRevision: 0, Status: "inactive", Reason: "Synthetic"}
					path = "/local/v1/production/recipes/bread-recipe/state"
					handler = productionBoundaryServer(f).changeRecipeState
				case "order":
					var in production.OrderInput
					f, in = orderFixture(t)
					input = in
					path = orderPath
					handler = productionBoundaryServer(f).createProductionOrder
				case "order-state":
					var in production.OrderInput
					f, in = orderFixture(t)
					requireMaterials(t, f, "POST", orderPath, in, 201)
					input = production.OrderStateInput{OperationID: "approve", OrderID: in.OrderID, ExpectedRevision: 1, Status: "approved", Reason: "Synthetic"}
					path = orderPath + "/state"
					handler = productionBoundaryServer(f).changeProductionOrder
				case "stage-plan", "stage-state":
					var in production.StagePlanInput
					var reserve production.ReserveInput
					f, in, reserve = stagesFixture(t)
					input = in
					path = stagePlansPath
					handler = productionBoundaryServer(f).configureProductionStages
					if action == "stage-state" {
						requireMaterials(t, f, "POST", stagePlansPath, in, 201)
						consumeStages(t, f, reserve)
						input = production.StageStateInput{OperationID: "start", OrderID: in.OrderID, StageID: "mix", ExpectedRevision: 1, Status: "running", Reason: "Synthetic"}
						path = stageStatePath
						handler = productionBoundaryServer(f).changeProductionStage
					}
				case "result":
					var in production.ResultInput
					f, in = resultsFixture(t)
					input = in
					path = resultsPath
					handler = productionBoundaryServer(f).completeProduction
				case "lot", "void-lot":
					var in production.LotInput
					f, in = lotsFixture(t)
					input = in
					path = lotsPath
					handler = productionBoundaryServer(f).recordProductionLot
					if action == "void-lot" {
						requireMaterials(t, f, "POST", lotsPath, in, 201)
						input = production.VoidLotInput{OperationID: "void", LotID: in.LotID, ExpectedRevision: 1, Reason: "Synthetic"}
						path = lotsPath + "/void"
						handler = productionBoundaryServer(f).voidProductionLot
					}
				case "loss", "void-loss":
					var in production.LossInput
					f, in = lossesFixture(t)
					input = in
					path = lossesPath
					handler = productionBoundaryServer(f).recordProductionLoss
					if action == "void-loss" {
						requireMaterials(t, f, "POST", lossesPath, in, 201)
						input = production.VoidLossInput{OperationID: "void", LossID: in.LossID, ExpectedRevision: 1, Reason: "Synthetic"}
						path = lossesPath + "/void"
						handler = productionBoundaryServer(f).voidProductionLoss
					}
				}
				var before, after int
				if e := f.db.QueryRow(`SELECT count(*) FROM outbox`).Scan(&before); e != nil {
					t.Fatal(e)
				}
				captured, e := localauth.Resolve(context.Background(), f.db, f.token)
				if e != nil {
					t.Fatal(e)
				}
				if mode == "logout" {
					if e = localauth.Logout(context.Background(), f.db, f.token); e != nil {
						t.Fatal(e)
					}
				} else {
					captured.Token = httpContractSetup(t).token
				}
				app := fiber.New(fiber.Config{ErrorHandler: apicontract.FrameworkError})
				app.Post(path, func(c *fiber.Ctx) error { c.Locals("session", captured); return handler(c) })
				if code, _ := request(t, app, "POST", path, orderBody(t, input), f.token); code != 401 {
					t.Errorf("stale admitted %s: %d", action, code)
				}
				if e = f.db.QueryRow(`SELECT count(*) FROM outbox`).Scan(&after); e != nil || before != after {
					t.Fatal("stale production event", before, after, e)
				}
				// A fresh admitted session must execute this same valid command.
				// This prevents a malformed fixture from masquerading as auth proof.
				if mode == "logout" {
					f.token = loginToken(t, f.app, f.owner.OwnerID)
				}
				captured, e = localauth.Resolve(context.Background(), f.db, f.token)
				if e != nil {
					t.Fatal(e)
				}
				want := 201
				switch action {
				case "recipe-state", "order-state", "stage-state", "void-lot", "void-loss":
					want = 200
				}
				if code, raw := request(t, app, "POST", path, orderBody(t, input), f.token); code != want {
					t.Fatalf("fresh command %s: %d want %d %s", action, code, want, raw)
				}

			})
		}
	}
}

func TestProductionConcurrentConsumeReleaseIndependentConnections(t *testing.T) {
	f, reserve := materialsFixture(t)
	requireMaterials(t, f, "POST", materialsPath, reserve, 201)
	ctx := context.Background()
	var file string
	if e := f.db.QueryRow(`SELECT file FROM pragma_database_list WHERE name='main'`).Scan(&file); e != nil {
		t.Fatal(e)
	}
	second, e := localdb.Open(ctx, file)
	if e != nil {
		t.Fatal(e)
	}
	defer second.Close()
	type result struct {
		in  production.MaterialChangeInput
		err error
	}
	results := make(chan result, 2)
	var wg sync.WaitGroup
	ready := make(chan struct{})
	for i, action := range []string{"consume", "release"} {
		db := f.db
		if i == 1 {
			db = second
		}
		in := production.MaterialChangeInput{OperationID: "compete-" + action, ReservationID: reserve.ReservationID, Action: action, Reason: "Synthetic"}
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-ready
			_, err := production.ChangeMaterials(ctx, db, productionBoundaryServer(f).contracts, materialsActor(f), f.device, in, f.token)
			results <- result{in, err}
		}()
	}
	close(ready)
	wg.Wait()
	close(results)
	wins := 0
	var winner production.MaterialChangeInput
	for r := range results {
		if r.err == nil {
			wins++
			winner = r.in
		} else if !errors.Is(r.err, production.ErrMaterials) && !errors.Is(r.err, production.ErrConflict) {
			t.Fatal(r.err)
		}
	}
	if wins != 1 {
		t.Fatal("winners", wins)
	}
	v, e := production.ChangeMaterials(ctx, second, productionBoundaryServer(f).contracts, materialsActor(f), f.device, winner, f.token)
	if e != nil || !v.Repeated {
		t.Fatal("recovery lost receipt", v, e)
	}
	wantMovements, wantBindings, status := 2, 0, "released"
	if winner.Action == "consume" {
		wantMovements, wantBindings, status = 4, 2, "consumed"
	}
	materialCount(t, f, "stock_movements", wantMovements)
	materialCount(t, f, "production_material_movements", wantBindings)
	materialCount(t, f, "production_material_events", 2)
	var stored string
	if e = f.db.QueryRow(`SELECT status FROM production_material_reservations WHERE id=?`, reserve.ReservationID).Scan(&stored); e != nil || stored != status {
		t.Fatal("mixed state", stored, e)
	}
}
