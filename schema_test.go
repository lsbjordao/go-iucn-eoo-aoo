package eooaoo

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// schemaChecker validates JSON against the subset of JSON Schema used in
// schemas/: type, required, properties, additionalProperties, items, enum,
// const, $ref, oneOf, anyOf and dependentRequired. It exists so the published
// schemas cannot silently drift from the structs that produce the output.
type schemaChecker struct {
	t    *testing.T
	docs map[string]map[string]any
}

func (c *schemaChecker) doc(name string) map[string]any {
	if d, ok := c.docs[name]; ok {
		return d
	}
	b, err := os.ReadFile(filepath.Join("schemas", name))
	if err != nil {
		c.t.Fatal(err)
	}
	var d map[string]any
	if err := json.Unmarshal(b, &d); err != nil {
		c.t.Fatalf("%s: %v", name, err)
	}
	c.docs[name] = d
	return d
}

func (c *schemaChecker) resolve(file, ref string) (string, map[string]any) {
	target, frag, _ := strings.Cut(ref, "#")
	if target != "" {
		file = target
	}
	node := c.doc(file)
	for _, part := range strings.Split(strings.Trim(frag, "/"), "/") {
		if part != "" {
			node = node[part].(map[string]any)
		}
	}
	return file, node
}

func jsonType(v any) string {
	switch x := v.(type) {
	case nil:
		return "null"
	case bool:
		return "boolean"
	case string:
		return "string"
	case float64:
		if x == math.Trunc(x) {
			return "integer"
		}
		return "number"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	}
	return fmt.Sprintf("%T", v)
}

func (c *schemaChecker) check(file string, s map[string]any, v any, path string) error {
	if ref, ok := s["$ref"].(string); ok {
		f, n := c.resolve(file, ref)
		return c.check(f, n, v, path)
	}
	for _, key := range []string{"oneOf", "anyOf"} {
		if alts, ok := s[key].([]any); ok {
			n := 0
			for _, a := range alts {
				if c.check(file, a.(map[string]any), v, path) == nil {
					n++
				}
			}
			if n == 0 || (key == "oneOf" && n != 1) {
				return fmt.Errorf("%s: %s matched %d alternatives", path, key, n)
			}
		}
	}
	if t, ok := s["type"]; ok {
		got, ok := jsonType(v), false
		var want []any
		if l, isList := t.([]any); isList {
			want = l
		} else {
			want = []any{t}
		}
		for _, w := range want {
			ok = ok || w == got || (w == "number" && got == "integer")
		}
		if !ok {
			return fmt.Errorf("%s: type %s, want %v", path, got, t)
		}
	}
	if e, ok := s["enum"].([]any); ok {
		found := false
		for _, x := range e {
			found = found || reflect.DeepEqual(x, v)
		}
		if !found {
			return fmt.Errorf("%s: %v not in enum %v", path, v, e)
		}
	}
	if k, ok := s["const"]; ok && !reflect.DeepEqual(k, v) {
		return fmt.Errorf("%s: %v != const %v", path, v, k)
	}
	if obj, ok := v.(map[string]any); ok {
		props, _ := s["properties"].(map[string]any)
		for _, r := range asList(s["required"]) {
			if _, ok := obj[r.(string)]; !ok {
				return fmt.Errorf("%s: missing required %q", path, r)
			}
		}
		if dr, ok := s["dependentRequired"].(map[string]any); ok {
			for k, need := range dr {
				if _, has := obj[k]; has {
					for _, n := range asList(need) {
						if _, ok := obj[n.(string)]; !ok {
							return fmt.Errorf("%s: %q requires %q", path, k, n)
						}
					}
				}
			}
		}
		for k, val := range obj {
			ps, ok := props[k].(map[string]any)
			if !ok {
				switch ap := s["additionalProperties"].(type) {
				case bool:
					if !ap {
						return fmt.Errorf("%s: property %q not in schema", path, k)
					}
					continue
				case map[string]any:
					ps = ap
				default:
					continue
				}
			}
			if err := c.check(file, ps, val, path+"."+k); err != nil {
				return err
			}
		}
	}
	if arr, ok := v.([]any); ok {
		if items, ok := s["items"].(map[string]any); ok {
			for i, x := range arr {
				if err := c.check(file, items, x, fmt.Sprintf("%s[%d]", path, i)); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func asList(v any) []any { l, _ := v.([]any); return l }

func (c *schemaChecker) validate(file string, value any) {
	c.t.Helper()
	b, err := json.Marshal(value)
	if err != nil {
		c.t.Fatal(err)
	}
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		c.t.Fatal(err)
	}
	if err := c.check(file, c.doc(file), v, "$"); err != nil {
		c.t.Fatalf("%s: %v", file, err)
	}
}

func TestOutputsMatchPublishedSchemas(t *testing.T) {
	c := &schemaChecker{t: t, docs: map[string]map[string]any{}}
	p := zeroZeroPoints()
	full := mustCalc(t, p, Config{})
	c.validate("result.schema.json", full)
	c.validate("result.schema.json", full.Summary())
	c.validate("result.schema.json", mustCalc(t, p[:1], Config{GridMode: "sampled"}))
	// The validator must reject undeclared fields, or this test proves nothing.
	if err := c.check("result.schema.json", c.doc("result.schema.json"), map[string]any{"unexpected": 1}, "$"); err == nil {
		t.Fatal("schema checker accepted an undeclared property")
	}
	func() {
		withWorkLimit(t, 100)
		defer func() { gridWorkLimit = 25000000 }()
		fb := mustCalc(t, p, Config{GridSteps: 2})
		if fb.AOO.FallbackReason == "" {
			t.Fatalf("expected a fallback result: %+v", fb.AOO)
		}
		c.validate("result.schema.json", fb)
	}()
	c.validate("protocol.schema.json", *full.Protocol)

	cmp, err := CompareProjections(context.Background(), p, Config{})
	if err != nil {
		t.Fatal(err)
	}
	c.validate("comparison.schema.json", cmp)

	two := 2
	audit, err := AuditOccurrences(context.Background(), append(auditRecords(p), AuditRecord{Point: Point{X: math.NaN(), Y: math.NaN()}, SourceRow: 9, ParseError: "record: bad"}), AuditConfig{CoarseDecimals: &two, SourceFlagField: "issue", UncertaintyField: "u", ContextFields: []string{"locality"}})
	if err != nil {
		t.Fatal(err)
	}
	c.validate("audit.schema.json", audit)

	m := zeroZeroManifest(t, p)
	c.validate("curation-manifest.schema.json", m)
	_, log, err := ApplyCuration(p, m)
	if err != nil {
		t.Fatal(err)
	}
	c.validate("curation-log.schema.json", log)

	inf, err := AnalyzeEOOInfluence(context.Background(), p, Config{}, InfluenceConfig{RecomputeProjection: true})
	if err != nil {
		t.Fatal(err)
	}
	c.validate("influence.schema.json", inf)
}
