package json2define

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/moozik/json2define/internal/standin"
	"github.com/moozik/json2define/internal/testtypes"
)

// TestGeneratedCodeCompiles builds the generated source in a throwaway
// module and checks that the initialized value is byte-for-byte equivalent
// to what encoding/json would have produced.
func TestGeneratedCodeCompiles(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	code, err := Generate(reflect.TypeOf(testtypes.Sample{}), []byte(sampleJSON), WithVarName("sample"))
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	compileAndRun(t, code, "sample", "testtypes.Sample", sampleJSON)
}

// TestGeneratedCodeCompilesWithPackageRemap reflects over the stand-in mirror,
// remaps it onto testtypes, and checks the result still compiles and matches
// encoding/json.
func TestGeneratedCodeCompilesWithPackageRemap(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	code, err := Generate(
		reflect.TypeOf(standin.Sample{}),
		[]byte(sampleJSON),
		WithVarName("sample"),
		WithPackageRemap(standinPkg, realPkg),
	)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	compileAndRun(t, code, "sample", "testtypes.Sample", sampleJSON)
}

// TestGeneratedCodeCompilesWithEnumConstants checks that generated enum
// constants compile and that the initialized value matches encoding/json.
func TestGeneratedCodeCompilesWithEnumConstants(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	opts := append([]Option{WithVarName("features")}, testtypesEnums()...)
	code, err := Generate(
		reflect.TypeOf(testtypes.Features{}),
		[]byte(featureJSON),
		opts...,
	)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	compileAndRun(t, code, "features", "testtypes.Features", featureJSON)
}

// TestGeneratedCodeCompilesWithEnumConstantsAndRemap reflects over the
// stand-in enums, remaps them onto testtypes, and checks the generated
// constants still compile and match encoding/json.
func TestGeneratedCodeCompilesWithEnumConstantsAndRemap(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	opts := append(standinEnums(), WithPackageRemap(standinPkg, realPkg))
	code, err := Generate(reflect.TypeOf(standin.Features{}), []byte(featureJSON), opts...)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	compileAndRun(t, code, "value", "testtypes.Features", featureJSON)
}

// TestGeneratedCodeCompilesWithCompactStructs checks that a struct rendered on
// one line, with its type name elided in a slice, still compiles and matches
// encoding/json.
func TestGeneratedCodeCompilesWithCompactStructs(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	opts := append([]Option{WithVarName("list")}, testtypesEnums()...)
	opts = append(opts, compactFeatureSpec())
	code, err := Generate(
		reflect.TypeOf(testtypes.FeatureList{}),
		[]byte(featureListJSON),
		opts...,
	)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	compileAndRun(t, code, "list", "testtypes.FeatureList", featureListJSON)
}

func compileAndRun(t *testing.T, code, varName, targetType, rawJSON string) {
	t.Helper()

	modRoot, err := filepath.Abs(".")
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), `module github.com/moozik/json2define/tmpgen

go 1.26

require github.com/moozik/json2define v0.0.0

replace github.com/moozik/json2define => `+modRoot+`
`)

	mainSrc := `package main

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"

	"github.com/moozik/json2define/internal/testtypes"
)

const rawJSON = ` + "`" + rawJSON + "`" + `

func main() {
	var want ` + targetType + `
	if err := json.Unmarshal([]byte(rawJSON), &want); err != nil {
		panic(err)
	}
	if !reflect.DeepEqual(` + varName + `, want) {
		got, _ := json.MarshalIndent(` + varName + `, "", "  ")
		fmt.Printf("value mismatch\nwant: %+v\ngot:  %+v\n%s\n", want, ` + varName + `, got)
		os.Exit(1)
	}
	fmt.Println("OK")
}
`
	writeFile(t, filepath.Join(dir, "main.go"), mainSrc)
	writeFile(t, filepath.Join(dir, "generated.go"), code)

	cmd := exec.Command("go", "run", ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOWORK=off", "GO111MODULE=on")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated code failed to build/run: %v\n%s\n--- generated ---\n%s", err, out, code)
	}
	if !strings.Contains(string(out), "OK") {
		t.Fatalf("unexpected program output: %s", out)
	}
}

func writeFile(t *testing.T, name, content string) {
	t.Helper()
	if err := os.WriteFile(name, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
