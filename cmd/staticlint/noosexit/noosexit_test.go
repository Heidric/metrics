package noosexit_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/Heidric/metrics.git/cmd/staticlint/noosexit"
)

func TestNoOsExit(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, noosexit.Analyzer, "a", "b", "c")
}
