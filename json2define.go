// Package json2define turns arbitrary JSON into compilable Go
// initialization code.
//
// The target Go type is supplied through reflection, so the generated
// composite literal is guaranteed to use the field names and field types
// of that type. The result is a complete, gofmt-ed source file that can be
// dropped into a package and compiled as-is.
package json2define

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"go/format"
	"path"
	"reflect"
	"strings"
)

// Generator holds the configuration and the state used to build Go source.
//
// A Generator is not safe for concurrent use; create one per generation
// with [New].
type Generator struct {
	packageName string
	varName     string
	samePkg     string
	remaps      map[string]packageRemap
	enums       map[enumKey]map[string]enumConst
	compact     func(reflect.Type) bool

	imports *importManager
}

// packageRemap records how a stand-in package used only for reflection should
// be rendered in the generated source.
type packageRemap struct {
	path string // real import path
	name string // real package name used as the qualifier
}

// enumKey identifies a named type by the package path and name reflection
// reports for it, so that it lines up with any [WithPackageRemap] applied to
// the same package.
type enumKey struct {
	pkgPath string
	name    string
}

// enumConst records the Go constant to emit for one enum value. pkgPath is the
// package that declares the constant, expressed the way reflection reports it
// (so remaps apply); an empty pkgPath means the constant lives next to the
// enum type.
type enumConst struct {
	pkgPath string
	name    string
}

// Option customizes a [Generator].
type Option func(*Generator)

// WithPackageName sets the package clause of the generated file.
// The default is "main".
func WithPackageName(name string) Option {
	return func(g *Generator) { g.packageName = name }
}

// WithVarName sets the name of the generated variable. The default is
// "value".
func WithVarName(name string) Option {
	return func(g *Generator) { g.varName = name }
}

// WithSamePackagePath marks pkgPath as the package that will contain the
// generated file. Types declared in that package are emitted without a
// package qualifier and are not imported.
func WithSamePackagePath(pkgPath string) Option {
	return func(g *Generator) { g.samePkg = pkgPath }
}

// WithPackageRemap makes every named type that reflection reports as
// belonging to the stand-in package fromPath be rendered as if it were
// declared in the real package toPath.
//
// It lets callers reflect over a local mirror of a type -- a copy declared in
// a dependency-free package -- while still generating a literal whose type is
// the real one, so the generator never has to import the real package (and
// everything that package drags in). The mirror must match the real package's
// types exactly; json2define only ever sees the mirror.
//
// The qualifier is taken from the last element of toPath, and the real package
// is imported unless toPath matches [WithSamePackagePath]. Remapping is applied
// before the same-package check, so WithSamePackagePath should be given the
// real path, not the stand-in path.
func WithPackageRemap(fromPath, toPath string) Option {
	return func(g *Generator) {
		if g.remaps == nil {
			g.remaps = make(map[string]packageRemap)
		}
		g.remaps[fromPath] = packageRemap{path: toPath, name: path.Base(toPath)}
	}
}

// WithEnumConstants registers named constants for the enum type t, a defined
// string, integer, or bool type.
//
// Each key is the value as it appears in the JSON and each value is the
// identifier of the constant to emit instead of a conversion. The constant is
// assumed to be declared next to t, so it is qualified exactly like t: package
// remaps and the same-package rule (see [WithPackageRemap] and
// [WithSamePackagePath]) apply unchanged.
//
// For example, registering {"request": "FeatureFromRequest"} for
// example.FeatureFrom makes the generator emit example.FeatureFromRequest where it
// would otherwise emit example.FeatureFrom("request"). Because t carries the
// reflected package path, register with the same (possibly stand-in) type the
// generator reflects over.
func WithEnumConstants(t reflect.Type, constants map[string]string) Option {
	return func(g *Generator) {
		for value, name := range constants {
			g.addEnumConstant(enumKey{t.PkgPath(), t.Name()}, value, "", name)
		}
	}
}

