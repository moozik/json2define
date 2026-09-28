package json2define

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"
)

var (
	timeType       = reflect.TypeOf(time.Time{})
	rawMessageType = reflect.TypeOf(json.RawMessage{})
)

// buildExpr renders v (a value decoded from JSON) as a Go expression of
// type t.
func (g *Generator) buildExpr(t reflect.Type, v any) (string, error) {
	if t == timeType {
		if v == nil {
			return g.typeString(t) + "{}", nil
		}
		return g.buildTime(v), nil
	}
	if v == nil {
		return g.zeroValue(t), nil
	}

	switch t.Kind() {
	case reflect.Pointer:
		inner, err := g.buildExpr(t.Elem(), v)
		if err != nil {
			return "", err
		}
		return "new(" + inner + ")", nil

	case reflect.Bool:
		b, ok := v.(bool)
		if !ok {
			return "", typeMismatch(t, v)
		}
		return g.scalar(t, strconv.FormatBool(b)), nil

	case reflect.String:
		s, ok := v.(string)
		if !ok {
			return "", typeMismatch(t, v)
		}
		return g.scalar(t, strconv.Quote(s)), nil

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return g.buildInt(t, v)

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return g.buildUint(t, v)

	case reflect.Float32, reflect.Float64:
		return g.buildFloat(t, v)

	case reflect.Slice:
		return g.buildSlice(t, v)

	case reflect.Array:
		return g.buildArray(t, v)

	case reflect.Map:
		return g.buildMap(t, v)

	case reflect.Struct:
		return g.buildStruct(t, v)

	case reflect.Interface:
		if t.NumMethod() != 0 {
			return "", fmt.Errorf("json2define: cannot construct a value of non-empty interface type %s", t)
		}
		return g.buildAny(v), nil

	default:
		return "", fmt.Errorf("json2define: unsupported type %s", t)
	}
}

func (g *Generator) buildInt(t reflect.Type, v any) (string, error) {
	n, ok := asNumber(v)
	if !ok {
		return "", typeMismatch(t, v)
	}
	if isIntegerLiteral(n) {
		return g.scalar(t, n), nil
	}
	f, err := strconv.ParseFloat(n, 64)
	if err != nil {
		return "", fmt.Errorf("json2define: invalid number %q for %s", n, t)
	}
	return g.scalar(t, strconv.FormatInt(int64(f), 10)), nil
}

func (g *Generator) buildUint(t reflect.Type, v any) (string, error) {
	n, ok := asNumber(v)
	if !ok {
		return "", typeMismatch(t, v)
	}
	if isIntegerLiteral(n) {
		return g.scalar(t, n), nil
	}
	f, err := strconv.ParseFloat(n, 64)
	if err != nil {
		return "", fmt.Errorf("json2define: invalid number %q for %s", n, t)
	}
	return g.scalar(t, strconv.FormatUint(uint64(f), 10)), nil
}

func (g *Generator) buildFloat(t reflect.Type, v any) (string, error) {
	n, ok := asNumber(v)
	if !ok {
		return "", typeMismatch(t, v)
	}
	return g.scalar(t, n), nil
}

func (g *Generator) buildSlice(t reflect.Type, v any) (string, error) {
	elem := t.Elem()
	if elem.Kind() == reflect.Uint8 && elem.PkgPath() == "" {
		if s, ok := v.(string); ok {
			q := strconv.Quote(s)
			if t != rawMessageType {
				if decoded, err := base64.StdEncoding.DecodeString(s); err == nil {
					q = strconv.Quote(string(decoded))
				}
			}
			return g.typeString(t) + "(" + q + ")", nil
		}
	}

	arr, ok := v.([]any)
	if !ok {
		return "", typeMismatch(t, v)
	}
	if len(arr) == 0 {
		return g.typeString(t) + "{}", nil
	}
	entries := make([]string, len(arr))
	for i, e := range arr {
		ex, err := g.buildExpr(elem, e)
		if err != nil {
			return "", err
		}
		entries[i] = ex
	}
	return composite(g.typeString(t), entries), nil
}

