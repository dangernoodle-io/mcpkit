// Command http demonstrates serving an mcpkit App over streamable-HTTP as
// one handler among the consumer's own on a single mux/server — proving
// mcpkit's HTTPHandler is a bare, path-agnostic http.Handler and that
// MCP-over-HTTP is entirely opt-in.
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/dangernoodle-io/mcpkit"
	"github.com/dangernoodle-io/mcpkit/host/generic"
	"github.com/dangernoodle-io/mcpkit/mcpx"
)

type helloIn struct {
	Name string `json:"name" jsonschema:"name to greet"`
}

type helloOut struct {
	Greeting string `json:"greeting"`
}

type helloCap struct{}

func (helloCap) Attach(r *mcpkit.Registrar) error {
	mcpkit.AddTool(r, &mcpx.Tool{
		Name:        "hello",
		Description: "greets the caller by name",
	}, func(_ context.Context, _ *mcpx.CallToolRequest, in helloIn) (*mcpx.CallToolResult, helloOut, error) {
		name := in.Name
		if name == "" {
			name = "world"
		}
		return nil, helloOut{Greeting: fmt.Sprintf("hello, %s!", name)}, nil
	})
	return nil
}

// newMux builds the co-mount example: MCP as just one handler among the
// consumer's own on a single mux.
func newMux(app *mcpkit.App) *http.ServeMux {
	mux := http.NewServeMux()
	// The mount path is the consumer's choice — "/mcp" here is illustrative,
	// not required. mcpkit imposes no route, and calling HTTPHandler at all
	// is opt-in: a server that never serves MCP over HTTP simply never calls it.
	mux.Handle("/mcp", app.HTTPHandler())
	// The same mux/server can serve unrelated purposes alongside MCP.
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	return mux
}

func main() {
	app, err := mcpkit.New(mcpkit.Info{Name: "http-demo", Version: "0.0.1"}, generic.New(), helloCap{})
	if err != nil {
		log.Fatalf("compose app: %v", err)
	}

	log.Fatal(http.ListenAndServe(":8080", newMux(app)))
}
