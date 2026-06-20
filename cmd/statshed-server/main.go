// Command statshed-server is the StatShed status-dashboard server. When complete it
// serves the REST API under /api, the Server-Sent Events stream at /api/events, and
// the embedded React SPA at /, backed by a single SQLite database.
//
// AIDEV-NOTE: Bootstrap stub (Task 0.1). The real wiring — config load, store open +
// goose migrate, chi router + middleware, SSE hub, the 60s background worker, and
// graceful shutdown — is added in later phases of loop/impl-guide.md (Phases 2-5),
// where main() also grows the --healthcheck subcommand. For now this only establishes
// the module, a compiling build, and the --version flag (S7).
package main

import (
	"flag"
	"fmt"
)

// version is the build version of the server, injected at link time via
// -ldflags "-X main.version=<tag>" (S7). It defaults to "dev" for un-stamped local
// builds.
var version = "dev"

func main() {
	showVersion := flag.Bool("version", false, "print the version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println(version)
		return
	}

	fmt.Printf("statshed-server %s\n", version)
}
