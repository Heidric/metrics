package noosexit

import (
	"go/ast"
	"go/token"
	"go/types"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/analysis"
)

const doc = `noosexit reports a diagnostic when a direct call to os.Exit is found
in the main function of the main package.

Rationale: calling os.Exit in main makes testing and graceful shutdown harder.
Prefer returning an error from main and handling it via log.Fatal or similar,
or exit in another function outside of main if absolutely necessary.`

var Analyzer = &analysis.Analyzer{
	Name: "noosexit",
	Doc:  doc,
	Run:  run,
}

func fileName(fset *token.FileSet, f *ast.File) string {
	if f == nil {
		return ""
	}
	pos := f.Pos()
	if !pos.IsValid() {
		return ""
	}
	tf := fset.File(pos)
	if tf == nil {
		return ""
	}
	return filepath.Clean(tf.Name())
}

func isBuildCachePkg(pass *analysis.Pass) bool {
	if pass.Fset == nil {
		return false
	}
	allInCache := true
	for _, f := range pass.Files {
		if f == nil {
			continue
		}
		fp := fileName(pass.Fset, f)
		low := strings.ToLower(fp)
		if !(strings.Contains(low, "go-build") ||
			strings.Contains(low, "library/caches/go-build") ||
			strings.Contains(low, "appdata\\local\\go-build")) {
			allInCache = false
			break
		}
	}
	return allInCache
}

func run(pass *analysis.Pass) (any, error) {
	if pass.Pkg.Name() != "main" {
		return nil, nil
	}
	if isBuildCachePkg(pass) {
		return nil, nil
	}

	for _, f := range pass.Files {
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || sel.Sel == nil || sel.Sel.Name != "Exit" {
				return true
			}
			if ident, ok := sel.X.(*ast.Ident); ok {
				if obj, ok := pass.TypesInfo.Uses[ident]; ok {
					if pkgName, ok := obj.(*types.PkgName); ok && pkgName.Imported() != nil && pkgName.Imported().Path() == "os" {
						pass.Reportf(sel.Sel.Pos(), "direct call to os.Exit in package main is forbidden")
					}
				}
			}
			return true
		})
	}
	return nil, nil
}
