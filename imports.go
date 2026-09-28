package json2define

import (
	"fmt"
	"path"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

// importManager tracks the packages referenced by generated code and hands
// out non-conflicting qualifiers for them.
type importManager struct {
	aliases  map[string]string // import path -> qualifier
	byAlias  map[string]string // qualifier -> import path
	declared map[string]string // import path -> package's declared name
}

func newImportManager() *importManager {
	return &importManager{
		aliases:  make(map[string]string),
		byAlias:  make(map[string]string),
		declared: make(map[string]string),
	}
}

// qualifier returns the qualifier to use for pkgPath, registering an import
// for it on first use. preferred is the package's declared name when known
// (for example "yaml" for the path "gopkg.in/yaml.v2"); it is only used to
// pick a readable, correct alias.
func (m *importManager) qualifier(pkgPath, preferred string) string {
	if pkgPath == "" {
		return ""
	}
	if q, ok := m.aliases[pkgPath]; ok {
		return q
	}
	declared := sanitizeIdentifier(preferred)
	base := declared
	if base == "" {
		base = sanitizeIdentifier(path.Base(pkgPath))
	}
	if base == "" {
		base = "pkg"
	}
	if declared == "" {
		declared = base
	}
	q := base
	for i := 2; ; i++ {
		if existing, ok := m.byAlias[q]; !ok || existing == pkgPath {
			break
		}
		q = base + strconv.Itoa(i)
	}
	m.aliases[pkgPath] = q
	m.byAlias[q] = pkgPath
	m.declared[pkgPath] = declared
	return q
}

// render returns an import declaration for every registered package, or an
// empty string when nothing was imported.
func (m *importManager) render() string {
	if len(m.aliases) == 0 {
		return ""
	}
	paths := make([]string, 0, len(m.aliases))
	for p := range m.aliases {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	var b strings.Builder
	b.WriteString("import (\n")
	for _, p := range paths {
		q := m.aliases[p]
		if q == m.declared[p] {
			fmt.Fprintf(&b, "\t%q\n", p)
		} else {
			fmt.Fprintf(&b, "\t%s %q\n", q, p)
		}
	}
	b.WriteString(")\n")
	return b.String()
}

// sanitizeIdentifier turns an arbitrary package base name into a valid Go
// identifier.
func sanitizeIdentifier(s string) string {
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || r == '_' || (b.Len() > 0 && unicode.IsDigit(r)) {
			b.WriteRune(r)
		}
	}
	out := b.String()
	if out == "" {
		return ""
	}
	if unicode.IsDigit(rune(out[0])) {
		out = "pkg" + out
	}
	return out
}
