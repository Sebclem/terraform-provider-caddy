package provider

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"sort"
	"strings"
)

type privateReader interface {
	GetKey(context.Context, string) ([]byte, diag.Diagnostics)
}
type privateWriter interface {
	SetKey(context.Context, string, []byte) diag.Diagnostics
}

func ownedFields(ctx context.Context, p privateReader) Ownership {
	o := Ownership{}
	if p != nil {
		data, _ := p.GetKey(ctx, "owned")
		_ = json.Unmarshal(data, &o)
	}
	return o
}

func saveOwned(ctx context.Context, p privateWriter, o Ownership) diag.Diagnostics {
	data, _ := json.Marshal(o)
	return p.SetKey(ctx, "owned", data)
}

func ownershipRevision(o Ownership) string {
	var keys []string
	for k, v := range o {
		if v && !strings.Contains(k, ".") {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	sum := sha256.Sum256([]byte(strings.Join(keys, ",")))
	return fmt.Sprintf("%x", sum[:8])
}
