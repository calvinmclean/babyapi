package main

import (
	"fmt"
	"go/ast"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/singlechecker"
	"golang.org/x/tools/go/ast/astutil"
)

// TODO: generate tests using each modifier name and generate tests?

const panicFuncName = "panicIfReadOnly"

var readOnlyAnalyzer = &analysis.Analyzer{
	Name: "readonlypanic",
	Doc:  "adds panicIfReadOnly() call to API methods that return API[T]",
	Run:  run,
}

func main() {
	singlechecker.Main(readOnlyAnalyzer)
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		findFunctionsNeedingPanic(pass, file)
	}

	return nil, nil
}

func findFunctionsNeedingPanic(pass *analysis.Pass, file *ast.File) {
	astutil.Apply(file, nil, func(c *astutil.Cursor) bool {
		funcDecl, ok := c.Node().(*ast.FuncDecl)
		if !ok {
			return true
		}

		recvType := getFuncReceiverType(funcDecl)
		returnType := getFuncReturnType(funcDecl)

		// exit if there is no receiver or return
		if recvType == nil || returnType == nil {
			return true
		}

		if !(isAPIType(recvType) && isAPIType(returnType)) {
			return true
		}

		if alreadyGenerated(funcDecl.Body.List) {
			return true
		}

		// Report diagnostic with suggested fix
		pass.Report(analysis.Diagnostic{
			Pos:     funcDecl.Body.Lbrace,
			End:     funcDecl.Body.Lbrace + 1,
			Message: fmt.Sprintf("function %s should call %s at the beginning", funcDecl.Name.Name, panicFuncName),
			SuggestedFixes: []analysis.SuggestedFix{
				{
					Message: fmt.Sprintf("add %s() call", panicFuncName),
					TextEdits: []analysis.TextEdit{
						{
							Pos:     funcDecl.Body.Lbrace + 1,
							End:     funcDecl.Body.Lbrace + 1,
							NewText: fmt.Appendf(nil, "a.%s()\n", panicFuncName),
						},
					},
				},
			},
		})

		return true
	})
}

func getFuncReturnType(f *ast.FuncDecl) *ast.StarExpr {
	if f.Type == nil || f.Type.Results == nil {
		return nil
	}

	returnType, _ := f.Type.Results.List[0].Type.(*ast.StarExpr)
	return returnType
}

func getFuncReceiverType(f *ast.FuncDecl) *ast.StarExpr {
	if f.Recv == nil || len(f.Recv.List) != 1 {
		return nil
	}

	recvType, _ := f.Recv.List[0].Type.(*ast.StarExpr)
	return recvType
}

func alreadyGenerated(bodyList []ast.Stmt) bool {
	for _, item := range bodyList {
		exprStmt, ok := item.(*ast.ExprStmt)
		if !ok {
			continue
		}

		callExpr, ok := exprStmt.X.(*ast.CallExpr)
		if !ok {
			continue
		}

		sel, ok := callExpr.Fun.(*ast.SelectorExpr)
		if !ok {
			continue
		}

		if sel.Sel.Name == panicFuncName {
			return true
		}
	}

	return false
}

func isAPIType(input *ast.StarExpr) bool {
	idx, ok := input.X.(*ast.IndexExpr)
	if !ok {
		return false
	}

	ident, ok := idx.X.(*ast.Ident)
	if !ok {
		return false
	}

	// Check if the index is an identifier (could be any type parameter)
	_, ok = idx.Index.(*ast.Ident)
	if !ok {
		return false
	}

	return ident.Name == "API"
}
