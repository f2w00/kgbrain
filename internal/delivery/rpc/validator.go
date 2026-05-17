package rpc

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/santhosh-tekuri/jsonschema/v5"
	"gopkg.in/yaml.v3"
)

type ParamsValidator struct {
	schemas    map[string]*jsonschema.Schema
	openrpcDoc json.RawMessage
}

func NewParamsValidator(path string) (*ParamsValidator, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read openrpc file: %w", err)
	}

	var doc map[string]any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse openrpc yaml: %w", err)
	}

	openrpcJSON, err := json.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("marshal openrpc json: %w", err)
	}

	schemas, err := buildMethodSchemas(doc)
	if err != nil {
		return nil, fmt.Errorf("build method schemas: %w", err)
	}

	return &ParamsValidator{
		schemas:    schemas,
		openrpcDoc: openrpcJSON,
	}, nil
}

func (v *ParamsValidator) Validate(method string, params json.RawMessage) error {
	schema, ok := v.schemas[method]
	if !ok {
		return nil
	}

	var data any
	if err := json.Unmarshal(params, &data); err != nil {
		return fmt.Errorf("parse params: %w", err)
	}

	if err := schema.Validate(data); err != nil {
		return err
	}
	return nil
}

func (v *ParamsValidator) OpenRPCDoc() json.RawMessage {
	return v.openrpcDoc
}

func buildMethodSchemas(doc map[string]any) (map[string]*jsonschema.Schema, error) {
	methodsRaw, ok := doc["methods"]
	if !ok {
		return nil, nil
	}

	methods, ok := methodsRaw.([]any)
	if !ok {
		return nil, fmt.Errorf("methods is not an array")
	}

	schemas := make(map[string]*jsonschema.Schema)

	for _, mRaw := range methods {
		m, ok := mRaw.(map[string]any)
		if !ok {
			continue
		}

		nameRaw, ok := m["name"]
		if !ok {
			continue
		}
		name, ok := nameRaw.(string)
		if !ok {
			continue
		}

		paramsRaw, ok := m["params"]
		if !ok {
			continue
		}
		params, ok := paramsRaw.([]any)
		if !ok {
			continue
		}

		schemaObj := buildParamSchema(params)
		schemaJSON, err := json.Marshal(schemaObj)
		if err != nil {
			return nil, fmt.Errorf("method %q: marshal schema: %w", name, err)
		}

		schema, err := jsonschema.CompileString(name, string(schemaJSON))
		if err != nil {
			return nil, fmt.Errorf("method %q: compile schema: %w", name, err)
		}

		schemas[name] = schema
	}

	return schemas, nil
}

func buildParamSchema(params []any) map[string]any {
	properties := make(map[string]any)
	required := []string{}

	for _, pRaw := range params {
		p, ok := pRaw.(map[string]any)
		if !ok {
			continue
		}

		nameRaw, ok := p["name"]
		if !ok {
			continue
		}
		name, ok := nameRaw.(string)
		if !ok {
			continue
		}

		schemaRaw, ok := p["schema"]
		if !ok {
			continue
		}
		schema, ok := schemaRaw.(map[string]any)
		if !ok {
			continue
		}

		properties[name] = schema

		reqRaw, ok := p["required"]
		if ok {
			if req, ok := reqRaw.(bool); ok && req {
				required = append(required, name)
			}
		}
	}

	result := map[string]any{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		result["required"] = required
	}

	return result
}
