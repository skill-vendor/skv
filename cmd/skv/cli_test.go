package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestRootHelpIncludesCommands(t *testing.T) {
	cmd := newRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute root: %v", err)
	}

	output := out.String()
	for _, fragment := range []string{"skv vendors skills", "init", "sync", "update", "verify", "import"} {
		if !strings.Contains(output, fragment) {
			t.Fatalf("expected help output to include %q, got:\n%s", fragment, output)
		}
	}
}

func TestAddRequiresRepoArg(t *testing.T) {
	cmd := newRootCmd()
	cmd.SetArgs([]string{"add"})

	if err := cmd.Execute(); err == nil || !errors.Is(err, errUsage) {
		t.Fatalf("expected usage error, got %v", err)
	}
}
