package spec

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestWriteLoadRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "skv.cue")

	original := &Spec{
		Tools: &Tools{
			Exclude: []string{"opencode"},
		},
		Skills: []SkillEntry{
			{
				Name: "skill-foo",
				Repo: "https://example.com/skill-pack",
				Path: "skills/skill-foo",
				Ref:  "main",
			},
			{
				Name:  "local-bar",
				Local: "./.skv/skills/local-bar",
			},
		},
	}

	if err := Write(path, original); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("load spec: %v", err)
	}

	if !reflect.DeepEqual(original, loaded) {
		t.Fatalf("round-trip mismatch: %#v vs %#v", original, loaded)
	}
}
