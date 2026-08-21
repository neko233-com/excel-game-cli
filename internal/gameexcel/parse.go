package gameexcel

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/xuri/excelize/v2"
)

func OpenTable(path string, cfg Config) (*Table, error) {
	f, err := excelize.OpenFile(path, excelize.Options{RawCellValue: false})
	if err != nil {
		return nil, fmt.Errorf("open workbook %s: %w", path, err)
	}
	defer f.Close()
	return readTable(f, path, cfg.normalized())
}

func readTable(f *excelize.File, path string, cfg Config) (*Table, error) {
	sheet, err := resolveSheet(f, cfg.Sheet)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	rows, err := f.GetRows(sheet)
	if err != nil {
		return nil, fmt.Errorf("read sheet %s: %w", sheet, err)
	}
	maxColumns := 0
	for _, row := range rows {
		if len(row) > maxColumns {
			maxColumns = len(row)
		}
	}

	schema, err := detectSchema(rows, Schema(cfg.Schema))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	var table *Table
	if schema == SchemaLuban {
		table, err = parseLuban(rows, path, sheet, maxColumns)
	} else {
		table, err = parseConfig233(rows, path, sheet, maxColumns, cfg.Header)
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return table, nil
}

func resolveSheet(f *excelize.File, requested string) (string, error) {
	if requested != "" {
		for _, name := range f.GetSheetList() {
			if name == requested {
				return name, nil
			}
		}
		return "", fmt.Errorf("worksheet not found: %s", requested)
	}
	for _, name := range f.GetSheetList() {
		return name, nil
	}
	return "", fmt.Errorf("workbook has no worksheets")
}

func detectSchema(rows [][]string, requested Schema) (Schema, error) {
	if requested != "" && requested != SchemaAuto && requested != SchemaConfig233 && requested != SchemaLuban {
		return "", fmt.Errorf("invalid schema %q", requested)
	}
	if requested == SchemaConfig233 || requested == SchemaLuban {
		return requested, nil
	}
	for _, row := range rows {
		if len(row) == 0 {
			continue
		}
		if strings.TrimSpace(row[0]) == "##var" {
			return SchemaLuban, nil
		}
	}
	return SchemaConfig233, nil
}

func parseConfig233(rows [][]string, path, sheet string, maxColumns int, h HeaderConfig) (*Table, error) {
	if h.ServerRow <= 0 || h.DataStartRow <= 0 {
		return nil, fmt.Errorf("invalid config233 header configuration")
	}
	server := h.ServerRow - 1
	dataStart := h.DataStartRow - 1
	if server >= len(rows) {
		return nil, fmt.Errorf("Server header row %d is missing", h.ServerRow)
	}
	if dataStart > len(rows) {
		dataStart = len(rows)
	}
	fields, err := fieldsFromRows(rows, server, h, h.SkipColumns)
	if err != nil {
		return nil, err
	}
	table := &Table{
		Name: strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)), SourcePath: path, Sheet: sheet,
		Schema: SchemaConfig233, Fields: fields,
		Source: SourceLayout{DataStartRow: dataStart, HeaderRows: []int{h.CommentRow - 1, h.DisplayRow - 1, h.ClientRow - 1, h.TypeRow - 1, h.ServerRow - 1}, MaxRows: len(rows), MaxColumns: maxColumns},
	}
	table.Rows, table.SourceRowIndices = dataRows(rows, dataStart, fields)
	return table, nil
}

