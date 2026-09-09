package memory_test

import (
	"context"
	"testing"

	"github.com/SanjayDrop5528/models-go-engine/model"
	"github.com/SanjayDrop5528/models-go-engine/query"
	"github.com/SanjayDrop5528/models-go-memory"
)

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
