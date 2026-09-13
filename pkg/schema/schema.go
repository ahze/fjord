package schema

import (
	"bytes"
	_ "embed"
	"fmt"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

//go:embed fjord-compose-v1.json
var composeSchemaBytes []byte

//go:embed fjord-catalog-v1.json
var catalogSchemaBytes []byte

var (
	composeSchema *jsonschema.Schema
	catalogSchema *jsonschema.Schema
)

func init() {
	compiler := jsonschema.NewCompiler()

	cDoc, err := jsonschema.UnmarshalJSON(bytes.NewReader(composeSchemaBytes))
	if err != nil {
		panic(fmt.Sprintf("invalid compose schema json: %v", err))
	}
	if err := compiler.AddResource("https://daemonless.io/schemas/fjord-compose-v1.json", cDoc); err != nil {
		panic(fmt.Sprintf("failed to load compose schema: %v", err))
	}

	catDoc, err := jsonschema.UnmarshalJSON(bytes.NewReader(catalogSchemaBytes))
	if err != nil {
		panic(fmt.Sprintf("invalid catalog schema json: %v", err))
	}
	if err := compiler.AddResource("https://daemonless.io/schemas/fjord-catalog-v1.json", catDoc); err != nil {
		panic(fmt.Sprintf("failed to load catalog schema: %v", err))
	}

	composeSchema, err = compiler.Compile("https://daemonless.io/schemas/fjord-compose-v1.json")
	if err != nil {
		panic(fmt.Sprintf("failed to compile compose schema: %v", err))
	}

	catalogSchema, err = compiler.Compile("https://daemonless.io/schemas/fjord-catalog-v1.json")
	if err != nil {
		panic(fmt.Sprintf("failed to compile catalog schema: %v", err))
	}
}

// ValidateManifest validates a byte slice containing JSON against the Fjord Compose schema.
func ValidateManifest(data []byte) error {
	v, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("invalid json: %w", err)
	}
	return composeSchema.Validate(v)
}

// ValidateCatalog validates a byte slice containing JSON against the Fjord Catalog schema.
func ValidateCatalog(data []byte) error {
	v, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("invalid json: %w", err)
	}
	return catalogSchema.Validate(v)
}
