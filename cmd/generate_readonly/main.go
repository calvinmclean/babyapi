package main

import (
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"

	"golang.org/x/tools/go/ast/astutil"
)

// TODO: generate tests using each modifier name and generate tests?

const panicFuncName = "panicIfReadOnly"

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run main.go <directory>")
		os.Exit(1)
	}

	dir := os.Args[1]

	// read the files in the specified directory
	files, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		fmt.Println("Error reading directory:", err)
		os.Exit(1)
	}

	// parse the expression to easily insert
	generatedExpr, err := parser.ParseExpr(fmt.Sprintf("a.%s()\n", panicFuncName))
	if err != nil {
		fmt.Println("Error reading directory:", err)
		os.Exit(1)
	}

	// parse each Go source file
	for _, file := range files {
		err = addReadOnlyPanicToFile(file, generatedExpr)
		if err != nil {
			fmt.Println("error parsing/editing file:", err)
		}
	}
}

func addReadOnlyPanicToFile(filename string, generatedExpr ast.Expr) error {
	fset := token.NewFileSet()

	node, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("error parsing file: %w", err)
	}

	astutil.Apply(node, nil, func(c *astutil.Cursor) bool {
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

		funcDecl.Body.List = append([]ast.Stmt{&ast.ExprStmt{
			X: generatedExpr,
		}}, funcDecl.Body.List...)

		c.Replace(funcDecl)

		return true
	})

	outputFile, err := os.OpenFile(filename, os.O_RDWR, 0644)
	if err != nil {
		return fmt.Errorf("error opening output file: %w", err)
	}
	defer outputFile.Close()

	err = format.Node(outputFile, fset, &printer.CommentedNode{Node: node})
	if err != nil {
		return fmt.Errorf("error writing file: %w", err)
	}

	return nil
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

	genIdent, ok := idx.Index.(*ast.Ident)
	if !ok {
		return false
	}

	recvType := ident.Name
	genType := genIdent.Name

	return recvType == "API" && genType == "T"
}
