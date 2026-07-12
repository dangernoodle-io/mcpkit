// Command minimal composes the smallest possible mcpkit server: a generic
// stdio host plus a single "hello" capability.
package main

import (
	"context"
	"fmt"
	"log"

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
	}, mcpkit.ReadOnly, func(_ context.Context, _ *mcpx.CallToolRequest, in helloIn) (*mcpx.CallToolResult, helloOut, error) {
		name := in.Name
		if name == "" {
			name = "world"
		}
		return nil, helloOut{Greeting: fmt.Sprintf("hello, %s!", name)}, nil
	})
	return nil
}

func main() {
	app, err := mcpkit.New(mcpkit.Info{Name: "minimal", Version: "0.0.1"}, generic.New(), helloCap{})
	if err != nil {
		log.Fatalf("compose app: %v", err)
	}
	if err := app.Run(context.Background()); err != nil {
		log.Fatalf("run app: %v", err)
	}
}
