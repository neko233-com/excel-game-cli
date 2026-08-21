package gameexcel

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
)

func WriteTable(w io.Writer, table *Table, cfg Config, format Format) error {
	cfg = cfg.normalized()
	switch format {
	case FormatConfig233:
		return writeConfig233(w, table)
	case FormatTSV:
		return writeDelimited(w, table, '\t', false, cfg)
	case FormatCSV:
		return writeDelimited(w, table, ',', false, cfg)
	case FormatJSON:
		return writeJSON(w, table, cfg)
	default:
		return fmt.Errorf("text writer does not support format %q", format)
	}
}

func writeConfig233(w io.Writer, table *Table) error {
	writer := csv.NewWriter(w)
	writer.Comma = '\t'
	writer.UseCRLF = false
	if err := writer.Write(fieldNames(table.Fields)); err != nil {
		return err
	}
	if err := writer.Write(fieldTypes(table.Fields)); err != nil {
		return err
	}
	for _, row := range table.Rows {
		if err := writer.Write(row); err != nil {
			return err
		}
	}
	writer.Flush()
	return writer.Error()
}

func writeDelimited(w io.Writer, table *Table, delimiter rune, config233Header bool, cfg Config) error {
	_ = config233Header
	writer := csv.NewWriter(w)
	writer.Comma = delimiter
	writer.UseCRLF = false
	if err := writer.Write(fieldNames(table.Fields)); err != nil {
		return err
	}
	for _, row := range table.Rows {
		values := make([]string, len(row))
		for index, value := range row {
			values[index] = stringValue(value, table.Fields[index].Type, ValueMode(cfg.ValueMode))
		}
		if err := writer.Write(values); err != nil {
			return err
		}
	}
	writer.Flush()
	return writer.Error()
}

func writeJSON(w io.Writer, table *Table, cfg Config) error {
	buffer := bufio.NewWriterSize(w, 64*1024)
	defer buffer.Flush()
	if cfg.JSONShape == "map" {
		return writeJSONMap(buffer, table, cfg)
	}
	if _, err := io.WriteString(buffer, "[\n"); err != nil {
		return err
	}
	for rowIndex, row := range table.Rows {
		if rowIndex > 0 {
			if _, err := io.WriteString(buffer, ",\n"); err != nil {
				return err
			}
		}
		if err := writeJSONObject(buffer, table.Fields, row, cfg); err != nil {
			return err
		}
	}
	_, err := io.WriteString(buffer, "\n]\n")
	return err
}

func writeJSONMap(w io.Writer, table *Table, cfg Config) error {
	keyIndex := 0
	for index, field := range table.Fields {
		if strings.EqualFold(field.Name, "id") || strings.EqualFold(field.Name, "key") {
			keyIndex = index
			break
		}
	}
	if _, err := io.WriteString(w, "{\n"); err != nil {
		return err
	}
	for rowIndex, row := range table.Rows {
		if rowIndex > 0 {
			if _, err := io.WriteString(w, ",\n"); err != nil {
				return err
			}
		}
		key, _ := json.Marshal(rowValue(row, keyIndex))
		if _, err := w.Write(key); err != nil {
			return err
		}
		if _, err := io.WriteString(w, ": "); err != nil {
			return err
		}
		if err := writeJSONObject(w, table.Fields, row, cfg); err != nil {
			return err
		}
	}
	_, err := io.WriteString(w, "\n}\n")
	return err
}

func writeJSONObject(w io.Writer, fields []Field, row []string, cfg Config) error {
	if _, err := io.WriteString(w, "{"); err != nil {
		return err
	}
	for index, field := range fields {
		if index > 0 {
			if _, err := io.WriteString(w, ", "); err != nil {
				return err
			}
		}
		name, _ := json.Marshal(field.Name)
		if _, err := w.Write(name); err != nil {
			return err
		}
		if _, err := io.WriteString(w, ": "); err != nil {
			return err
		}
		encoded, err := json.Marshal(jsonValue(rowValue(row, index), field.Type, cfg))
		if err != nil {
			return fmt.Errorf("encode field %s: %w", field.Name, err)
		}
		if _, err := w.Write(encoded); err != nil {
			return err
		}
	}
	_, err := io.WriteString(w, "}")
	return err
}

func fieldNames(fields []Field) []string {
	result := make([]string, len(fields))
	for index, field := range fields {
		result[index] = field.Name
	}
	return result
}

func fieldTypes(fields []Field) []string {
	result := make([]string, len(fields))
	for index, field := range fields {
		result[index] = field.Type
	}
	return result
}

func rowValue(row []string, index int) string {
	if index < 0 || index >= len(row) {
		return ""
	}
	return row[index]
}

func stringValue(value, typ string, mode ValueMode) string {
	if mode == ValueModeTyped {
		if typed, ok := typedValue(value, typ); ok {
			switch v := typed.(type) {
			case bool:
				return strconv.FormatBool(v)
			case int64:
				return strconv.FormatInt(v, 10)
			case uint64:
				return strconv.FormatUint(v, 10)
			case float64:
				return strconv.FormatFloat(v, 'g', -1, 64)
			}
		}
	}
	return value
}

func jsonValue(value, typ string, cfg Config) any {
	if value == "" && cfg.ValueMode != string(ValueModeTyped) {
		return ""
	}
	if cfg.ValueMode != string(ValueModeTyped) {
		return value
	}
	if typed, ok := typedValue(value, typ); ok {
		return typed
	}
	return value
}

func typedValue(value, typ string) (any, bool) {
	t := strings.ToLower(strings.TrimSpace(typ))
	if strings.HasSuffix(t, "?") && strings.EqualFold(strings.TrimSpace(value), "null") {
		return nil, true
	}
	switch t {
	case "bool", "boolean":
		v, err := strconv.ParseBool(strings.ToLower(strings.TrimSpace(value)))
		if err != nil {
			switch strings.ToLower(strings.TrimSpace(value)) {
			case "1", "yes", "y":
				return true, true
			case "0", "no", "n":
				return false, true
			}
			return nil, false
		}
		return v, true
	case "int", "int8", "int16", "int32", "long", "int64":
		v, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
		return v, err == nil
	case "uint", "uint8", "uint16", "uint32", "ulong", "uint64":
		v, err := strconv.ParseUint(strings.TrimSpace(value), 10, 64)
		return v, err == nil
	case "float", "float32", "double", "float64":
		v, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
		if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
			return nil, false
		}
		return v, true
	case "json":
		if json.Valid([]byte(value)) {
			var v any
			if err := json.Unmarshal([]byte(value), &v); err == nil {
				return v, true
			}
		}
	}
	return nil, false
}
