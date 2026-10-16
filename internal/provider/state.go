package provider

import (
	"git.sebclem.fr/sebclem/terraform-provider-caddy/internal/caddy"
	"reflect"
	"sort"
)

type Ownership map[string]bool
type MutationIntent struct {
	Set    map[string]caddy.Value
	Remove []string
}

func PlanFields(config, live map[string]caddy.Value, previous Ownership) (MutationIntent, Ownership, error) {
	intent := MutationIntent{Set: map[string]any{}}
	next := Ownership{}
	for k, v := range config {
		next[k] = true
		if !reflect.DeepEqual(v, live[k]) {
			intent.Set[k] = v
		}
	}
	for k, owned := range previous {
		if _, ok := config[k]; owned && !ok {
			if _, exists := live[k]; exists {
				intent.Remove = append(intent.Remove, k)
			}
		}
	}
	sort.Strings(intent.Remove)
	return intent, next, nil
}
