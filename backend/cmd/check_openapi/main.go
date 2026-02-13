package main

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type openAPIDoc struct {
	Components struct {
		Schemas map[string]schema `yaml:"schemas"`
	} `yaml:"components"`
}

type schema struct {
	Type       string            `yaml:"type"`
	Ref        string            `yaml:"$ref"`
	Properties map[string]schema `yaml:"properties"`
	Required   []string          `yaml:"required"`
	Items      *schema           `yaml:"items"`
}

type schemaShape struct {
	Type       string
	Required   []string
	Properties map[string]propertyShape
}

type propertyShape struct {
	Type     string
	ItemsRef string
}

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintf(os.Stderr, "usage: %s <gateway-openapi.yaml> <internal-openapi.yaml>\n", os.Args[0])
		os.Exit(2)
	}

	gatewayPath := os.Args[1]
	internalPath := os.Args[2]

	gatewayDoc, err := loadDoc(gatewayPath)
	if err != nil {
		exitErr(err)
	}
	internalDoc, err := loadDoc(internalPath)
	if err != nil {
		exitErr(err)
	}

	gatewayErr, err := getSchema(gatewayDoc, "ErrorResponse")
	if err != nil {
		exitErr(fmt.Errorf("gateway: %w", err))
	}
	internalErr, err := getSchema(internalDoc, "ErrorResponse")
	if err != nil {
		exitErr(fmt.Errorf("internal: %w", err))
	}

	gatewayDetail, err := getSchema(gatewayDoc, "ErrorDetail")
	if err != nil {
		exitErr(fmt.Errorf("gateway: %w", err))
	}
	internalDetail, err := getSchema(internalDoc, "ErrorDetail")
	if err != nil {
		exitErr(fmt.Errorf("internal: %w", err))
	}

	if err := validateErrorResponse("gateway", gatewayErr); err != nil {
		exitErr(err)
	}
	if err := validateErrorResponse("internal", internalErr); err != nil {
		exitErr(err)
	}
	if err := validateErrorDetail("gateway", gatewayDetail); err != nil {
		exitErr(err)
	}
	if err := validateErrorDetail("internal", internalDetail); err != nil {
		exitErr(err)
	}

	if err := ensureSameShape("ErrorResponse", shapeFromSchema(gatewayErr), shapeFromSchema(internalErr)); err != nil {
		exitErr(err)
	}
	if err := ensureSameShape("ErrorDetail", shapeFromSchema(gatewayDetail), shapeFromSchema(internalDetail)); err != nil {
		exitErr(err)
	}

	fmt.Println("OpenAPI consistency check passed.")
}

func loadDoc(path string) (openAPIDoc, error) {
	var doc openAPIDoc
	raw, err := os.ReadFile(path)
	if err != nil {
		return doc, fmt.Errorf("read %s: %w", path, err)
	}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return doc, fmt.Errorf("parse %s: %w", path, err)
	}
	return doc, nil
}

func getSchema(doc openAPIDoc, name string) (schema, error) {
	if doc.Components.Schemas == nil {
		return schema{}, errors.New("components.schemas missing")
	}
	s, ok := doc.Components.Schemas[name]
	if !ok {
		return schema{}, fmt.Errorf("schema %q missing", name)
	}
	return s, nil
}

func validateErrorResponse(scope string, s schema) error {
	if s.Type != "object" {
		return fmt.Errorf("%s ErrorResponse must be object", scope)
	}
	required := makeSet(s.Required)
	for _, field := range []string{"error", "code"} {
		if !required[field] {
			return fmt.Errorf("%s ErrorResponse.required must include %q", scope, field)
		}
	}
	errorProp, ok := s.Properties["error"]
	if !ok || errorProp.Type != "string" {
		return fmt.Errorf("%s ErrorResponse.error must be string", scope)
	}
	codeProp, ok := s.Properties["code"]
	if !ok || codeProp.Type != "string" {
		return fmt.Errorf("%s ErrorResponse.code must be string", scope)
	}
	reqIDProp, ok := s.Properties["requestId"]
	if !ok || reqIDProp.Type != "string" {
		return fmt.Errorf("%s ErrorResponse.requestId must be string", scope)
	}
	detailsProp, ok := s.Properties["details"]
	if !ok || detailsProp.Type != "array" {
		return fmt.Errorf("%s ErrorResponse.details must be array", scope)
	}
	if detailsProp.Items == nil || strings.TrimSpace(detailsProp.Items.Ref) != "#/components/schemas/ErrorDetail" {
		return fmt.Errorf("%s ErrorResponse.details.items must reference ErrorDetail", scope)
	}
	return nil
}

func validateErrorDetail(scope string, s schema) error {
	if s.Type != "object" {
		return fmt.Errorf("%s ErrorDetail must be object", scope)
	}
	required := makeSet(s.Required)
	if !required["reason"] {
		return fmt.Errorf("%s ErrorDetail.required must include \"reason\"", scope)
	}
	reasonProp, ok := s.Properties["reason"]
	if !ok || reasonProp.Type != "string" {
		return fmt.Errorf("%s ErrorDetail.reason must be string", scope)
	}
	return nil
}

func shapeFromSchema(s schema) schemaShape {
	out := schemaShape{
		Type:       s.Type,
		Required:   append([]string(nil), s.Required...),
		Properties: make(map[string]propertyShape, len(s.Properties)),
	}
	sort.Strings(out.Required)
	for name, prop := range s.Properties {
		shape := propertyShape{Type: prop.Type}
		if prop.Items != nil {
			shape.ItemsRef = strings.TrimSpace(prop.Items.Ref)
		}
		out.Properties[name] = shape
	}
	return out
}

func ensureSameShape(name string, left, right schemaShape) error {
	if left.Type != right.Type {
		return fmt.Errorf("%s type mismatch: %q vs %q", name, left.Type, right.Type)
	}
	if strings.Join(left.Required, ",") != strings.Join(right.Required, ",") {
		return fmt.Errorf("%s required mismatch: %v vs %v", name, left.Required, right.Required)
	}
	if len(left.Properties) != len(right.Properties) {
		return fmt.Errorf("%s property count mismatch: %d vs %d", name, len(left.Properties), len(right.Properties))
	}
	for key, leftProp := range left.Properties {
		rightProp, ok := right.Properties[key]
		if !ok {
			return fmt.Errorf("%s missing property %q in internal schema", name, key)
		}
		if leftProp != rightProp {
			return fmt.Errorf("%s property %q mismatch: %+v vs %+v", name, key, leftProp, rightProp)
		}
	}
	return nil
}

func makeSet(items []string) map[string]bool {
	out := make(map[string]bool, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		out[item] = true
	}
	return out
}

func exitErr(err error) {
	fmt.Fprintln(os.Stderr, err.Error())
	os.Exit(1)
}
