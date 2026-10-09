package chains

import (
	"strings"
	"testing"
)

func TestExtractJSON(t *testing.T) {
	data := []byte(`{"status":"paid","data":{"object":{"id":"cs_1","amount_total":20000}}}`)
	if v, err := extractJSON(data, "status"); err != nil || v != "paid" {
		t.Fatalf("status: got %v, %v", v, err)
	}
	if v, err := extractJSON(data, "data.object.id"); err != nil || v != "cs_1" {
		t.Fatalf("id: got %v, %v", v, err)
	}
	if v, err := extractJSON(data, "data.object.amount_total"); err != nil || stringify(v) != "20000" {
		t.Fatalf("amount: got %v, %v", v, err)
	}
	if _, err := extractJSON(data, "nope.missing"); err == nil {
		t.Fatal("expected error for missing path")
	}
}

func TestResolveContext(t *testing.T) {
	rc := &runCtx{
		target: "http://localhost:3000",
		steps: map[string]StepResult{
			"checkout": {Status: 200, Body: []byte(`{"data":{"object":{"id":"cs_1"}}}`)},
		},
	}
	if got := rc.resolve("{{target}}/api/orders"); got != "http://localhost:3000/api/orders" {
		t.Fatalf("target: got %q", got)
	}
	if got := rc.resolve("{{steps.checkout.status}}"); got != "200" {
		t.Fatalf("status: got %q", got)
	}
	if got := rc.resolve("/orders/{{steps.checkout.body.data.object.id}}"); got != "/orders/cs_1" {
		t.Fatalf("body path: got %q", got)
	}
}

func TestEvalExpect(t *testing.T) {
	sr := StepResult{Status: 200, Body: []byte(`{"status":"paid"}`)}
	if errs := evalExpect(&Expect{Status: 200}, sr); len(errs) != 0 {
		t.Fatalf("200: got %v", errs)
	}
	if errs := evalExpect(&Expect{Status: "2xx"}, sr); len(errs) != 0 {
		t.Fatalf("2xx: got %v", errs)
	}
	if errs := evalExpect(&Expect{Status: 404}, sr); len(errs) == 0 {
		t.Fatal("404 should fail")
	}
	if errs := evalExpect(&Expect{Body: &BodyExpect{JSONPath: "status", Equals: "paid"}}, sr); len(errs) != 0 {
		t.Fatalf("body equals: got %v", errs)
	}
	if errs := evalExpect(&Expect{Body: &BodyExpect{JSONPath: "status", Equals: "unpaid"}}, sr); len(errs) == 0 {
		t.Fatal("body mismatch should fail")
	}
}

func TestEvalExpectExists(t *testing.T) {
	tr, fa := true, false
	sr := StepResult{Status: 200, Body: []byte(`{"status":"paid"}`)}

	if errs := evalExpect(&Expect{Body: &BodyExpect{JSONPath: "status", Exists: &tr}}, sr); len(errs) != 0 {
		t.Fatalf("exists true on present path: got %v", errs)
	}
	if errs := evalExpect(&Expect{Body: &BodyExpect{JSONPath: "missing", Exists: &fa}}, sr); len(errs) != 0 {
		t.Fatalf("exists false on absent path: got %v", errs)
	}
	if errs := evalExpect(&Expect{Body: &BodyExpect{JSONPath: "status", Exists: &fa}}, sr); len(errs) == 0 {
		t.Fatal("exists false on present path should fail")
	}
	if errs := evalExpect(&Expect{Body: &BodyExpect{JSONPath: "missing", Exists: &tr}}, sr); len(errs) == 0 {
		t.Fatal("exists true on absent path should fail")
	}
}

func TestEvalExpectAPIMessage(t *testing.T) {
	expect := &Expect{Body: &BodyExpect{JSONPath: "status", Equals: "paid"}}

	if errs := evalExpect(expect, StepResult{Status: 200, Body: []byte(`{"status":"paid"}`)}); len(errs) != 0 {
		t.Fatalf("paid should pass: got %v", errs)
	}

	sr := StepResult{Status: 200, Body: []byte(`{"status":200}`)}
	errs := evalExpect(expect, sr)
	if len(errs) == 0 {
		t.Fatal("{\"status\":200} should fail equals paid")
	}
	msg := errs[0]
	for _, want := range []string{"status", "paid", "200"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("error %q should mention %q", msg, want)
		}
	}
}
