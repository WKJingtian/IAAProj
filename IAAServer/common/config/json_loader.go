package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

// LoadJSONConfig reads a JSON file and unmarshals it into v.
func LoadJSONConfig(path string, v any) error {
	if strings.TrimSpace(path) == "" {
		return errors.New("config path cannot be empty")
	}
	if v == nil {
		return errors.New("target cannot be nil")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read config file %q failed: %w", path, err)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return fmt.Errorf("config file %q is empty", path)
	}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(v); err != nil {
		return fmt.Errorf("decode config file %q failed: %w", path, err)
	}

	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return fmt.Errorf("config file %q contains trailing non-JSON content", path)
	}

	return nil
}
