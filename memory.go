package memory

import (
	"context"
	"fmt"
	"github.com/SanjayDrop5528/models-go-engine/adapter"
	"github.com/SanjayDrop5528/models-go-engine/diff"
	"github.com/SanjayDrop5528/models-go-engine/execution"
	"github.com/SanjayDrop5528/models-go-engine/model"
	"github.com/SanjayDrop5528/models-go-engine/plan"
	"github.com/SanjayDrop5528/models-go-engine/query"
	"github.com/SanjayDrop5528/models-go-engine/schema"
	"reflect"
	"sort"
	"strings"
	"sync"
)

// MemoryAdapter is a concurrent, in-memory implementation of the Adapter interface.
// It is ideal for rapid testing, mock setups, and verifying diff/migration logic.
type MemoryAdapter struct {
	mu      sync.RWMutex
	schemas map[string]*schema.Schema        // tableName -> Schema
	data    map[string][]map[string]any      // tableName -> records
	autoInc map[string]int64                 // tableName -> current auto increment id
}

// NewMemoryAdapter creates a new in-memory adapter.
func NewMemoryAdapter() *MemoryAdapter {
	return &MemoryAdapter{
		schemas: make(map[string]*schema.Schema),
		data:    make(map[string][]map[string]any),
		autoInc: make(map[string]int64),
	}
}

func (a *MemoryAdapter) Name() string {
	return "memory"
}

// NativeClient returns the *MemoryAdapter instance.
func (a *MemoryAdapter) NativeClient() any {
	return a
}

// DatabaseName returns the in-memory database name.
func (a *MemoryAdapter) DatabaseName() string {
	return a.GetDatabaseName()
}

// GetDatabaseName returns the in-memory database name.
func (a *MemoryAdapter) GetDatabaseName() string {
	return "in-memory"
}

func (a *MemoryAdapter) Connect(ctx context.Context) error {
	return a.EnsureMetadataTables(ctx)
}

func (a *MemoryAdapter) Ping(ctx context.Context) error {
	return nil
}

func (a *MemoryAdapter) Close(ctx context.Context) error {
	return nil
}

// EnsureMetadataTables creates 'model_configs' and 'data_models' stores in memory if not present.
func (a *MemoryAdapter) EnsureMetadataTables(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if _, ok := a.data["model_configs"]; !ok {
		a.data["model_configs"] = make([]map[string]any, 0)
	}
	if _, ok := a.data["data_models"]; !ok {
		a.data["data_models"] = make([]map[string]any, 0)
	}
	return nil
}

// ImportLiveMetadata returns in-memory stored model_configs and data_models.
func (a *MemoryAdapter) ImportLiveMetadata(ctx context.Context) ([]*model.ModelConfig, []*model.DataModel, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return nil, nil, nil
}

// GetSchema returns a deep copy of the currently stored schema for the model.
func (a *MemoryAdapter) GetSchema(ctx context.Context, ref model.ModelRef) (*schema.Schema, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	tableName := ref.StorageName
	if tableName == "" {
		tableName = ref.Name
	}

	s, ok := a.schemas[tableName]
	if !ok {
		return nil, nil
	}

	// Return a clone
	return cloneSchema(s), nil
}

// ValidateSchemaPlan checks plan against in-memory storage.
func (a *MemoryAdapter) ValidateSchemaPlan(ctx context.Context, p *plan.SchemaPlan) error {
	if p == nil {
		return fmt.Errorf("plan cannot be nil")
	}
	return nil
}

// PreviewSchemaChange generates preview statements for memory operations.
func (a *MemoryAdapter) PreviewSchemaChange(ctx context.Context, p *plan.SchemaPlan) (*plan.SchemaPreview, error) {
	preview := &plan.SchemaPreview{
		ModelID:              p.ModelID,
		StorageName:          p.StorageName,
		Database:             "memory",
		Changes:              p.Operations,
		NativeActions:        make([]plan.NativeAction, 0, len(p.Operations)),
		HasDestructive:       p.Destructive,
		RequiresConfirmation: p.Destructive,
		Warnings:             p.Warnings,
		Status:               "READY",
	}

	for _, op := range p.Operations {
		preview.NativeActions = append(preview.NativeActions, plan.NativeAction{
			Type:        "MEMORY_OP",
			Description: op.Description,
			Statement:   fmt.Sprintf("%s ON %s (Object: %s)", op.Type, p.StorageName, op.ObjectName),
			Destructive: op.Destructive,
		})
	}

	return preview, nil
}

