// uncli-core is a host's own UNCLI as a program of its own (decision 0027),
// for hosts not written in Go: the host starts it, reads the port it
// prints, and forwards a path of its server (/uncli/, say) to it, adding
// the key to each request.
//
//	uncli-core --data <folder> --origin https://app.example --key-file <file> [--name App] [--listen 127.0.0.1:0] [--user-cli-config] [--allow-service-account]
//
// It listens on loopback only, and prints one JSON line once it's ready:
// {"port":51234}. It stops when its standard input closes, so start it
// with a pipe there: it ends with the host, however the host ends.
// The key can also come in UNCLI_CORE_KEY instead of a file.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/harvey-withington/uncli/app/uncli"
)

func main() {
	uncli.RunHook()
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "uncli-core:", err)
		os.Exit(1)
	}
}

func run() error {
	var origins multi
	data := flag.String("data", "", "the host's folder for everything this UNCLI keeps (required)")
	flag.Var(&origins, "origin", "a page origin of the host's, such as https://app.example (repeat for more)")
	keyFile := flag.String("key-file", "", "a file holding the key the host's proxy adds (or UNCLI_CORE_KEY)")
	name := flag.String("name", "", "the host's name, recorded on the approvals its pages answer")
	listen := flag.String("listen", "127.0.0.1:0", "the loopback address to listen on")
	userCLI := flag.Bool("user-cli-config", false, "use the user's own CLI config instead of one in the data folder")
	asService := flag.Bool("allow-service-account", false, "run even as a Windows service account such as LocalSystem")
	flag.Parse()

	key := os.Getenv("UNCLI_CORE_KEY")
	if *keyFile != "" {
		b, err := os.ReadFile(*keyFile)
		if err != nil {
			return err
		}
		key = strings.TrimSpace(string(b))
	}
	if key == "" {
		return errors.New("give the key with --key-file or UNCLI_CORE_KEY")
	}
	host, _, err := net.SplitHostPort(*listen)
	if err != nil {
		return err
	}
	if ip := net.ParseIP(host); ip == nil || !ip.IsLoopback() {
		return errors.New("--listen must be a loopback address, such as 127.0.0.1:0")
	}
	ln, err := net.Listen("tcp", *listen)
	if err != nil {
		return err
	}
	core, err := uncli.Start(uncli.Options{DataDir: *data, Origins: origins, Name: *name, Key: key, Hooks: true, UserCLIConfig: *userCLI, AllowServiceAccount: *asService})
	if err != nil {
		return err
	}
	defer core.Close()
	srv := &http.Server{Handler: core.KeyedHandler(), ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 2 * time.Minute}
	go func() {
		_, _ = io.Copy(io.Discard, os.Stdin) // the host has gone
		_ = srv.Close()
	}()
	ready, _ := json.Marshal(map[string]int{"port": ln.Addr().(*net.TCPAddr).Port})
	fmt.Println(string(ready))
	if err := srv.Serve(ln); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// multi is a flag given more than once.
type multi []string

func (m *multi) String() string     { return strings.Join(*m, ",") }
func (m *multi) Set(v string) error { *m = append(*m, v); return nil }
