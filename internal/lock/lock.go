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
	Repo     string   `json:"repo"`
	Path     string   `json:"path,omitempty"`
	Ref      string   `json:"ref,omitempty"`
	Commit   string   `json:"commit"`
	Checksum string   `json:"checksum"`
	License  *License `json:"license,omitempty"`
}

type License struct {
	SPDX string `json:"spdx,omitempty"`
	Path string `json:"path,omitempty"`
}

func Write(path string, lock *Lock) error {
	data, err := json.MarshalIndent(lock, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}
