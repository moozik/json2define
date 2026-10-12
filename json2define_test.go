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

const featureJSON = `{"from":"request","type":"float32","level":1,"by_key":{"request":1,"redis":2}}`

// enumOptions registers the enum constants used by Features for the given
// (possibly stand-in) enum types. The key is the type reflection reports, so it
// must be the same mirror the generator reflects over.
func enumOptions(from, featureType, level reflect.Type) []Option {
	return []Option{
		WithEnumConstants(from, map[string]string{
			"request": "FeatureFromRequest",
			"redis":   "FeatureFromRedis",
			"other":   "FeatureFromOther",
		}),
		WithEnumConstants(featureType, map[string]string{
			"float32": "FeatureTypeFloat32",
			"string":  "FeatureTypeString",
			"bool":    "FeatureTypeBool",
		}),
		WithEnumConstants(level, map[string]string{
			"1": "LevelHigh",
		}),
	}
}

func testtypesEnums() []Option {
	return enumOptions(
		reflect.TypeOf(testtypes.FeatureFrom("")),
		reflect.TypeOf(testtypes.FeatureType("")),
		reflect.TypeOf(testtypes.Level(0)),
	)
}

func standinEnums() []Option {
	return enumOptions(
		reflect.TypeOf(standin.FeatureFrom("")),
		reflect.TypeOf(standin.FeatureType("")),
		reflect.TypeOf(standin.Level(0)),
	)
}

// TestGenerateEnumConstants replaces conversions such as
// testtypes.FeatureFrom("request") with the registered constant.
func TestGenerateEnumConstants(t *testing.T) {
	out, err := Generate(reflect.TypeOf(testtypes.Features{}), []byte(featureJSON), testtypesEnums()...)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	wants := []string{
		"From: testtypes.FeatureFromRequest",
		"Type: testtypes.FeatureTypeFloat32",
		"Level: testtypes.LevelHigh",
		"testtypes.FeatureFromRequest: 1",
		"testtypes.FeatureFromRedis: 2",
	}
	for _, want := range wants {
		if !contains(out, want) {
			t.Errorf("generated code missing %q\n---\n%s", want, out)
		}
	}
	for _, notWant := range []string{`FeatureFrom("request")`, `FeatureType("float32")`, "Level(1)"} {
		if strings.Contains(out, notWant) {
			t.Errorf("enum conversion %q must be replaced by a constant:\n%s", notWant, out)
		}
	}
}

// TestGenerateEnumConstantsStringTag covers the ",string" field path.
func TestGenerateEnumConstantsStringTag(t *testing.T) {
	type target struct {
		From testtypes.FeatureFrom `json:"from,string"`
	}
	out, err := Generate(reflect.TypeOf(target{}), []byte(`{"from":"request"}`), testtypesEnums()...)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if !contains(out, "From: testtypes.FeatureFromRequest") {
		t.Errorf("expected enum constant for a ,string field:\n%s", out)
	}
}

