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
