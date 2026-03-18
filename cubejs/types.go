// Package cubejs provides an HTTP client for the Cube.js REST API.
package cubejs

// cubeError is embedded in response types to capture the Cube.js error field
// (e.g., {"error": "Continue wait"}) without a second JSON unmarshal pass.
type cubeError struct {
	Error string `json:"error"`
}

// CubeError returns the error string from a Cube.js response, if any.
func (e cubeError) CubeError() string { return e.Error }

// Query represents a Cube.js query request.
type Query struct {
	Measures       []string          `json:"measures,omitempty"`
	Dimensions     []string          `json:"dimensions,omitempty"`
	Filters        []Filter          `json:"filters,omitempty"`
	TimeDimensions []TimeDimension   `json:"timeDimensions,omitempty"`
	Order          map[string]string `json:"order,omitempty"`
	Limit          *int              `json:"limit,omitempty"`
}

// Filter represents a Cube.js query filter.
type Filter struct {
	Member   string   `json:"member"`
	Operator string   `json:"operator"`
	Values   []string `json:"values,omitempty"`
}

// TimeDimension represents a Cube.js time dimension specification.
type TimeDimension struct {
	Dimension   string   `json:"dimension"`
	Granularity string   `json:"granularity,omitempty"`
	DateRange   []string `json:"dateRange,omitempty"`
}

// MetaResponse represents the response from GET /cubejs-api/v1/meta.
type MetaResponse struct {
	cubeError
	Cubes []CubeMeta `json:"cubes"`
}

// CubeMeta describes a single cube's metadata.
type CubeMeta struct {
	Name       string       `json:"name"`
	Title      string       `json:"title,omitempty"`
	Measures   []MemberMeta `json:"measures"`
	Dimensions []MemberMeta `json:"dimensions"`
	Segments   []SegmentMeta `json:"segments"`
	Joins      []JoinMeta   `json:"joins,omitempty"`
}

// MemberMeta describes a measure or dimension.
type MemberMeta struct {
	Name        string `json:"name"`
	Title       string `json:"title,omitempty"`
	Type        string `json:"type"`
	ShortTitle  string `json:"shortTitle,omitempty"`
	Description string `json:"description,omitempty"`
}

// SegmentMeta describes a segment.
type SegmentMeta struct {
	Name       string `json:"name"`
	Title      string `json:"title,omitempty"`
	ShortTitle string `json:"shortTitle,omitempty"`
}

// JoinMeta describes a join between cubes.
type JoinMeta struct {
	Name         string `json:"name"`
	Relationship string `json:"relationship"`
}

// LoadResponse represents the response from POST /cubejs-api/v1/load.
type LoadResponse struct {
	cubeError
	Data       []map[string]any `json:"data"`
	Annotation Annotation       `json:"annotation"`
}

// Annotation describes the query result metadata.
type Annotation struct {
	Measures       map[string]AnnotationMember `json:"measures,omitempty"`
	Dimensions     map[string]AnnotationMember `json:"dimensions,omitempty"`
	TimeDimensions map[string]AnnotationMember `json:"timeDimensions,omitempty"`
}

// AnnotationMember describes annotation info for a single member.
type AnnotationMember struct {
	Title      string `json:"title,omitempty"`
	ShortTitle string `json:"shortTitle,omitempty"`
	Type       string `json:"type,omitempty"`
}

// DryRunResponse represents the response from POST /cubejs-api/v1/dry-run.
type DryRunResponse struct {
	cubeError
	TransformedQuery TransformedQuery `json:"transformedQuery"`
	QueryType        string           `json:"queryType"`
}

// TransformedQuery describes how Cube.js transformed the incoming query.
type TransformedQuery struct {
	Measures       []string          `json:"measures,omitempty"`
	Dimensions     []string          `json:"dimensions,omitempty"`
	TimeDimensions []TimeDimension   `json:"timeDimensions,omitempty"`
	Order          map[string]string `json:"order,omitempty"`
	Limit          *int              `json:"limit,omitempty"`
	Filters        []Filter          `json:"filters,omitempty"`
}

// SQLResponse represents the response from POST /cubejs-api/v1/sql.
type SQLResponse struct {
	cubeError
	SQL SQL `json:"sql"`
}

// SQL contains the generated SQL query and parameters.
type SQL struct {
	Query  string `json:"query"`
	Params []any  `json:"params,omitempty"`
}
