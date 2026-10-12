package json2define

import (
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/moozik/json2define/internal/standin"
	"github.com/moozik/json2define/internal/testtypes"
)

const (
	standinPkg = "github.com/moozik/json2define/internal/standin"
	realPkg    = "github.com/moozik/json2define/internal/testtypes"
)

var whitespace = regexp.MustCompile(`\s+`)

// contains reports whether out contains want, ignoring whitespace
// differences introduced by gofmt's column alignment.
func contains(out, want string) bool {
	return strings.Contains(whitespace.ReplaceAllString(out, " "), whitespace.ReplaceAllString(want, " "))
}

const sampleJSON = `{
	"name": "a\"b\n",
	"age": 30,
	"ratio": 1.25,
	"active": true,
	"tags": ["x", "y"],
	"meta": {"k": "v"},
	"scores": {"1": 2.5},
	"nested": {"flag": true, "score": 1.5},
	"ptr_int": 7,
	"ptr_str": null,
	"any": {"n": 1, "f": 2.5, "b": true, "a": [1, "s", null]},
	"bytes": "aGk=",
	"when": "2023-05-04T06:07:08Z",
	"emb": "e",
	"unknown": "ignored"
}`

func generateSample(t *testing.T, opts ...Option) string {
	t.Helper()
	out, err := Generate(reflect.TypeOf(testtypes.Sample{}), []byte(sampleJSON), opts...)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	return out
}

func TestGenerateFragments(t *testing.T) {
	out := generateSample(t)

	wants := []string{
		`Name: "a\"b\n"`,
		"Age: 30",
		"Ratio: 1.25",
		"Active: true",
		"Tags: []string{",
		`"x"`,
		"Scores: map[int]float64{",
		"1: 2.5",
		"Nested: testtypes.Nested{",
		"Flag: true",
		"PtrInt: new(7)",
		"PtrStr: nil",
		"Any: map[string]any{",
		"[]any{",
		"float64(1)",
		`Bytes: []byte("hi")`,
		"When: time.Date(2023, time.Month(5), 4, 6, 7, 8, 0, time.UTC)",
		"Embedded: testtypes.Embedded{",
		`Emb: "e"`,
		"package main",
		`"github.com/moozik/json2define/internal/testtypes"`,
		`"time"`,
	}
	for _, want := range wants {
		if !contains(out, want) {
			t.Errorf("generated code missing %q\n---\n%s", want, out)
		}
	}

	if strings.Contains(out, "Ignored") {
		t.Errorf("json:\"-\" field must be skipped:\n%s", out)
	}
	if strings.Contains(out, `unknown`) {
		t.Errorf("unknown JSON keys must be ignored:\n%s", out)
	}
}

func TestGenerateOptions(t *testing.T) {
	out := generateSample(t, WithPackageName("config"), WithVarName("sample"))
	if !strings.Contains(out, "package config") {
		t.Errorf("package clause not applied:\n%s", out)
	}
	if !contains(out, "var sample testtypes.Sample = testtypes.Sample{") {
		t.Errorf("var clause not applied:\n%s", out)
	}
}

func TestGenerateNilPointerAndEmptyCollections(t *testing.T) {
	type small struct {
		Ptr  *int           `json:"ptr"`
		Tags []string       `json:"tags"`
		Meta map[string]int `json:"meta"`
	}
	out, err := Generate(reflect.TypeOf(small{}), []byte(`{"ptr":null,"tags":[],"meta":{}}`),
		WithSamePackagePath("github.com/moozik/json2define"))
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	for _, want := range []string{"Ptr: nil", "Tags: []string{}", "Meta: map[string]int{}"} {
		if !contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestGenerateInterfaceNumbersAreFloat64(t *testing.T) {
	target := reflect.TypeOf((*any)(nil)).Elem()
	out, err := Generate(target, []byte(`{"n":1}`), WithSamePackagePath("x"))
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if !strings.Contains(out, "float64(1)") {
		t.Errorf("interfaces should decode numbers as float64:\n%s", out)
	}
}

func TestGenerateInvalidJSON(t *testing.T) {
	if _, err := Generate(reflect.TypeOf(testtypes.Sample{}), []byte(`{`)); err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestGenerateFor(t *testing.T) {
	out, err := GenerateFor[testtypes.Nested]([]byte(`{"flag":true,"score":2}`))
	if err != nil {
		t.Fatalf("GenerateFor: %v", err)
	}
	if !strings.Contains(out, "var value testtypes.Nested = testtypes.Nested{") {
		t.Errorf("unexpected output:\n%s", out)
	}
}

// TestGeneratePackageRemap reflects over the stand-in mirror but expects the
// real testtypes package to be referenced and imported.
func TestGeneratePackageRemap(t *testing.T) {
	out, err := Generate(
		reflect.TypeOf(standin.Sample{}),
		[]byte(sampleJSON),
		WithPackageRemap(standinPkg, realPkg),
	)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	wants := []string{
		"var value testtypes.Sample = testtypes.Sample{",
		"Nested: testtypes.Nested{",
		"Embedded: testtypes.Embedded{",
		`"github.com/moozik/json2define/internal/testtypes"`,
	}
	for _, want := range wants {
		if !contains(out, want) {
			t.Errorf("generated code missing %q\n---\n%s", want, out)
		}
	}
	if strings.Contains(out, "standin") {
		t.Errorf("stand-in package must not appear in generated code:\n%s", out)
	}
}

// TestGeneratePackageRemapSamePackage remaps the stand-in onto the package that
// will hold the generated file, so the qualifier is dropped entirely.
func TestGeneratePackageRemapSamePackage(t *testing.T) {
	out, err := Generate(
		reflect.TypeOf(standin.Nested{}),
		[]byte(`{"flag":true,"score":2}`),
		WithPackageRemap(standinPkg, realPkg),
		WithSamePackagePath(realPkg),
	)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if !strings.Contains(out, "var value Nested = Nested{") {
		t.Errorf("expected an unqualified type:\n%s", out)
	}
	if strings.Contains(out, "testtypes") || strings.Contains(out, "standin") {
		t.Errorf("same-package types must not be qualified:\n%s", out)
	}
}
