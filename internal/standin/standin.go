// Package standin holds reflection-only mirrors of the structs declared in
// package testtypes. The types here are structurally identical to their
// testtypes counterparts; only the package they live in differs. They exist to
// exercise [json2define.WithPackageRemap].
package standin

import "time"

// Nested mirrors testtypes.Nested.
type Nested struct {
	Flag  bool    `json:"flag"`
	Score float64 `json:"score"`
}

// Embedded mirrors testtypes.Embedded.
type Embedded struct {
	Emb string `json:"emb"`
}

// Sample mirrors testtypes.Sample.
type Sample struct {
	Name    string            `json:"name"`
	Age     int               `json:"age"`
	Ratio   float32           `json:"ratio"`
	Active  bool              `json:"active"`
	Tags    []string          `json:"tags"`
	Meta    map[string]string `json:"meta"`
	Scores  map[int]float64   `json:"scores"`
	Nested  Nested            `json:"nested"`
	PtrInt  *int              `json:"ptr_int"`
	PtrStr  *string           `json:"ptr_str"`
	Any     any               `json:"any"`
	Bytes   []byte            `json:"bytes"`
	When    time.Time         `json:"when"`
	Ignored string            `json:"-"`
	Embedded
}

// FeatureFrom mirrors testtypes.FeatureFrom.
type FeatureFrom string

// FeatureType mirrors testtypes.FeatureType.
type FeatureType string

// Level mirrors testtypes.Level.
type Level int

// LevelHigh mirrors testtypes.LevelHigh so that a constant declared in a
// remapped package can be rendered against the real one.
const LevelHigh Level = 1

// Features mirrors testtypes.Features.
type Features struct {
	From  FeatureFrom         `json:"from"`
	Type  FeatureType         `json:"type"`
	Level Level               `json:"level"`
	ByKey map[FeatureFrom]int `json:"by_key"`
}

// FeatureSpec mirrors testtypes.FeatureSpec.
type FeatureSpec struct {
	Field   string      `json:"field"`
	From    FeatureFrom `json:"from"`
	Type    FeatureType `json:"type"`
	Default int         `json:"default"`
}

// FeatureList mirrors testtypes.FeatureList.
type FeatureList struct {
	Features []FeatureSpec `json:"features"`
}
