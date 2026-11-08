package server

import (
	"reflect"
	"testing"

	"github.com/kindaMid72/bappelitbangda-sulselprov/api/internal/api"
)

// A3: every method of api.StrictServerInterface (reflection) has an operationRules entry,
// GetHealth and CreateSession are public, and no public rule requires a permission.
func TestOperationRulesCoverInterface(t *testing.T) {
	t.Parallel()

	iface := reflect.TypeOf((*api.StrictServerInterface)(nil)).Elem()
	if iface.NumMethod() == 0 {
		t.Fatal("StrictServerInterface has no methods")
	}

	for i := range iface.NumMethod() {
		methodName := iface.Method(i).Name
		rule, ok := operationRules[methodName]
		if !ok {
			t.Errorf("missing operationRules entry for interface method %q", methodName)
			continue
		}
		if rule.public && rule.perm != nil {
			t.Errorf("operation %q: cannot be both public and require permission (%v)", methodName, rule.perm)
		}
	}

	healthRule, ok := operationRules["GetHealth"]
	if !ok {
		t.Error("missing operationRules entry for GetHealth")
	} else if !healthRule.public {
		t.Error("operationRules[\"GetHealth\"].public must be true")
	}

	sessionRule, ok := operationRules["CreateSession"]
	if !ok {
		t.Error("missing operationRules entry for CreateSession")
	} else if !sessionRule.public {
		t.Error("operationRules[\"CreateSession\"].public must be true")
	}

	for name, r := range operationRules {
		if r.public && r.perm != nil {
			t.Errorf("operationRules[%q]: cannot be both public and require a permission", name)
		}
	}
}