func (g *Generator) buildArray(t reflect.Type, v any) (string, error) {
	arr, ok := v.([]any)
	if !ok {
		return "", typeMismatch(t, v)
	}
	if len(arr) > t.Len() {
		return "", fmt.Errorf("json2define: type %s holds %d elements, JSON has %d", t, t.Len(), len(arr))
	}
	if len(arr) == 0 {
		return g.typeString(t) + "{}", nil
	}
	entries := make([]string, len(arr))
	for i, e := range arr {
		ex, err := g.buildExpr(t.Elem(), e)
		if err != nil {
			return "", err
		}
		entries[i] = ex
	}
	return composite(g.typeString(t), entries), nil
}

func (g *Generator) buildMap(t reflect.Type, v any) (string, error) {
	m, ok := v.(map[string]any)
	if !ok {
		return "", typeMismatch(t, v)
	}
	if len(m) == 0 {
		return g.typeString(t) + "{}", nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	entries := make([]string, 0, len(m))
	for _, k := range keys {
		key, err := g.buildMapKey(t.Key(), k)
		if err != nil {
			return "", err
		}
		val, err := g.buildExpr(t.Elem(), m[k])
		if err != nil {
			return "", err
		}
		entries = append(entries, key+": "+val)
	}
	return composite(g.typeString(t), entries), nil
}

func (g *Generator) buildMapKey(t reflect.Type, key string) (string, error) {
	switch t.Kind() {
	case reflect.String:
		return g.scalar(t, strconv.Quote(key)), nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if _, err := strconv.ParseInt(key, 10, 64); err != nil {
			return "", fmt.Errorf("json2define: map key %q is not a valid %s", key, t)
		}
		return g.scalar(t, key), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if _, err := strconv.ParseUint(key, 10, 64); err != nil {
			return "", fmt.Errorf("json2define: map key %q is not a valid %s", key, t)
		}
		return g.scalar(t, key), nil
	case reflect.Float32, reflect.Float64:
		if _, err := strconv.ParseFloat(key, 64); err != nil {
			return "", fmt.Errorf("json2define: map key %q is not a valid %s", key, t)
		}
		return g.scalar(t, key), nil
	case reflect.Bool:
		if _, err := strconv.ParseBool(key); err != nil {
			return "", fmt.Errorf("json2define: map key %q is not a valid %s", key, t)
		}
		return g.scalar(t, key), nil
	default:
		return "", fmt.Errorf("json2define: unsupported map key type %s", t)
	}
}

func (g *Generator) buildStruct(t reflect.Type, v any) (string, error) {
	m, ok := v.(map[string]any)
	if !ok {
		return "", typeMismatch(t, v)
	}
	entries, err := g.structEntries(t, m)
	if err != nil {
		return "", err
	}
	return composite(g.typeString(t), entries), nil
}

// structEntries resolves the JSON object m against the fields of struct
// type t and returns the keyed entries of a composite literal.
func (g *Generator) structEntries(t reflect.Type, m map[string]any) ([]string, error) {
	var entries []string
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.PkgPath != "" { // unexported
			continue
		}
		ft := parseFieldTag(f)
		if ft.skip {
			continue
		}

		if f.Anonymous && !ft.named {
			et := f.Type
			isPtr := false
			if et.Kind() == reflect.Pointer {
				et, isPtr = et.Elem(), true
			}
			if et.Kind() == reflect.Struct && et != timeType {
				sub, err := g.structEntries(et, m)
				if err != nil {
					return nil, err
				}
				if len(sub) == 0 {
					continue
				}
				val := composite(g.typeString(et), sub)
				if isPtr {
					val = "new(" + val + ")"
				}
				entries = append(entries, f.Name+": "+val)
				continue
			}
		}

		raw, ok := lookupJSON(m, f.Name, ft)
		if !ok {
			continue
		}
		val, err := g.fieldValue(f.Type, ft, raw)
		if err != nil {
			return nil, err
		}
		entries = append(entries, f.Name+": "+val)
	}
	return entries, nil
}

