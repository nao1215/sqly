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
	bytes, err := os.ReadFile(filepath.Clean(jsonFilePath))
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

// Dump write contents of DB table to JSON file
func (r *jsonRepository) Dump(f *os.File, table *model.Table) error {
	data := make([]map[string]interface{}, 0)

	for _, v := range table.Records {
		d := make(map[string]interface{}, 0)
		for i, r := range v {
			d[table.Header[i]] = r
		}
		data = append(data, d)
	}
	b, err := json.MarshalIndent(data, "", "   ")
	if err != nil {
		return err
	}
	_, err = f.Write(b)
	return err
}
