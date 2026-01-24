package tasksource

import (
	"fmt"
	"strings"
)

// SourceSpec specifies how to create a task source.
type SourceSpec struct {
	Type   SourceType
	Config map[string]string
}

// ParseSourceSpec parses a source specification string.
// Format: "type:param1=value1,param2=value2"
// Examples:
//   - "todolist:path=tasks.txt"
//   - "beads:cwd=/path/to/project"
//   - "linear:team=TEAM123"
//   - "github:owner=user,repo=myrepo"
func ParseSourceSpec(spec string) (SourceSpec, error) {
	parts := strings.SplitN(spec, ":", 2)
	if len(parts) != 2 {
		return SourceSpec{}, fmt.Errorf("invalid source spec format: %s", spec)
	}

	sourceType := SourceType(parts[0])
	config := make(map[string]string)

	// Parse config params
	if parts[1] != "" {
		for _, param := range strings.Split(parts[1], ",") {
			kv := strings.SplitN(param, "=", 2)
			if len(kv) != 2 {
				return SourceSpec{}, fmt.Errorf("invalid parameter format: %s", param)
			}
			config[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
		}
	}

	return SourceSpec{
		Type:   sourceType,
		Config: config,
	}, nil
}

// CreateSource creates a TaskSource from a specification.
func CreateSource(spec SourceSpec) (TaskSource, error) {
	switch spec.Type {
	case SourceTypeTodolist:
		path, ok := spec.Config["path"]
		if !ok {
			return nil, fmt.Errorf("%w: todolist requires 'path' parameter", ErrInvalidConfig)
		}
		return NewTodolistSource(path)

	case SourceTypeBeads:
		cwd := spec.Config["cwd"]
		if cwd == "" {
			cwd = "." // Default to current directory
		}
		return NewBeadsSource(cwd)

	case SourceTypeLinear:
		return NewLinearSource(LinearConfig{
			APIKey: spec.Config["apiKey"],
			TeamID: spec.Config["team"],
		})

	case SourceTypeGitHub:
		owner, hasOwner := spec.Config["owner"]
		repo, hasRepo := spec.Config["repo"]
		if !hasOwner || !hasRepo {
			return nil, fmt.Errorf("%w: github requires 'owner' and 'repo' parameters", ErrInvalidConfig)
		}
		return NewGitHubSource(GitHubConfig{
			Token: spec.Config["token"],
			Owner: owner,
			Repo:  repo,
		})

	default:
		return nil, fmt.Errorf("unsupported source type: %s", spec.Type)
	}
}

// CreateMultiSourceFromSpecs creates a MultiSource from multiple specifications.
func CreateMultiSourceFromSpecs(specs []SourceSpec) (*MultiSource, error) {
	var sources []TaskSource

	for _, spec := range specs {
		source, err := CreateSource(spec)
		if err != nil {
			return nil, fmt.Errorf("failed to create source %s: %w", spec.Type, err)
		}
		sources = append(sources, source)
	}

	if len(sources) == 0 {
		return nil, fmt.Errorf("no sources specified")
	}

	return NewMultiSource(sources...), nil
}

// CreateMultiSourceFromStrings creates a MultiSource from string specifications.
func CreateMultiSourceFromStrings(specs []string) (*MultiSource, error) {
	var sourceSpecs []SourceSpec

	for _, specStr := range specs {
		spec, err := ParseSourceSpec(specStr)
		if err != nil {
			return nil, err
		}
		sourceSpecs = append(sourceSpecs, spec)
	}

	return CreateMultiSourceFromSpecs(sourceSpecs)
}
