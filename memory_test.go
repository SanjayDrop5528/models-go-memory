package memory_test

import (
	"context"
	"testing"

	"github.com/SanjayDrop5528/models-go-engine/adapter"
	"github.com/SanjayDrop5528/models-go-engine/crud"
	"github.com/SanjayDrop5528/models-go-engine/model"
	"github.com/SanjayDrop5528/models-go-engine/query"
	"github.com/SanjayDrop5528/models-go-memory"
)

type memoryModelResolver map[string]*model.Model

func (r memoryModelResolver) GetActive(_ context.Context, id string) (*model.Model, error) {
	return r[id], nil
}

func TestMemory_CRUDEngineHydratesRelations(t *testing.T) {
	ctx := context.Background()
	store := memory.NewMemoryAdapter()
	registry := adapter.NewRegistry()
	registry.Register("memory", store)

	customer := &model.Model{ID: "customer", Name: "Customer", StorageName: "customers", Database: "memory", PrimaryKey: &model.PrimaryKey{Columns: []string{"id"}}}
	line := &model.Model{ID: "line", Name: "Line", StorageName: "lines", Database: "memory", PrimaryKey: &model.PrimaryKey{Columns: []string{"id"}}}
	order := &model.Model{
		ID: "order", Name: "Order", StorageName: "orders", Database: "memory", PrimaryKey: &model.PrimaryKey{Columns: []string{"id"}},
		Relations: []model.Relation{
			{Name: "Customer", Type: model.RelManyToOne, TargetModel: "customer", ForeignKey: "customer_id", TargetKey: "id", LoadWithChildren: true},
			{Name: "Lines", Type: model.RelOneToMany, TargetModel: "line", ForeignKey: "order_id", TargetKey: "id", LoadWithChildren: true},
		},
	}
	_, _ = store.Create(ctx, customer.Ref(), map[string]any{"id": "c1", "name": "Ada"})
	_, _ = store.Create(ctx, order.Ref(), map[string]any{"id": "o1", "customer_id": "c1"})
	_, _ = store.Create(ctx, line.Ref(), map[string]any{"id": "l1", "order_id": "o1"})

	engine := crud.NewEngine(registry)
	engine.SetModelResolver(memoryModelResolver{"customer": customer, "line": line})
	rows, _, err := engine.Find(ctx, order, query.New())
	if err != nil {
		t.Fatal(err)
	}
	if rows[0]["Customer"].(map[string]any)["name"] != "Ada" || len(rows[0]["Lines"].([]map[string]any)) != 1 {
		t.Fatalf("relations were not hydrated: %+v", rows[0])
	}
}

func TestMemory_UnifiedQuery(t *testing.T) {
	ctx := context.Background()
	adp := memory.NewMemoryAdapter()

	ref := model.ModelRef{Name: "users", StorageName: "users"}

	// Insert test records
	_, _ = adp.Create(ctx, ref, map[string]any{"id": 1, "name": "Alice", "status": "active", "age": 25, "secret": "s1"})
	_, _ = adp.Create(ctx, ref, map[string]any{"id": 2, "name": "Bob", "status": "inactive", "age": 30, "secret": "s2"})
	_, _ = adp.Create(ctx, ref, map[string]any{"id": 3, "name": "Charlie", "status": "active", "age": 35, "secret": "s3"})

	// Query using unified query builder
	q := query.New().
		Table("users").
		Where("status = ?", "active").
		Where("age >= ?", 26).
		Column("id", "name", "age").
		ExcludeColumn("secret").
		OrderBy("age", query.SortDesc).
		Limit(10)

	results, total, err := adp.Find(ctx, ref, q)
	if err != nil {
		t.Fatalf("Find failed: %v", err)
	}

	if total != 1 || len(results) != 1 {
		t.Fatalf("expected 1 result, got total=%d len=%d", total, len(results))
	}

	row := results[0]
	if row["name"] != "Charlie" {
		t.Fatalf("expected Charlie, got %v", row["name"])
	}
	if _, hasSecret := row["secret"]; hasSecret {
		t.Fatalf("expected secret to be excluded, but was present")
	}
}
