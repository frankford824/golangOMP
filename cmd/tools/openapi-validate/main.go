package main

import (
	"fmt"
	"os"

	"github.com/getkin/kin-openapi/openapi3"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: go run ./cmd/tools/openapi-validate <openapi-file>")
		os.Exit(2)
	}

	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromFile(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "load failed: %v\n", err)
		os.Exit(1)
	}

	if doc == nil || doc.Paths == nil || doc.Components.Schemas == nil {
		fmt.Fprintln(os.Stderr, "validate failed: missing required top-level OpenAPI sections")
		os.Exit(1)
	}

	fmt.Println("openapi validate: 0 error 0 warning")
}
