package persistence

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/nao1215/sqly/domain/model"
	"github.com/nao1215/sqly/domain/repository"
)

type jsonRepository struct{}

// NewJSONRepository return JSONRepository
func NewJSONRepository() repository.JSONRepository {
	return &jsonRepository{}
}

func (r *jsonRepository) List(jsonFilePath string) (*model.JSON, error) {
	bytes, err := os.ReadFile(jsonFilePath)
	if err != nil {
		return nil, err
	}

	j := model.JSON{
		Name: filepath.Base(jsonFilePath),
		JSON: make([]map[string]interface{}, 0),
	}
	if err = json.Unmarshal(bytes, &j.JSON); err != nil {
		return nil, err
	}
	return &j, nil
}
