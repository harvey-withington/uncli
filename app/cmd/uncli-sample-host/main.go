// uncli-sample-host is a Go host running its own UNCLI (decision 0027): it
// serves a page with <uncli-chat src="/uncli"> and mounts UNCLI's handler
// at /uncli/. A real host puts its own sign-in in front of the mount.
//
//	npm --prefix app/frontend run build:embed
//	go run ./cmd/uncli-sample-host --page frontend/dist-embed/sample --data <folder>
//
// then open the address it prints. --user-cli-config uses the CLI you're
// already signed in to, so it works before the host has a setup screen.
package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"

	"github.com/harvey-withington/uncli/app/uncli"
)

func main() {
	uncli.RunHook() // with Options.Hooks: this program answers the CLIs' hooks too
	page := flag.String("page", "frontend/dist-embed/sample", "the built sample page")
	data := flag.String("data", "", "the host's folder for its UNCLI (required)")
	addr := flag.String("listen", "127.0.0.1:8790", "where to serve the page")
	userCLI := flag.Bool("user-cli-config", false, "use your own CLI sign-in and settings")
	flag.Parse()

	origin := "http://" + *addr
	core, err := uncli.Start(uncli.Options{DataDir: *data, Origins: []string{origin}, Name: "Sample host", Hooks: true, UserCLIConfig: *userCLI})
	if err != nil {
		log.Fatal(err)
	}
	defer core.Close()

	mux := http.NewServeMux()
	mux.Handle("/uncli/", http.StripPrefix("/uncli", core.Handler())) // a real host: behind its sign-in
	mux.Handle("/", http.FileServer(http.Dir(*page)))
	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Open %s/?src=/uncli\n", origin)
	log.Fatal(http.Serve(ln, mux))
}
