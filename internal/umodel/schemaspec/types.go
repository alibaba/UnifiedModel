package schemaspec

import "regexp"

type Schema struct {
	Name    string
	Version string
	Spec    Property
}

type Property struct {
	Type         string
	Properties   map[string]Property
	Constraint   *Constraint
	ReleaseStage string
}

type Constraint struct {
	Required     bool
	Enum         []any
	Pattern      *regexp.Regexp
	PatternStr   string
	MinLen       *int
	MaxLen       *int
	Array        *ArraySpec
	Map          *MapSpec
	DefaultValue any
}

type ArraySpec struct {
	Item    Property
	MinSize *int
	MaxSize *int
}

type MapSpec struct {
	Key     Property
	Value   Property
	MinSize *int
	MaxSize *int
}

type Result struct {
	Errors   []Issue
	Warnings []Issue
}

type Issue struct {
	Path   string
	Reason string
}
