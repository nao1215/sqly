package model

import (
	"sort"
	"strconv"
	"strings"
)

// JSON is json data with indefinite keys
type JSON struct {
	// name is json file name
	name string
	// json is key and value
	json []map[string]interface{}
}

// NewJSON create new JSON.
func NewJSON(name string, json []map[string]interface{}) *JSON {
	return &JSON{
		name: name,
		json: json,
	}
}

// ToTable convert JSON to Table.
func (j *JSON) ToTable() *Table {
	var keys []string
	for _, json := range j.json {
		for k := range json {
			keys = append(keys, k)
		}
	}
	header := sliceUnique(keys)
	sort.Strings(header)

	records := make([]Record, 0, len(j.json))
	for _, json := range j.json {
		r := Record{}
		for _, h := range header {
			if val, ok := json[h]; ok {
				switch v := val.(type) {
				case string:
					r = append(r, v)
				case float64:
					r = append(r, strconv.FormatFloat(v, 'f', -1, 64))
				case int:
					r = append(r, strconv.Itoa(v))
				// TODO: If value is array, convert to string.
				default:
					r = append(r, "")
				}
			} else {
				r = append(r, "")
			}
		}
		records = append(records, r)
	}

	return NewTable(strings.TrimSuffix(j.name, ".json"), header, records)
}

func sliceUnique(target []string) (unique []string) {
	m := map[string]bool{}

	for _, v := range target {
		if !m[v] {
			m[v] = true
			unique = append(unique, v)
		}
	}
	return unique
}
