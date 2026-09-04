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
func NewMemoryDataSetCompiler() *MemoryDataSetCompiler {
	return &MemoryDataSetCompiler{}
}

// Compile compiles the QueryAST into generic SQL.
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

	sql := fmt.Sprintf("SELECT\n  %s\n%s", strings.Join(selectCols, ",\n  "), fromClause)
	if len(joinClauses) > 0 {
		sql += "\n" + strings.Join(joinClauses, "\n")
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
func (a *MemoryAdapter) CompileDataSet(ctx context.Context, ast *planner.QueryAST, ds *domain.DataSet) (*compiler.CompiledPipeline, error) {
	return NewMemoryDataSetCompiler().Compile(ctx, ast, ds)
}

// DataSetCompiler returns the adapter.DataSetCompiler instance.
func (a *MemoryAdapter) DataSetCompiler() adapter.DataSetCompiler {
	return &genericCompilerWrapper{c: NewMemoryDataSetCompiler()}
}

type genericCompilerWrapper struct {
	c compiler.DataSetCompiler
}

func (w *genericCompilerWrapper) Compile(ctx context.Context, ast any, ds any) (any, error) {
	qAst, _ := ast.(*planner.QueryAST)
	dSet, _ := ds.(*domain.DataSet)
	return w.c.Compile(ctx, qAst, dSet)
}