// fieldValue renders a single struct field value, honoring the ",string"
// JSON option.
func (g *Generator) fieldValue(t reflect.Type, ft fieldTag, v any) (string, error) {
	s, quoted := v.(string)
	if !ft.quoted || !quoted {
		return g.buildExpr(t, v)
	}

	switch t.Kind() {
	case reflect.String:
		return g.scalar(t, strconv.Quote(s)), nil
	case reflect.Bool:
		b, err := strconv.ParseBool(s)
		if err != nil {
			return "", fmt.Errorf("json2define: invalid bool %q for %s", s, t)
		}
		return g.scalar(t, strconv.FormatBool(b)), nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return "", fmt.Errorf("json2define: invalid int %q for %s", s, t)
		}
		return g.scalar(t, strconv.FormatInt(n, 10)), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		n, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			return "", fmt.Errorf("json2define: invalid uint %q for %s", s, t)
		}
		return g.scalar(t, strconv.FormatUint(n, 10)), nil
	case reflect.Float32, reflect.Float64:
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return "", fmt.Errorf("json2define: invalid float %q for %s", s, t)
		}
		return g.scalar(t, strconv.FormatFloat(f, 'g', -1, 64)), nil
	default:
		return "", fmt.Errorf("json2define: unsupported ,string field type %s", t)
	}
}

// buildAny renders a value for an empty interface field by inferring its Go
// type from the JSON. Numbers become float64 to match encoding/json's
// default decoding.
func (g *Generator) buildAny(v any) string {
	switch x := v.(type) {
	case nil:
		return "nil"
	case bool:
		return strconv.FormatBool(x)
	case string:
		return strconv.Quote(x)
	case json.Number:
		return "float64(" + x.String() + ")"
	case []any:
		parts := make([]string, len(x))
		for i, e := range x {
			parts[i] = g.buildAny(e)
		}
		return "[]any{" + strings.Join(parts, ", ") + "}"
	case map[string]any:
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		parts := make([]string, 0, len(x))
		for _, k := range keys {
			parts = append(parts, strconv.Quote(k)+": "+g.buildAny(x[k]))
		}
		return "map[string]any{" + strings.Join(parts, ", ") + "}"
	default:
		return fmt.Sprintf("%#v", x)
	}
}

// scalar wraps a literal in a conversion when t is a defined type, so that
// defined basic types such as time.Duration or a custom string remain
// type-correct.
func (g *Generator) scalar(t reflect.Type, literal string) string {
	if t.Name() != "" && t.PkgPath() != "" {
		return g.typeString(t) + "(" + literal + ")"
	}
	return literal
}

func (g *Generator) zeroValue(t reflect.Type) string {
	switch t.Kind() {
	case reflect.Pointer, reflect.Interface, reflect.Slice, reflect.Map, reflect.Func, reflect.Chan:
		return "nil"
	case reflect.Bool:
		return g.scalar(t, "false")
	case reflect.String:
		return g.scalar(t, `""`)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64, reflect.Complex64, reflect.Complex128:
		return g.scalar(t, "0")
	default:
		return g.typeString(t) + "{}"
	}
}

// typeString renders t as a Go type expression, registering imports for any
// referenced package.
func (g *Generator) typeString(t reflect.Type) string {
	if t == nil {
		return "any"
	}
	if t.Name() != "" {
		if t.PkgPath() == "" || t.PkgPath() == g.samePkg {
			return t.Name()
		}
		return g.imports.qualifier(t.PkgPath(), packageName(t)) + "." + t.Name()
	}
	switch t.Kind() {
	case reflect.Pointer:
		return "*" + g.typeString(t.Elem())
	case reflect.Slice:
		if t.Elem().Kind() == reflect.Uint8 && t.Elem().PkgPath() == "" && t.Elem().Name() == "uint8" {
			return "[]byte"
		}
		return "[]" + g.typeString(t.Elem())
	case reflect.Array:
		return "[" + strconv.Itoa(t.Len()) + "]" + g.typeString(t.Elem())
	case reflect.Map:
		return "map[" + g.typeString(t.Key()) + "]" + g.typeString(t.Elem())
	case reflect.Struct:
		return g.anonymousStruct(t)
	default:
		return t.String()
	}
}

