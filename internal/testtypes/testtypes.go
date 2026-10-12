// Package testtypes holds fixture structs used by json2define's tests and
// integration tests.
package testtypes

import "time"

// Nested is a simple nested struct.
type Nested struct {
	Flag  bool    `json:"flag"`
	Score float64 `json:"score"`
}

// Embedded is embedded into Sample to exercise promoted fields.
type Embedded struct {
	Emb string `json:"emb"`
}

// Sample covers the type shapes json2define is expected to handle.
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

// FeatureFrom is a string enum whose constants are registered in tests.
type FeatureFrom string

// Feature sources.
const (
	FeatureFromRequest FeatureFrom = "request"
	FeatureFromRedis   FeatureFrom = "redis"
	FeatureFromOther   FeatureFrom = "other"
)

// FeatureType is a string enum whose constants are registered in tests.
type FeatureType string

// Feature value types.
const (
	FeatureTypeFloat32 FeatureType = "float32"
	FeatureTypeString  FeatureType = "string"
	FeatureTypeBool    FeatureType = "bool"
)

// Level is an integer enum whose constants are registered in tests.
type Level int

// Feature levels.
const (
	LevelLow Level = iota
	LevelHigh
)

// Features exercises enum fields, including an enum used as a map key.
type Features struct {
	From  FeatureFrom         `json:"from"`
	Type  FeatureType         `json:"type"`
	Level Level               `json:"level"`
	ByKey map[FeatureFrom]int `json:"by_key"`
}

// FeatureSpec is a small, scalar-only struct used to exercise
// [WithCompactStructs], typically as a slice element.
type FeatureSpec struct {
	Field   string      `json:"field"`
	From    FeatureFrom `json:"from"`
	Type    FeatureType `json:"type"`
	Default int         `json:"default"`
}

// FeatureList wraps a slice of FeatureSpec.
type FeatureList struct {
	Features []FeatureSpec `json:"features"`
}
