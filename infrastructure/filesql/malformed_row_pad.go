package filesql

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// rejectLongDelimitedRows checks the one malformed-row case that filesql's
// Fill policy cannot distinguish: a long row. Pad may add missing trailing
// fields, but it must never discard fields that were present in the source.
func rejectLongDelimitedRows(paths []string) error {
	files, err := delimitedInputFiles(paths)
	if err != nil {
		return err
	}
	for _, path := range files {
		data, err := readDecompressed(path)
		if err != nil {
			return fmt.Errorf("read %s for --import-mode pad: %w", path, err)
		}
		reader := csv.NewReader(bytes.NewReader(data))
		reader.FieldsPerRecord = -1
		if strings.EqualFold(filepath.Ext(stripCompressionExt(path)), ".tsv") {
			reader.Comma = '\t'
		}

		header, err := reader.Read()
		if errors.Is(err, io.EOF) {
			continue
		}
		if err != nil {
			return fmt.Errorf("read header of %s for --import-mode pad: %w", path, err)
		}
		for row := 2; ; row++ {
			record, err := reader.Read()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				return fmt.Errorf("read row %d of %s for --import-mode pad: %w", row, path, err)
			}
			if len(record) > len(header) {
				return fmt.Errorf("--import-mode pad refuses to truncate row %d of %s: got %d fields, header has %d", row, path, len(record), len(header))
			}
		}
	}
	return nil
}

func delimitedInputFiles(paths []string) ([]string, error) {
	var files []string
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			if isDelimitedInput(path) {
				files = append(files, path)
			}
			continue
		}
		if err := filepath.WalkDir(path, func(current string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if !entry.IsDir() && isDelimitedInput(current) {
				files = append(files, current)
			}
			return nil
		}); err != nil {
			return nil, err
		}
	}
	return files, nil
}

func isDelimitedInput(path string) bool {
	ext := strings.ToLower(filepath.Ext(stripCompressionExt(path)))
	return ext == ".csv" || ext == ".tsv"
}
