package lock

import (
	"encoding/json"
	"os"
)

type Lock struct {
	Skills []Skill `json:"skills"`
}

type Skill struct {
	Name     string   `json:"name"`
	Local    string   `json:"local,omitempty"`
	Repo     string   `json:"repo,omitempty"`
	Path     string   `json:"path,omitempty"`
	Ref      string   `json:"ref,omitempty"`
	Commit   string   `json:"commit,omitempty"`
	Checksum string   `json:"checksum"`
	License  *License `json:"license,omitempty"`
}

type License struct {
	SPDX string `json:"spdx,omitempty"`
	Path string `json:"path,omitempty"`
}

func Write(path string, lock *Lock) error {
	// Ensure Skills is never nil for consistent JSON output
	if lock.Skills == nil {
		lock.Skills = []Skill{}
	}
	data, err := json.MarshalIndent(lock, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

func Load(path string) (*Lock, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var lock Lock
	if err := json.Unmarshal(data, &lock); err != nil {
		return nil, err
	}
	return &lock, nil
}