// WithEnumConstantIn registers a single enum value -> constant mapping where the
// constant is declared in a different package than t. pkgPath is the import path
// of that package as reflection reports it, so [WithPackageRemap] applies to it
// as well; pass "" to declare the constant next to t (the same as
// [WithEnumConstants]).
func WithEnumConstantIn(t reflect.Type, value, pkgPath, name string) Option {
	return func(g *Generator) {
		g.addEnumConstant(enumKey{t.PkgPath(), t.Name()}, value, pkgPath, name)
	}
}

func (g *Generator) addEnumConstant(key enumKey, value, pkgPath, name string) {
	if g.enums == nil {
		g.enums = make(map[enumKey]map[string]enumConst)
	}
	if g.enums[key] == nil {
		g.enums[key] = make(map[string]enumConst)
	}
	g.enums[key][value] = enumConst{pkgPath: pkgPath, name: name}
}

// WithCompactStructs renders struct literals selected by pred on a single line,
// for example:
//
//	{Field: "af_media_source", From: spec.FeatureFromRequest, Type: spec.FeatureTypeString, Default: -1}
//
// instead of the default one-field-per-line form:
//
//	spec.Feature{
//		Field:   "af_media_source",
//		From:    spec.FeatureFromRequest,
//		Type:    spec.FeatureTypeString,
//		Default: -1,
//	}
//
// Only structs are eligible: an enclosing slice, array, or map keeps the
// multi-line form, but its elements are inlined when pred accepts the element
// type. Because Go lets the element type be elided in such literals, a
// selected struct used as a slice element is emitted as `{...}`:
//
//	Features: []spec.Feature{
//		{Field: "af_media_source", From: spec.FeatureFromRequest, Type: spec.FeatureTypeString, Default: -1},
//		{Field: "af_other", From: spec.FeatureFromRedis, Type: spec.FeatureTypeBool, Default: 0},
//	}
//
// A struct stays multi-line when any of its fields renders on more than one
// line (a slice field, for example), so nested values never get crammed onto a
// single line. Use pred to select small, leaf structs -- feature descriptors,
// table rows -- and leave larger or nested types expanded.
func WithCompactStructs(pred func(reflect.Type) bool) Option {
	return func(g *Generator) { g.compact = pred }
}

// New returns a Generator configured with opts.
func New(opts ...Option) *Generator {
	g := &Generator{
		packageName: "main",
		varName:     "value",
		imports:     newImportManager(),
	}
	for _, opt := range opts {
		opt(g)
	}
	return g
}

// Generate builds a complete, gofmt-ed Go source file declaring a variable
// of type t initialized from data.
func (g *Generator) Generate(t reflect.Type, data []byte) (string, error) {
	if t == nil {
		return "", errors.New("json2define: nil target type")
	}
	value, err := g.decode(data)
	if err != nil {
		return "", err
	}
	expr, err := g.buildExpr(t, value)
	if err != nil {
		return "", err
	}
	typeName := g.typeString(t)

	var b strings.Builder
	b.WriteString("// Code generated by json2define; DO NOT EDIT.\n\n")
	fmt.Fprintf(&b, "package %s\n\n", g.packageName)
	if imports := g.imports.render(); imports != "" {
		b.WriteString(imports)
		b.WriteString("\n")
	}
	fmt.Fprintf(&b, "var %s %s = %s\n", g.varName, typeName, expr)

	src := b.String()
	formatted, err := format.Source([]byte(src))
	if err != nil {
		return src, fmt.Errorf("json2define: generated source is invalid: %w", err)
	}
	return string(formatted), nil
}

// Generate creates a new Generator configured with opts and generates a
// variable of type t from data.
func Generate(t reflect.Type, data []byte, opts ...Option) (string, error) {
	return New(opts...).Generate(t, data)
}

// GenerateFor creates a new Generator configured with opts and generates a
// variable of type T from data.
func GenerateFor[T any](data []byte, opts ...Option) (string, error) {
	return Generate(reflect.TypeOf((*T)(nil)).Elem(), data, opts...)
}

func (g *Generator) decode(data []byte) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, fmt.Errorf("json2define: invalid JSON: %w", err)
	}
	if dec.More() {
		return nil, errors.New("json2define: unexpected trailing data after JSON value")
	}
	return v, nil
}
