package json2define

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/moozik/json2define/internal/testtypes"
)

// TestGeneratedCodeCompiles builds the generated source in a throwaway
// module and checks that the initialized value is byte-for-byte equivalent
// to what encoding/json would have produced.
func TestGeneratedCodeCompiles(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	modRoot, err := filepath.Abs(".")
	if err != nil {
		t.Fatal(err)
	}

	code, err := Generate(reflect.TypeOf(testtypes.Sample{}), []byte(sampleJSON), WithVarName("sample"))
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), `module github.com/moozik/json2define/tmpgen

go 1.22

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

const rawJSON = ` + "`" + sampleJSON + "`" + `

func main() {
	var want testtypes.Sample
	if err := json.Unmarshal([]byte(rawJSON), &want); err != nil {
		panic(err)
	}
	if !reflect.DeepEqual(sample, want) {
		got, _ := json.MarshalIndent(sample, "", "  ")
		fmt.Printf("value mismatch\nwant: %+v\ngot:  %+v\n%s\n", want, sample, got)
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
