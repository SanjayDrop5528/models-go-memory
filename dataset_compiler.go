// Package memory provides an in-memory, thread-safe storage adapter, query engine,
// and Dataset Studio compiler for testing, local prototyping, and mock environments.
//
// File: dataset_compiler.go
// Usage:
//
//	This file implements the MemoryDataSetCompiler, translating QueryAST plans into
//	portable ANSI SQL expressions for execution in in-memory or generic database environments.
package memory

import (
	"context"
	"fmt"
	"strings"

	"github.com/SanjayDrop5528/models-go-engine/adapter"
	"github.com/SanjayDrop5528/models-go-engine/dataset/compiler"
	"github.com/SanjayDrop5528/models-go-engine/dataset/domain"
	"github.com/SanjayDrop5528/models-go-engine/dataset/planner"
)

// MemoryDataSetCompiler compiles QueryAST for in-memory / generic SQL execution.
type MemoryDataSetCompiler struct{}

// NewMemoryDataSetCompiler creates a new Memory dataset compiler instance.
//
// Purpose:
//
//	Initializes a MemoryDataSetCompiler for in-memory dataset compilation.
//
// Where it is used:
//   - Instantiated in MemoryAdapter.DataSetCompiler and standalone unit tests.
//
// When can it be used:
//   - When compiling dataset pipelines for in-memory execution or offline previews.
//
// Parameters:
//   - None.
//
// Returns:
//   - result1 (*MemoryDataSetCompiler): Returned value.
//
// Business Rules:
//   - Follows the invariants documented by NewMemoryDataSetCompiler and its owning package.
//
// Side Effects:
//   - Does not intentionally mutate caller-owned input; configured dependencies may still be consulted.
//
// Errors:
//   - This function does not return an error.
//
// Note:
//   - Callers should preserve the documented ownership and lifecycle of supplied values.
func NewMemoryDataSetCompiler() *MemoryDataSetCompiler {
	return &MemoryDataSetCompiler{}
}

// Compile compiles the QueryAST into generic SQL.
//
// Purpose:
//
//	Translates a QueryAST execution plan into ANSI SQL with projections, joins, WHERE filters, and GROUP BY.
//
// Where it is used:
//   - Called by DataSetService when driver is "memory".
//
// When can it be used:
//   - When running datasets without an external SQL or document database server.
//
// Parameters:
//   - ctx (context.Context): Input value used by the operation.
//   - ast (*planner.QueryAST): Input value used by the operation.
//   - ds (*domain.DataSet): Input value used by the operation.
//
// Returns:
//   - result1 (*compiler.CompiledPipeline): Returned value.
//   - result2 (error): Returned value.
//
// Business Rules:
//   - Follows the invariants documented by Compile and its owning package.
//
// Side Effects:
//   - Does not intentionally mutate caller-owned input; configured dependencies may still be consulted.
//
// Errors:
//   - Returns validation, configuration, context, or underlying dependency errors.
//
// Note:
//   - The context controls cancellation and request lifetime.
func (c *MemoryDataSetCompiler) Compile(ctx context.Context, ast *planner.QueryAST, ds *domain.DataSet) (*compiler.CompiledPipeline, error) {
	if ast == nil {
		return nil, domain.NewError(domain.ErrPipelineCompilationFailed, "cannot compile nil AST")
	}

	var selectCols []string
	for _, p := range ast.Projections {
		colExpr := fmt.Sprintf("\"%s\".\"%s\"", p.SourceTable, p.SourceField)
		if p.Alias != "" && p.Alias != p.SourceField {
			colExpr += fmt.Sprintf(" AS \"%s\"", p.Alias)
		}
		selectCols = append(selectCols, colExpr)
	}

	for _, cc := range ast.CustomColumns {
		expr := cc.Expression
		if expr != "" {
			alias := cc.Alias
			if alias == "" {
				alias = cc.Label
			}
			selectCols = append(selectCols, fmt.Sprintf("%s AS \"%s\"", expr, alias))
		}
	}

	if len(selectCols) == 0 {
		selectCols = append(selectCols, "*")
	}

	fromClause := fmt.Sprintf("FROM \"%s\" AS \"%s\"", ast.BaseTable.Table, ast.BaseTable.Alias)

	var joinClauses []string
	for _, j := range ast.Joins {
		onCondition := fmt.Sprintf("\"%s\".\"%s\" = \"%s\".\"%s\"", j.FromTable, j.FromField, j.Alias, j.ToField)
		if j.ConvertString {
			switch strings.ToUpper(j.CastMode) {
			case "FROM_ONLY":
				onCondition = fmt.Sprintf("CAST(\"%s\".\"%s\" AS TEXT) = \"%s\".\"%s\"", j.FromTable, j.FromField, j.Alias, j.ToField)
			case "TO_ONLY":
				onCondition = fmt.Sprintf("\"%s\".\"%s\" = CAST(\"%s\".\"%s\" AS TEXT)", j.FromTable, j.FromField, j.Alias, j.ToField)
			default:
				onCondition = fmt.Sprintf("CAST(\"%s\".\"%s\" AS TEXT) = CAST(\"%s\".\"%s\" AS TEXT)", j.FromTable, j.FromField, j.Alias, j.ToField)
			}
		}
		joinClauses = append(joinClauses, fmt.Sprintf("LEFT JOIN \"%s\" AS \"%s\" ON %s", j.ToTable, j.Alias, onCondition))
	}

	var groupByCols []string
	for _, g := range ast.GroupBy {
		groupByCols = append(groupByCols, fmt.Sprintf("\"%s\".\"%s\"", g.Table, g.Field))
	}

	sql := fmt.Sprintf("SELECT\n  %s\n%s", strings.Join(selectCols, ",\n  "), fromClause)
	if len(joinClauses) > 0 {
		sql += "\n" + strings.Join(joinClauses, "\n")
	}
	if len(groupByCols) > 0 {
		sql += "\nGROUP BY " + strings.Join(groupByCols, ", ")
	}

	return &compiler.CompiledPipeline{
		ExecutableQuery:   sql + ";",
		ReferencePipeline: sql + ";",
		Parameters:        ast.Parameters,
		SaveMode:          ds.SaveMode,
		Driver:            "memory",
	}, nil
}

