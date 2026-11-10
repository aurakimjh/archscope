package store

import (
	"encoding/json"
	"fmt"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
	"io"
	"os"
	"path/filepath"
)

const annotationFile = "annotations.json"

func ReadAnnotations(dir string) (map[string]models.HTTPAnnotation, error) {
	out := map[string]models.HTTPAnnotation{}
	file, err := os.Open(filepath.Join(dir, annotationFile))
	if os.IsNotExist(err) {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, 8<<20+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > 8<<20 {
		return nil, fmt.Errorf("annotation file exceeds 8 MiB")
	}
	if err = json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	if out == nil {
		out = map[string]models.HTTPAnnotation{}
	}
	return out, nil
}
func WriteAnnotations(dir string, values map[string]models.HTTPAnnotation) error {
	raw, err := json.Marshal(values)
	if err != nil {
		return err
	}
	if len(raw) > 8<<20 {
		return fmt.Errorf("annotations exceed 8 MiB")
	}
	file, err := os.CreateTemp(dir, ".annotations-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err = file.Write(raw); err != nil {
		file.Close()
		return err
	}
	if err = file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), filepath.Join(dir, annotationFile))
}
