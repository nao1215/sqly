package model

import (
	"sort"
	"strconv"
	"strings"
)

// JSON is json data with indefinite keys
type JSON struct {
	// Name is json file name
	Name string
	// JSON is key and value
	JSON []map[string]interface{}
}

// ToTable convert JSON to Table.
func (j *JSON) ToTable() *Table {
	var keys []string
	for _, json := range j.JSON {
		for k := range json {
			keys = append(keys, k)
		}
	}
	header := sliceUnique(keys)
	sort.Strings(header)

	var records []Record
	for _, json := range j.JSON {
		r := Record{}
		for _, h := range header {
			if val, ok := json[h]; ok {
				switch v := val.(type) {
				case string:
					r = append(r, v)
				case float64:
					r = append(r, strconv.FormatFloat(v, 'f', -1, 64))
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

	return &Table{
		Name:    strings.TrimSuffix(j.Name, ".json"),
		Header:  header,
		Records: records,
	}
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
