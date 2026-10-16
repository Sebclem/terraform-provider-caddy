package provider

import (
	"fmt"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func matcherAttributes(negation bool) map[string]schema.Attribute {
	a := map[string]schema.Attribute{}
	for _, k := range []string{"hosts", "paths", "methods", "client_ip_ranges"} {
		a[k] = schema.ListAttribute{Optional: true, ElementType: types.StringType}
	}
	if negation {
		a["not"] = schema.ListNestedAttribute{Optional: true, NestedObject: schema.NestedAttributeObject{Attributes: matcherAttributes(false)}}
	}
	return a
}
func matcherTypes(negation bool) map[string]attr.Type {
	a := map[string]attr.Type{}
	for k, v := range matcherAttributes(negation) {
		a[k] = v.GetType()
	}
	return a
}

var matchTypes = matcherTypes(true)

func encodeMatchers(raw any) (any, error) {
	if raw == nil {
		return []any{}, nil
	}
	a := raw.([]any)
	out := make([]any, len(a))
	for i, v := range a {
		m := v.(map[string]any)
		obj := map[string]any{}
		for k, v := range m {
			switch k {
			case "hosts":
				obj["host"] = v
			case "paths":
				obj["path"] = v
			case "methods":
				obj["method"] = v
			case "client_ip_ranges":
				obj["client_ip"] = map[string]any{"ranges": v}
			case "not":
				n, e := encodeMatchers(v)
				if e != nil {
					return nil, e
				}
				obj["not"] = n
			}
		}
		if len(obj) == 0 {
			return nil, fmt.Errorf("matcher set must have at least one condition")
		}
		out[i] = obj
	}
	return out, nil
}
func decodeMatchers(raw any) (outcome any, err error) {
	defer func() {
		if recover() != nil {
			outcome = nil
			err = fmt.Errorf("unsupported matcher shape")
		}
	}()
	a, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("invalid matcher array")
	}
	out := make([]any, len(a))
	for i, v := range a {
		m, ok := v.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("invalid matcher object")
		}
		obj := map[string]any{}
		for k, v := range m {
			switch k {
			case "host":
				obj["hosts"] = v
			case "path":
				obj["paths"] = v
			case "method":
				obj["methods"] = v
			case "client_ip":
				obj["client_ip_ranges"] = v.(map[string]any)["ranges"]
			case "not":
				for _, inner := range v.([]any) {
					if _, exists := inner.(map[string]any)["not"]; exists {
						return nil, fmt.Errorf("only one negation level is supported")
					}
				}
				n, e := decodeMatchers(v)
				if e != nil {
					return nil, e
				}
				obj["not"] = n
			default:
				return nil, fmt.Errorf("unsupported matcher %s", k)
			}
		}
		out[i] = obj
	}
	return out, nil
}
