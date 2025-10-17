/*
Command staticlint runs a custom multichecker over the project.

USAGE

	go run ./cmd/staticlint -- <packages>

EXAMPLES

	# run against the whole module
	go run ./cmd/staticlint -- ./...

	# run on a specific package
	go run ./cmd/staticlint -- ./metrics/...

WHAT'S INSIDE

- Standard analyzers from golang.org/x/tools/go/analysis/passes (printf, shadow, nilness, etc.).
- All Staticcheck SA analyzers (honnef.co/go/tools/staticcheck).
- Extra Staticcheck classes: a couple from simple (S1000, S1009) and one from stylecheck (ST1000).
- Two public analyzers:
  - github.com/timakin/bodyclose/passes/bodyclose — ensures HTTP response bodies are closed.
  - github.com/kyoh86/exportloopref — catches references to loop variables in closures.

- Custom analyzer noosexit — forbids direct calls to os.Exit inside main.main of package main.

CUSTOM RULE (noosexit)

This rule scans only packages named "main" and inspects the AST of main.main.
If it sees a call expression whose resolved function object is os.Exit,
it reports a diagnostic. This still allows using log.Fatal and friends.

# EXIT-FREE MAIN MIGRATION

Replace:

	os.Exit(1)

with:

	log.Fatal(err) // or return an error from subcommands

# INTEGRATION

You can put the command into CI, e.g.:

	staticcheck_cmd="go run ./cmd/staticlint --"
	$staticcheck_cmd ./...
*/
package main
