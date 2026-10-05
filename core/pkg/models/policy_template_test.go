package models

import (
	"encoding/json"
	"testing"
)

func TestJSONBHooksReplaceValuesPostgresRejects(t *testing.T) {
	tpl := &PolicyTemplate{}
	_ = tpl.BeforeSave(nil)
	if tpl.DefaultScope != "{}" || tpl.RemediationTemplate != "{}" || tpl.Examples != "[]" {
		t.Fatalf("template defaults: %+v", tpl)
	}

	inst := &PolicyInstance{}
	_ = inst.BeforeSave(nil)
	if inst.LabelSelectors != "{}" || inst.Exemptions != "[]" {
		t.Fatalf("instance defaults: %+v", inst)
	}

	for in, want := range map[string]string{
		"":             `{}`,
		`{"k":"v"}`:    `{"k":"v"}`,
		"role changed": `{"message":"role changed"}`,
	} {
		a := &AuditLog{Details: in}
		_ = a.BeforeSave(nil)
		if a.Details != want || !json.Valid([]byte(a.Details)) {
			t.Errorf("audit details %q -> %q, want %q", in, a.Details, want)
		}
	}
}