// CompileDataSet compiles QueryAST into Memory SQL.
//
// Parameters:
//   - ctx (context.Context): Input value used by the operation.
//   - ast (*planner.QueryAST): Input value used by the operation.
//   - ds (*domain.DataSet): Input value used by the operation.
//
// Returns:
//   - result1 (*compiler.CompiledPipeline): Returned value.
//   - result2 (error): Returned value.
//
// Business Rules:
//   - Follows the invariants documented by CompileDataSet and its owning package.
//
// Side Effects:
//   - Does not intentionally mutate caller-owned input; configured dependencies may still be consulted.
//
// Errors:
//   - Returns validation, configuration, context, or underlying dependency errors.
//
// Note:
//   - The context controls cancellation and request lifetime.
func (a *MemoryAdapter) CompileDataSet(ctx context.Context, ast *planner.QueryAST, ds *domain.DataSet) (*compiler.CompiledPipeline, error) {
	return NewMemoryDataSetCompiler().Compile(ctx, ast, ds)
}

// DataSetCompiler returns the adapter.DataSetCompiler instance.
//
// Parameters:
//   - None.
//
// Returns:
//   - result1 (adapter.DataSetCompiler): Returned value.
//
// Business Rules:
//   - Follows the invariants documented by DataSetCompiler and its owning package.
//
// Side Effects:
//   - Does not intentionally mutate caller-owned input; configured dependencies may still be consulted.
//
// Errors:
//   - This function does not return an error.
//
// Note:
//   - Callers should preserve the documented ownership and lifecycle of supplied values.
func (a *MemoryAdapter) DataSetCompiler() adapter.DataSetCompiler {
	return &genericCompilerWrapper{c: NewMemoryDataSetCompiler()}
}

type genericCompilerWrapper struct {
	c compiler.DataSetCompiler
}

// Parameters:
//   - ctx (context.Context): Input value used by the operation.
//   - ast (any): Input value used by the operation.
//   - ds (any): Input value used by the operation.
//
// Returns:
//   - result1 (any): Returned value.
//   - result2 (error): Returned value.
//
// Business Rules:
//   - Follows the invariants documented by Compile and its owning package.
//
// Side Effects:
//   - Does not intentionally mutate caller-owned input; configured dependencies may still be consulted.
//
// Errors:
//   - Returns validation, configuration, context, or underlying dependency errors.
//
// Note:
//   - The context controls cancellation and request lifetime.
func (w *genericCompilerWrapper) Compile(ctx context.Context, ast any, ds any) (any, error) {
	qAst, _ := ast.(*planner.QueryAST)
	dSet, _ := ds.(*domain.DataSet)
	return w.c.Compile(ctx, qAst, dSet)
}