// TestGenerateEnumConstantsWithRemap reflects over the stand-in enums and
// expects the constants to be qualified with, and imported from, the real
// package.
func TestGenerateEnumConstantsWithRemap(t *testing.T) {
	opts := append(standinEnums(), WithPackageRemap(standinPkg, realPkg))
	out, err := Generate(reflect.TypeOf(standin.Features{}), []byte(featureJSON), opts...)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	wants := []string{
		"From: testtypes.FeatureFromRequest",
		"Type: testtypes.FeatureTypeFloat32",
		"Level: testtypes.LevelHigh",
		"testtypes.FeatureFromRedis: 2",
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

// TestGenerateEnumConstantsSamePackage remaps the stand-in onto the package
// that will hold the generated file, so constants are emitted unqualified.
func TestGenerateEnumConstantsSamePackage(t *testing.T) {
	opts := append(standinEnums(),
		WithPackageRemap(standinPkg, realPkg),
		WithSamePackagePath(realPkg),
	)
	out, err := Generate(reflect.TypeOf(standin.Features{}), []byte(featureJSON), opts...)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	for _, want := range []string{"From: FeatureFromRequest", "Level: LevelHigh"} {
		if !contains(out, want) {
			t.Errorf("generated code missing %q\n---\n%s", want, out)
		}
	}
	if strings.Contains(out, "testtypes") || strings.Contains(out, "standin") {
		t.Errorf("same-package constants must not be qualified:\n%s", out)
	}
}

// TestGenerateEnumConstantInOtherPackage registers a constant that lives in a
// package different from its enum type's. That package is itself remapped, so
// the constant must be rendered against the real package and import.
func TestGenerateEnumConstantInOtherPackage(t *testing.T) {
	out, err := Generate(
		reflect.TypeOf(testtypes.Features{}),
		[]byte(`{"level":1}`),
		WithEnumConstantIn(reflect.TypeOf(testtypes.Level(0)), "1", standinPkg, "LevelHigh"),
		WithPackageRemap(standinPkg, realPkg),
	)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if !contains(out, "Level: testtypes.LevelHigh") {
		t.Errorf("constant from a remapped package must use the real qualifier:\n%s", out)
	}
	if strings.Contains(out, "standin") {
		t.Errorf("stand-in package must not appear in generated code:\n%s", out)
	}
}

const featureListJSON = `{"features":[
	{"field":"af_media_source","from":"request","type":"string","default":-1},
	{"field":"af_other","from":"redis","type":"bool","default":0}
]}`

func compactFeatureSpec() Option {
	return WithCompactStructs(func(t reflect.Type) bool {
		return t == reflect.TypeOf(testtypes.FeatureSpec{})
	})
}

// TestGenerateCompactStructs renders a selected struct on a single line and
// elides its type name when it is a slice element, matching the concise
// []T{{...}} form.
func TestGenerateCompactStructs(t *testing.T) {
	opts := append(testtypesEnums(), compactFeatureSpec())
	out, err := Generate(reflect.TypeOf(testtypes.FeatureList{}), []byte(featureListJSON), opts...)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	want := `Features: []testtypes.FeatureSpec{
		{Field: "af_media_source", From: testtypes.FeatureFromRequest, Type: testtypes.FeatureTypeString, Default: -1},
		{Field: "af_other", From: testtypes.FeatureFromRedis, Type: testtypes.FeatureTypeBool, Default: 0},
	}`
	if !contains(out, want) {
		t.Errorf("compact structs not rendered as expected:\n%s", out)
	}
	// The selected struct must not keep the one-field-per-line layout.
	if strings.Contains(out, "Default: -1,\n\t\t}") {
		t.Errorf("selected struct must be inlined:\n%s", out)
	}
}

// TestGenerateWithoutCompactStructs confirms the default stays multi-line.
func TestGenerateWithoutCompactStructs(t *testing.T) {
	out, err := Generate(reflect.TypeOf(testtypes.FeatureList{}), []byte(featureListJSON), testtypesEnums()...)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if !contains(out, "testtypes.FeatureSpec{\n\t\t\tField:") {
		t.Errorf("expected the default multi-line literal:\n%s", out)
	}
}

// TestGenerateCompactStructsWithRemap selects a struct reflected from the
// stand-in package and expects the elided element type to use the real package.
func TestGenerateCompactStructsWithRemap(t *testing.T) {
	opts := append(standinEnums(),
		WithPackageRemap(standinPkg, realPkg),
		WithCompactStructs(func(t reflect.Type) bool {
			return t == reflect.TypeOf(standin.FeatureSpec{})
		}),
	)
	out, err := Generate(reflect.TypeOf(standin.FeatureList{}), []byte(featureListJSON), opts...)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	want := `Features: []testtypes.FeatureSpec{
		{Field: "af_media_source", From: testtypes.FeatureFromRequest, Type: testtypes.FeatureTypeString, Default: -1},
		{Field: "af_other", From: testtypes.FeatureFromRedis, Type: testtypes.FeatureTypeBool, Default: 0},
	}`
	if !contains(out, want) {
		t.Errorf("remapped compact structs not rendered as expected:\n%s", out)
	}
	if strings.Contains(out, "standin") {
		t.Errorf("stand-in package must not appear in generated code:\n%s", out)
	}
}