// packageName returns the declared package name reflect uses to qualify
// named type t.
func packageName(t reflect.Type) string {
	s := t.String()
	if i := strings.LastIndexByte(s, '.'); i >= 0 {
		return s[:i]
	}
	return ""
}

func (g *Generator) anonymousStruct(t reflect.Type) string {
	var b strings.Builder
	b.WriteString("struct {")
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		b.WriteString(" ")
		b.WriteString(f.Name)
		b.WriteString(" ")
		b.WriteString(g.typeString(f.Type))
		if tag := string(f.Tag); tag != "" {
			b.WriteString(" ")
			b.WriteString(strconv.Quote(tag))
		}
		b.WriteString(";")
	}
	if t.NumField() > 0 {
		b.WriteString(" ")
	}
	b.WriteString("}")
	return b.String()
}

func (g *Generator) buildTime(v any) string {
	s, ok := v.(string)
	if !ok {
		return "time.Time{}"
	}
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02",
	}
	for _, layout := range layouts {
		tm, err := time.Parse(layout, s)
		if err != nil {
			continue
		}
		loc := "time.UTC"
		if name, offset := tm.Zone(); offset != 0 {
			loc = fmt.Sprintf("time.FixedZone(%q, %d)", name, offset)
		}
		g.imports.qualifier("time", "time")
		return fmt.Sprintf("time.Date(%d, time.Month(%d), %d, %d, %d, %d, %d, %s)",
			tm.Year(), int(tm.Month()), tm.Day(), tm.Hour(), tm.Minute(), tm.Second(), tm.Nanosecond(), loc)
	}
	return "time.Time{}"
}

// composite renders a (possibly keyed) list of entries as a composite
// literal. gofmt normalizes the indentation afterwards.
func composite(typeName string, entries []string) string {
	if len(entries) == 0 {
		return typeName + "{}"
	}
	var b strings.Builder
	b.WriteString(typeName)
	b.WriteString("{\n")
	for _, e := range entries {
		b.WriteString("\t")
		b.WriteString(e)
		b.WriteString(",\n")
	}
	b.WriteString("}")
	return b.String()
}

func asNumber(v any) (string, bool) {
	switch n := v.(type) {
	case json.Number:
		return n.String(), true
	case float64:
		return strconv.FormatFloat(n, 'g', -1, 64), true
	default:
		return "", false
	}
}

func isIntegerLiteral(s string) bool {
	if s == "" {
		return false
	}
	return !strings.ContainsAny(s, ".eE")
}

type fieldTag struct {
	name      string
	omitEmpty bool
	quoted    bool
	named     bool
	skip      bool
}

func parseFieldTag(f reflect.StructField) fieldTag {
	tag := f.Tag.Get("json")
	if tag == "-" {
		return fieldTag{skip: true}
	}
	ft := fieldTag{name: f.Name}
	if tag == "" {
		return ft
	}
	parts := strings.Split(tag, ",")
	if parts[0] != "" {
		ft.name = parts[0]
		ft.named = true
	}
	for _, opt := range parts[1:] {
		switch opt {
		case "omitempty":
			ft.omitEmpty = true
		case "string":
			ft.quoted = true
		}
	}
	return ft
}

func lookupJSON(m map[string]any, fieldName string, ft fieldTag) (any, bool) {
	if v, ok := m[ft.name]; ok {
		return v, true
	}
	if ft.named {
		return nil, false
	}
	for k, v := range m {
		if strings.EqualFold(k, fieldName) {
			return v, true
		}
	}
	return nil, false
}

func typeMismatch(t reflect.Type, v any) error {
	return fmt.Errorf("json2define: cannot use JSON value of type %T as %s", v, t)
}
