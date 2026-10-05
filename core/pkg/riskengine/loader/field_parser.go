package loader

// FieldPath represents a parsed field path
type FieldPath struct {
	Segments []PathSegment
}

// PathSegment represents a segment in a field path
type PathSegment struct {
	Name       string
	IsArray    bool
	ArrayIndex *int   // nil means all items ([])
	Field      string // For array items, the field to access
}
