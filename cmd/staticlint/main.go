// Package main implements a multichecker that aggregates standard analysis passes,
// Staticcheck analyzers, a couple of public analyzers,
// and a custom rule that forbids direct os.Exit calls in main.main.
package main

import (
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/multichecker"

	"golang.org/x/tools/go/analysis/passes/asmdecl"
	"golang.org/x/tools/go/analysis/passes/assign"
	"golang.org/x/tools/go/analysis/passes/atomic"
	"golang.org/x/tools/go/analysis/passes/bools"
	"golang.org/x/tools/go/analysis/passes/buildssa"
	"golang.org/x/tools/go/analysis/passes/cgocall"
	"golang.org/x/tools/go/analysis/passes/composite"
	"golang.org/x/tools/go/analysis/passes/copylock"
	"golang.org/x/tools/go/analysis/passes/deepequalerrors"
	"golang.org/x/tools/go/analysis/passes/errorsas"
	"golang.org/x/tools/go/analysis/passes/fieldalignment"
	"golang.org/x/tools/go/analysis/passes/httpresponse"
	"golang.org/x/tools/go/analysis/passes/loopclosure"
	"golang.org/x/tools/go/analysis/passes/lostcancel"
	"golang.org/x/tools/go/analysis/passes/nilfunc"
	"golang.org/x/tools/go/analysis/passes/nilness"
	"golang.org/x/tools/go/analysis/passes/printf"
	"golang.org/x/tools/go/analysis/passes/shadow"
	"golang.org/x/tools/go/analysis/passes/sortslice"
	"golang.org/x/tools/go/analysis/passes/stdmethods"
	"golang.org/x/tools/go/analysis/passes/stringintconv"
	"golang.org/x/tools/go/analysis/passes/structtag"
	"golang.org/x/tools/go/analysis/passes/testinggoroutine"
	"golang.org/x/tools/go/analysis/passes/tests"
	"golang.org/x/tools/go/analysis/passes/unmarshal"
	"golang.org/x/tools/go/analysis/passes/unreachable"
	"golang.org/x/tools/go/analysis/passes/unsafeptr"
	"golang.org/x/tools/go/analysis/passes/unusedresult"

	"honnef.co/go/tools/simple"
	"honnef.co/go/tools/staticcheck"
	"honnef.co/go/tools/stylecheck"

	"github.com/kyoh86/exportloopref"
	"github.com/timakin/bodyclose/passes/bodyclose"

	"github.com/Heidric/metrics.git/cmd/staticlint/noosexit"
)

func main() {
	var analyzers []*analysis.Analyzer

	for _, a := range []*analysis.Analyzer{
		asmdecl.Analyzer, assign.Analyzer, atomic.Analyzer, bools.Analyzer, buildssa.Analyzer,
		cgocall.Analyzer, composite.Analyzer, copylock.Analyzer, deepequalerrors.Analyzer,
		errorsas.Analyzer, fieldalignment.Analyzer, httpresponse.Analyzer, loopclosure.Analyzer,
		lostcancel.Analyzer, nilfunc.Analyzer, nilness.Analyzer, printf.Analyzer, shadow.Analyzer,
		sortslice.Analyzer, stdmethods.Analyzer, stringintconv.Analyzer, structtag.Analyzer,
		testinggoroutine.Analyzer, tests.Analyzer, unmarshal.Analyzer, unreachable.Analyzer,
		unsafeptr.Analyzer, unusedresult.Analyzer,
	} {
		analyzers = append(analyzers, a)
	}

	for _, a := range staticcheck.Analyzers {
		if len(a.Analyzer.Name) >= 2 && a.Analyzer.Name[:2] == "SA" {
			analyzers = append(analyzers, a.Analyzer)
		}
	}
	for _, a := range simple.Analyzers {
		if a.Analyzer.Name == "S1000" || a.Analyzer.Name == "S1009" {
			analyzers = append(analyzers, a.Analyzer)
		}
	}
	for _, a := range stylecheck.Analyzers {
		if a.Analyzer.Name == "ST1000" {
			analyzers = append(analyzers, a.Analyzer)
		}
	}

	analyzers = append(analyzers, bodyclose.Analyzer)
	analyzers = append(analyzers, exportloopref.Analyzer)

	analyzers = append(analyzers, noosexit.Analyzer)

	multichecker.Main(analyzers...)
}
