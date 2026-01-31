package spec

import (
	_ "embed"
	"fmt"
	"os"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

//go:embed skv.schema.cue
var schemaData string

// Spec mirrors the supported subset of skv.cue.
type Spec struct {
	Tools  *Tools       `json:"tools,omitempty"`
	Skills []SkillEntry `json:"skills"`
}

type Tools struct {
	Exclude []string `json:"exclude,omitempty"`
}

type SkillEntry struct {
	Name  string `json:"name"`
	Repo  string `json:"repo,omitempty"`
	Path  string `json:"path,omitempty"`
	Ref   string `json:"ref,omitempty"`
	Local string `json:"local,omitempty"`
}

func Load(path string) (*Spec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	ctx := cuecontext.New()
	v := ctx.CompileBytes(data, cue.Filename(path))
	if err := v.Err(); err != nil {
		return nil, err
	}

	schema := ctx.CompileBytes([]byte(schemaData), cue.Filename("skv.schema.cue"))
	if err := schema.Err(); err != nil {
		return nil, err
	}

	combined := schema.Unify(v)
	if err := combined.Validate(); err != nil {
		return nil, err
	}

	skvVal := combined.LookupPath(cue.ParsePath("skv"))
	if !skvVal.Exists() {
		return nil, fmt.Errorf("skv.cue: missing skv field")
	}

	var spec Spec
	if err := skvVal.Decode(&spec); err != nil {
		return nil, err
	}

	return &spec, nil
}

func Write(path string, spec *Spec) error {
	var b strings.Builder
	b.WriteString("skv: {\n")
	if spec.Tools != nil && len(spec.Tools.Exclude) > 0 {
		b.WriteString("  tools: {\n")
		b.WriteString("    exclude: [")
		for i, tool := range spec.Tools.Exclude {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(fmt.Sprintf("%q", tool))
		}
		b.WriteString("]\n")
		b.WriteString("  }\n")
	}
	b.WriteString("  skills: [\n")
	for _, skill := range spec.Skills {
		b.WriteString("    {\n")
		b.WriteString(fmt.Sprintf("      name: %q\n", skill.Name))
		if skill.Repo != "" {
			b.WriteString(fmt.Sprintf("      repo: %q\n", skill.Repo))
		}
		if skill.Path != "" {
			b.WriteString(fmt.Sprintf("      path: %q\n", skill.Path))
		}
		if skill.Ref != "" {
			b.WriteString(fmt.Sprintf("      ref: %q\n", skill.Ref))
		}
		if skill.Local != "" {
			b.WriteString(fmt.Sprintf("      local: %q\n", skill.Local))
		}
		b.WriteString("    },\n")
	}
	b.WriteString("  ]\n")
	b.WriteString("}\n")

	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func AppendSkill(path string, entry SkillEntry) error {
	spec, err := Load(path)
	if err != nil {
		return err
	}
	spec.Skills = append(spec.Skills, entry)
	return Write(path, spec)
}