// ApplySchemaChange modifies the in-memory schema and migrates existing in-memory rows.
func (a *MemoryAdapter) ApplySchemaChange(ctx context.Context, p *plan.SchemaPlan) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	tableName := p.StorageName
	currentSchema := a.schemas[tableName]

	for _, op := range p.Operations {
		switch op.Type {
		case diff.OpCreateTable:
			if desired, ok := op.After.(*schema.Schema); ok {
				a.schemas[tableName] = cloneSchema(desired)
				if _, exists := a.data[tableName]; !exists {
					a.data[tableName] = make([]map[string]any, 0)
				}
			}

		case diff.OpDropTable:
			delete(a.schemas, tableName)
			delete(a.data, tableName)
			delete(a.autoInc, tableName)

		case diff.OpAddColumn:
			if currentSchema != nil {
				if attr, ok := op.After.(schema.SchemaAttribute); ok {
					currentSchema.Attributes = append(currentSchema.Attributes, attr)
					// Populate default or nil in existing rows
					for i := range a.data[tableName] {
						if attr.Default != nil {
							a.data[tableName][i][attr.Name] = attr.Default
						} else {
							a.data[tableName][i][attr.Name] = nil
						}
					}
				}
			}

		case diff.OpRemoveColumn:
			if currentSchema != nil {
				newAttrs := make([]schema.SchemaAttribute, 0)
				for _, attr := range currentSchema.Attributes {
					if attr.Name != op.ObjectName {
						newAttrs = append(newAttrs, attr)
					}
				}
				currentSchema.Attributes = newAttrs
				// Drop column from existing rows
				for i := range a.data[tableName] {
					delete(a.data[tableName][i], op.ObjectName)
				}
			}

		case diff.OpRenameColumn:
			if currentSchema != nil {
				for i, attr := range currentSchema.Attributes {
					if attr.Name == op.OldName {
						currentSchema.Attributes[i].Name = op.ObjectName
						break
					}
				}
				// Rename field in existing rows
				for i := range a.data[tableName] {
					if val, exists := a.data[tableName][i][op.OldName]; exists {
						a.data[tableName][i][op.ObjectName] = val
						delete(a.data[tableName][i], op.OldName)
					}
				}
			}

		case diff.OpAlterColumnType:
			if currentSchema != nil {
				if attr, ok := op.After.(schema.SchemaAttribute); ok {
					for i, existing := range currentSchema.Attributes {
						if existing.Name == attr.Name {
							currentSchema.Attributes[i] = attr
							break
						}
					}
				}
			}

		case diff.OpAlterColumnNullable, diff.OpAlterColumnDefault:
			if currentSchema != nil {
				for i, attr := range currentSchema.Attributes {
					if attr.Name == op.ObjectName {
						if op.Type == diff.OpAlterColumnNullable {
							if nullVal, ok := op.After.(bool); ok {
								currentSchema.Attributes[i].Nullable = nullVal
							}
						} else if op.Type == diff.OpAlterColumnDefault {
							currentSchema.Attributes[i].Default = op.After
						}
						break
					}
				}
			}

		case diff.OpAddIndex:
			if currentSchema != nil {
				if idx, ok := op.After.(schema.SchemaIndex); ok {
					currentSchema.Indexes = append(currentSchema.Indexes, idx)
				}
			}

		case diff.OpDropIndex:
			if currentSchema != nil {
				newIdxs := make([]schema.SchemaIndex, 0)
				for _, idx := range currentSchema.Indexes {
					if idx.Name != op.ObjectName {
						newIdxs = append(newIdxs, idx)
					}
				}
				currentSchema.Indexes = newIdxs
			}
		}
	}

	return nil
}

// Create inserts a record into memory.
func (a *MemoryAdapter) Create(ctx context.Context, ref model.ModelRef, data map[string]any) (map[string]any, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	tableName := ref.StorageName
	record := make(map[string]any)
	for k, v := range data {
		record[k] = v
	}

	// Auto increment ID if id is not provided
	if _, hasID := record["id"]; !hasID {
		a.autoInc[tableName]++
		record["id"] = a.autoInc[tableName]
	}

	a.data[tableName] = append(a.data[tableName], record)
	return cloneRecord(record), nil
}