func fieldsFromRows(rows [][]string, serverRow int, h HeaderConfig, skip int) ([]Field, error) {
	server := rowAt(rows, serverRow)
	client := rowAt(rows, h.ClientRow-1)
	display := rowAt(rows, h.DisplayRow-1)
	types := rowAt(rows, h.TypeRow-1)
	if skip < 0 {
		skip = 0
	}
	fields := make([]Field, 0, len(server)-skip)
	seen := make(map[string]struct{}, len(server))
	for column := skip; column < len(server); column++ {
		name := strings.TrimSpace(server[column])
		if name == "" {
			continue
		}
		if _, exists := seen[name]; exists {
			return nil, fmt.Errorf("duplicate field name %q", name)
		}
		seen[name] = struct{}{}
		field := Field{Name: name, Type: "string", SourceColumn: column}
		field.ClientName = strings.TrimSpace(cellAt(client, column))
		field.DisplayName = strings.TrimSpace(cellAt(display, column))
		if value := strings.TrimSpace(cellAt(types, column)); value != "" {
			field.Type = value
		}
		fields = append(fields, field)
	}
	if len(fields) == 0 {
		return nil, fmt.Errorf("no fields found in Server header row")
	}
	return fields, nil
}

func parseLuban(rows [][]string, path, sheet string, maxColumns int) (*Table, error) {
	varRow, typeRow, commentRow, lastHeader := -1, -1, -1, -1
	for index, row := range rows {
		marker := strings.TrimSpace(cellAt(row, 0))
		if strings.HasPrefix(marker, "##") {
			if index > lastHeader {
				lastHeader = index
			}
			switch marker {
			case "##var":
				if varRow == -1 {
					varRow = index
				}
			case "##type":
				if typeRow == -1 {
					typeRow = index
				}
			case "##comment":
				if commentRow == -1 {
					commentRow = index
				}
			}
		}
	}
	if varRow < 0 {
		return nil, fmt.Errorf("Luban ##var row is missing")
	}
	if lastHeader < varRow {
		lastHeader = varRow
	}
	varNames := rowAt(rows, varRow)
	types := rowAt(rows, typeRow)
	comments := rowAt(rows, commentRow)
	fields := make([]Field, 0, len(varNames)-1)
	seen := map[string]struct{}{}
	for column := 1; column < len(varNames); column++ {
		name := strings.TrimSpace(varNames[column])
		if name == "" {
			continue
		}
		if _, exists := seen[name]; exists {
			return nil, fmt.Errorf("duplicate field name %q", name)
		}
		seen[name] = struct{}{}
		field := Field{Name: name, Type: "string", SourceColumn: column}
		if value := strings.TrimSpace(cellAt(types, column)); value != "" {
			field.Type = value
		}
		field.DisplayName = strings.TrimSpace(cellAt(comments, column))
		field.ClientName = field.Name
		fields = append(fields, field)
	}
	if len(fields) == 0 {
		return nil, fmt.Errorf("no fields found in Luban ##var row")
	}
	table := &Table{
		Name: strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)), SourcePath: path, Sheet: sheet,
		Schema: SchemaLuban, Fields: fields,
		Source: SourceLayout{DataStartRow: lastHeader + 1, HeaderRows: []int{varRow, typeRow, commentRow}, MaxRows: len(rows), MaxColumns: maxColumns},
	}
	table.Rows, table.SourceRowIndices = dataRows(rows, lastHeader+1, fields)
	return table, nil
}

func dataRows(rows [][]string, start int, fields []Field) ([][]string, []int) {
	result := make([][]string, 0, max(0, len(rows)-start))
	indices := make([]int, 0, max(0, len(rows)-start))
	for rowIndex := start; rowIndex < len(rows); rowIndex++ {
		row := make([]string, len(fields))
		empty := true
		for index, field := range fields {
			row[index] = cellAt(rowAt(rows, rowIndex), field.SourceColumn)
			if row[index] != "" {
				empty = false
			}
		}
		if !empty {
			result = append(result, row)
			indices = append(indices, rowIndex)
		}
	}
	return result, indices
}

func rowAt(rows [][]string, index int) []string {
	if index < 0 || index >= len(rows) {
		return nil
	}
	return rows[index]
}

func cellAt(row []string, index int) string {
	if index < 0 || index >= len(row) {
		return ""
	}
	return row[index]
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
