package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"golang.org/x/tools/imports"
	"os"
	"path/filepath"
	"strings"
)

type StructInfo struct {
	InterfaceName string
	SourceFile    string
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: <path-to-go-file-or-folder>")
		os.Exit(1)
	}
	root := os.Args[1]

	markerComment := "// +generate_interface"

	if len(os.Args) > 2 {
		markerComment = os.Args[2]
	}

	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, root, nil, parser.ParseComments)
	if err != nil {
		panic(err)
	}

	for _, pkg := range pkgs {
		// Map to store struct names and their generated interface names
		structsToInterfaces := map[string]StructInfo{}
		// Map to store methods for each struct across all files
		methods := map[string][]*ast.FuncDecl{}

		// First pass: Collect structs with the marker comment
		for filename, file := range pkg.Files {
			ast.Inspect(file, func(n ast.Node) bool {
				gen, ok := n.(*ast.GenDecl)
				if !ok || gen.Tok != token.TYPE {
					return true
				}
				for _, spec := range gen.Specs {
					typeSpec, ok := spec.(*ast.TypeSpec)
					if !ok {
						continue
					}
					_, ok = typeSpec.Type.(*ast.StructType)
					if !ok {
						continue
					}
					if gen.Doc != nil {
						for _, comment := range gen.Doc.List {
							if comment.Text == markerComment {
								structsToInterfaces[typeSpec.Name.Name] = StructInfo{
									InterfaceName: typeSpec.Name.String() + "Interface",
									SourceFile:    filename,
								}
							}
						}
					}
				}
				return true
			})
		}

		// Second pass: Collect methods for structs across all files
		for _, file := range pkg.Files {
			ast.Inspect(file, func(n ast.Node) bool {
				fn, ok := n.(*ast.FuncDecl)
				if !ok || fn.Recv == nil {
					return true
				}
				recvType := recvName(fn.Recv.List[0].Type)
				if recvType == "" {
					return true
				}
				// Check if the method belongs to a struct we're interested in
				if _, ok := structsToInterfaces[recvType]; ok {
					methods[recvType] = append(methods[recvType], fn)
				}
				return true
			})
		}

		// For each marked struct, generate interface
		for structName, structInfo := range structsToInterfaces {
			ifacePath := filepath.Join(filepath.Dir(structInfo.SourceFile), strings.Split(strings.ToLower(structInfo.InterfaceName), "interface")[0]+".interface.go")
			var buf bytes.Buffer

			buf.WriteString(fmt.Sprintf("package %s\n\n", pkg.Name))
			buf.WriteString(fmt.Sprintf("type %s interface {\n", structInfo.InterfaceName))
			for _, fn := range methods[structName] {
				buf.WriteString("\t" + fn.Name.Name + formatFieldList(fn.Type.Params) + formatResultList(fn.Type.Results) + "\n")
			}
			buf.WriteString("}\n")

			formatted, err := imports.Process(ifacePath, buf.Bytes(), nil)
			if err != nil {
				panic(err) // or handle nicely
			}
			os.WriteFile(ifacePath, formatted, 0644)
			// Write the generated interface to a file
		}
	}
}

// helper: get the receiver type
func recvName(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.StarExpr:
		if ident, ok := e.X.(*ast.Ident); ok {
			return ident.Name
		}
	case *ast.Ident:
		return e.Name
	}
	return ""
}

// helper: format function params
func formatFieldList(fl *ast.FieldList) string {
	var parts []string
	if fl == nil {
		return "()"
	}
	for _, f := range fl.List {
		var buf bytes.Buffer
		printer.Fprint(&buf, token.NewFileSet(), f.Type)
		typeStr := buf.String()
		if len(f.Names) > 0 {
			for _, name := range f.Names {
				parts = append(parts, fmt.Sprintf("%s %s", name.Name, typeStr))
			}
		} else {
			parts = append(parts, typeStr)
		}
	}
	return "(" + strings.Join(parts, ", ") + ")"
}

// helper: format function results
func formatResultList(fl *ast.FieldList) string {
	if fl == nil || len(fl.List) == 0 {
		return ""
	}
	var parts []string
	for _, f := range fl.List {
		var buf bytes.Buffer
		printer.Fprint(&buf, token.NewFileSet(), f.Type)
		typeStr := buf.String()
		if len(f.Names) > 0 {
			for _, name := range f.Names {
				parts = append(parts, fmt.Sprintf("%s %s", name.Name, typeStr))
			}
		} else {
			parts = append(parts, typeStr)
		}
	}
	// special case: single result, no names
	if len(parts) == 1 && !strings.Contains(parts[0], " ") {
		return " " + parts[0]
	}
	return " (" + strings.Join(parts, ", ") + ")"
}