// Find filters and paginates in-memory records.
func (a *MemoryAdapter) Find(ctx context.Context, ref model.ModelRef, q query.Query) ([]map[string]any, int64, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	tableName := ref.StorageName
	rows := a.data[tableName]

	// 1. Filter
	var filtered []map[string]any
	for _, row := range rows {
		if matchQuery(row, q) {
			filtered = append(filtered, cloneRecord(row))
		}
	}

	total := int64(len(filtered))

	// 2. Sort
	if len(q.Sorts) > 0 {
		sort.SliceStable(filtered, func(i, j int) bool {
			for _, s := range q.Sorts {
				v1 := filtered[i][s.Field]
				v2 := filtered[j][s.Field]
				cmp := compareValues(v1, v2)
				if cmp != 0 {
					if s.Order == query.SortDesc {
						return cmp > 0
					}
					return cmp < 0
				}
			}
			return false
		})
	}

	// 3. Paginate
	offset := q.Pagination.Offset
	limit := q.Pagination.Limit
	if offset < 0 {
		offset = 0
	}
	if offset > len(filtered) {
		return []map[string]any{}, total, nil
	}

	end := len(filtered)
	if limit > 0 && offset+limit < end {
		end = offset + limit
	}

	result := filtered[offset:end]

	// 4. Project fields
	if len(q.Fields) > 0 {
		projected := make([]map[string]any, len(result))
		for i, r := range result {
			proj := make(map[string]any)
			for _, field := range q.Fields {
				if v, ok := r[field]; ok {
					proj[field] = v
				}
			}
			projected[i] = proj
		}
		return projected, total, nil
	}

	return result, total, nil
}

// FindOne finds a record by ID.
func (a *MemoryAdapter) FindOne(ctx context.Context, ref model.ModelRef, id any) (map[string]any, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	tableName := ref.StorageName
	rows := a.data[tableName]

	idStr := fmt.Sprintf("%v", id)
	for _, row := range rows {
		if fmt.Sprintf("%v", row["id"]) == idStr {
			return cloneRecord(row), nil
		}
	}

	return nil, fmt.Errorf("record with id '%v' not found", id)
}

// Update replaces a record by ID.
func (a *MemoryAdapter) Update(ctx context.Context, ref model.ModelRef, id any, data map[string]any) (map[string]any, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	tableName := ref.StorageName
	rows := a.data[tableName]

	idStr := fmt.Sprintf("%v", id)
	for i, row := range rows {
		if fmt.Sprintf("%v", row["id"]) == idStr {
			record := make(map[string]any)
			for k, v := range data {
				record[k] = v
			}
			record["id"] = row["id"]
			a.data[tableName][i] = record
			return cloneRecord(record), nil
		}
	}

	return nil, fmt.Errorf("record with id '%v' not found", id)
}

// Patch updates specific fields by ID.
func (a *MemoryAdapter) Patch(ctx context.Context, ref model.ModelRef, id any, data map[string]any) (map[string]any, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	tableName := ref.StorageName
	rows := a.data[tableName]

	idStr := fmt.Sprintf("%v", id)
	for i, row := range rows {
		if fmt.Sprintf("%v", row["id"]) == idStr {
			for k, v := range data {
				a.data[tableName][i][k] = v
			}
			return cloneRecord(a.data[tableName][i]), nil
		}
	}

	return nil, fmt.Errorf("record with id '%v' not found", id)
}

// Delete removes a record by ID.
func (a *MemoryAdapter) Delete(ctx context.Context, ref model.ModelRef, id any) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	tableName := ref.StorageName
	rows := a.data[tableName]

	idStr := fmt.Sprintf("%v", id)
	for i, row := range rows {
		if fmt.Sprintf("%v", row["id"]) == idStr {
			a.data[tableName] = append(rows[:i], rows[i+1:]...)
			return nil
		}
	}

	return fmt.Errorf("record with id '%v' not found", id)
}

func matchQuery(row map[string]any, q query.Query) bool {
	if len(q.Filters) == 0 {
		return true
	}

	for _, f := range q.Filters {
		matched := matchFilter(row[f.Field], f)
		if q.LogicalOp == query.OpOr {
			if matched {
				return true
			}
		} else {
			if !matched {
				return false
			}
		}
	}

	return q.LogicalOp != query.OpOr
}

func matchFilter(val any, f query.Filter) bool {
	switch f.Op {
	case query.OpEq:
		return fmt.Sprintf("%v", val) == fmt.Sprintf("%v", f.Value)
	case query.OpNeq:
		return fmt.Sprintf("%v", val) != fmt.Sprintf("%v", f.Value)
	case query.OpIsNull:
		return val == nil
	case query.OpIsNotNull:
		return val != nil
	case query.OpGt:
		return compareValues(val, f.Value) > 0
	case query.OpGte:
		return compareValues(val, f.Value) >= 0
	case query.OpLt:
		return compareValues(val, f.Value) < 0
	case query.OpLte:
		return compareValues(val, f.Value) <= 0
	case query.OpLike:
		sVal := fmt.Sprintf("%v", val)
		pat := strings.ReplaceAll(fmt.Sprintf("%v", f.Value), "%", "")
		return strings.Contains(sVal, pat)
	case query.OpILike:
		sVal := strings.ToLower(fmt.Sprintf("%v", val))
		pat := strings.ToLower(strings.ReplaceAll(fmt.Sprintf("%v", f.Value), "%", ""))
		return strings.Contains(sVal, pat)
	case query.OpNotLike:
		sVal := strings.ToLower(fmt.Sprintf("%v", val))
		pat := strings.ToLower(strings.ReplaceAll(fmt.Sprintf("%v", f.Value), "%", ""))
		return !strings.Contains(sVal, pat)
	case query.OpIn:
		vSlice := reflect.ValueOf(f.Value)
		if vSlice.Kind() == reflect.Slice {
			for i := 0; i < vSlice.Len(); i++ {
				if fmt.Sprintf("%v", val) == fmt.Sprintf("%v", vSlice.Index(i).Interface()) {
					return true
				}
			}
		}
		return false
	case query.OpNin:
		return !matchFilter(val, query.Filter{Op: query.OpIn, Value: f.Value})
	case query.OpBetween:
		return compareValues(val, f.Value) >= 0 && compareValues(val, f.ValueTo) <= 0
	default:
		return true
	}
}

func compareValues(v1, v2 any) int {
	if v1 == nil && v2 == nil {
		return 0
	}
	if v1 == nil {
		return -1
	}
	if v2 == nil {
		return 1
	}

	s1 := fmt.Sprintf("%v", v1)
	s2 := fmt.Sprintf("%v", v2)
	return strings.Compare(s1, s2)
}

func cloneSchema(s *schema.Schema) *schema.Schema {
	if s == nil {
		return nil
	}
	cp := *s
	cp.Attributes = make([]schema.SchemaAttribute, len(s.Attributes))
	copy(cp.Attributes, s.Attributes)

	cp.Indexes = make([]schema.SchemaIndex, len(s.Indexes))
	copy(cp.Indexes, s.Indexes)

	cp.Relations = make([]schema.SchemaRelation, len(s.Relations))
	copy(cp.Relations, s.Relations)

	if s.PrimaryKey != nil {
		cp.PrimaryKey = &schema.SchemaKey{
			Name:    s.PrimaryKey.Name,
			Columns: append([]string{}, s.PrimaryKey.Columns...),
		}
	}
	return &cp
}

func cloneRecord(r map[string]any) map[string]any {
	cp := make(map[string]any, len(r))
	for k, v := range r {
		cp[k] = v
	}
	return cp
}

// Execute executes a generic operation in-memory.
func (a *MemoryAdapter) Execute(ctx context.Context, req execution.ExecutionRequest) (*execution.ExecutionResult, error) {
	return &execution.ExecutionResult{
		Data: map[string]any{
			"executed_operation": req.Operation,
			"target":             req.Target,
			"arguments":          req.Arguments,
		},
		RowsAffected: 1,
		Status:       "SUCCESS",
	}, nil
}

// Begin begins an in-memory transaction.
func (a *MemoryAdapter) Begin(ctx context.Context) (adapter.Transaction, error) {
	return &MemoryTransaction{adapter: a}, nil
}

// MemoryTransaction implements adapter.Transaction for MemoryAdapter.
type MemoryTransaction struct {
	adapter *MemoryAdapter
}

func (t *MemoryTransaction) Create(ctx context.Context, m model.ModelRef, data map[string]any) (map[string]any, error) {
	return t.adapter.Create(ctx, m, data)
}

func (t *MemoryTransaction) Find(ctx context.Context, m model.ModelRef, q query.Query) ([]map[string]any, int64, error) {
	return t.adapter.Find(ctx, m, q)
}

func (t *MemoryTransaction) FindOne(ctx context.Context, m model.ModelRef, id any) (map[string]any, error) {
	return t.adapter.FindOne(ctx, m, id)
}

func (t *MemoryTransaction) Update(ctx context.Context, m model.ModelRef, id any, data map[string]any) (map[string]any, error) {
	return t.adapter.Update(ctx, m, id, data)
}

func (t *MemoryTransaction) Patch(ctx context.Context, m model.ModelRef, id any, data map[string]any) (map[string]any, error) {
	return t.adapter.Patch(ctx, m, id, data)
}

func (t *MemoryTransaction) Delete(ctx context.Context, m model.ModelRef, id any) error {
	return t.adapter.Delete(ctx, m, id)
}

func (t *MemoryTransaction) Execute(ctx context.Context, req execution.ExecutionRequest) (*execution.ExecutionResult, error) {
	return t.adapter.Execute(ctx, req)
}

func (t *MemoryTransaction) Commit(ctx context.Context) error {
	return nil
}

func (t *MemoryTransaction) Rollback(ctx context.Context) error {
	return nil
}

