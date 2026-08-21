package gameexcel

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/xuri/excelize/v2"
)

func rewriteAsLuban(f *excelize.File, table *Table) error {
	sheet := table.Sheet
	rows, err := f.GetRows(sheet)
	if err != nil {
		return fmt.Errorf("read workbook before Luban conversion: %w", err)
	}
	maxRows, maxColumns := len(rows), table.Source.MaxColumns
	if maxRows < table.Source.DataStartRow+len(table.Rows) {
		maxRows = table.Source.DataStartRow + len(table.Rows)
	}
	if maxColumns < len(table.Fields)+1 {
		maxColumns = len(table.Fields) + 1
	}

	// Capture source style IDs before moving data upward. Only the value moves;
	// the source style is copied to the target cell, so fills/borders survive.
	type styleKey struct{ row, column int }
	styles := make(map[styleKey]int)
	for row := 0; row < len(rows); row++ {
		for column := 0; column < maxColumns; column++ {
			cell, cellErr := excelize.CoordinatesToCellName(column+1, row+1)
			if cellErr != nil {
				return cellErr
			}
			styleID, styleErr := f.GetCellStyle(sheet, cell)
			if styleErr != nil {
				return styleErr
			}
			styles[styleKey{row: row, column: column}] = styleID
		}
	}

	// Clear the old table values while leaving the workbook's cell styles in place.
	// Conversion is an explicit schema change, so formulas in the source table are
	// intentionally replaced by their displayed string values.
	for row := 0; row < maxRows; row++ {
		for column := 0; column < maxColumns; column++ {
			cell, cellErr := excelize.CoordinatesToCellName(column+1, row+1)
			if cellErr != nil {
				return cellErr
			}
			if err := f.SetCellStr(sheet, cell, ""); err != nil {
				return err
			}
		}
	}

	write := func(row, column int, value string, sourceRow, sourceColumn int) error {
		cell, err := excelize.CoordinatesToCellName(column+1, row+1)
		if err != nil {
			return err
		}
		if err := f.SetCellStr(sheet, cell, value); err != nil {
			return err
		}
		if styleID, ok := styles[styleKey{row: sourceRow, column: sourceColumn}]; ok {
			if err := f.SetCellStyle(sheet, cell, cell, styleID); err != nil {
				return err
			}
		}
		return nil
	}

	commentRow, typeRow, varRow := -1, -1, -1
	if table.Schema == SchemaLuban {
		varRow = findHeaderRow(table, 0)
		typeRow = findHeaderRow(table, 1)
		commentRow = findHeaderRow(table, 2)
	} else {
		commentRow = findHeaderRow(table, 0)
		typeRow = findHeaderRow(table, 3)
		varRow = findHeaderRow(table, 2)
	}
	if commentRow < 0 {
		commentRow = table.Source.DataStartRow - 1
	}
	if typeRow < 0 {
		typeRow = table.Source.DataStartRow - 2
	}
	if varRow < 0 {
		varRow = table.Source.DataStartRow - 3
	}
	if err := write(0, 0, "##var", varRow, 0); err != nil {
		return err
	}
	if err := write(1, 0, "##type", typeRow, 0); err != nil {
		return err
	}
	if err := write(2, 0, "##comment", commentRow, 0); err != nil {
		return err
	}
	for index, field := range table.Fields {
		targetColumn := index + 1
		if err := write(0, targetColumn, field.Name, varRow, field.SourceColumn); err != nil {
			return err
		}
		if err := write(1, targetColumn, field.Type, typeRow, field.SourceColumn); err != nil {
			return err
		}
		if err := write(2, targetColumn, field.DisplayName, commentRow, field.SourceColumn); err != nil {
			return err
		}
	}
	for rowIndex, row := range table.Rows {
		targetRow := rowIndex + 3
		sourceRow := table.Source.DataStartRow + rowIndex
		if rowIndex < len(table.SourceRowIndices) {
			sourceRow = table.SourceRowIndices[rowIndex]
		}
		for column, field := range table.Fields {
			if err := write(targetRow, column+1, rowValue(row, column), sourceRow, field.SourceColumn); err != nil {
				return err
			}
		}
	}
	return nil
}

func findHeaderRow(table *Table, index int) int {
	if index < 0 || index >= len(table.Source.HeaderRows) {
		return -1
	}
	return table.Source.HeaderRows[index]
}

func FormatWorkbook(input, output string, cfg Config) error {
	cfg = cfg.normalized()
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	f, err := excelize.OpenFile(input, excelize.Options{RawCellValue: false})
	if err != nil {
		return fmt.Errorf("open workbook %s: %w", input, err)
	}
	defer f.Close()
	if err := normalizeFile(f, cfg); err != nil {
		return err
	}
	if err := f.SaveAs(output); err != nil {
		return fmt.Errorf("save formatted workbook %s: %w", output, err)
	}
	return nil
}

func normalizeFile(f *excelize.File, cfg Config) error {
	sheets := f.GetSheetList()
	if cfg.Sheet != "" {
		sheets = []string{cfg.Sheet}
	}
	for _, sheet := range sheets {
		rows, err := f.GetRows(sheet)
		if err != nil {
			return fmt.Errorf("read sheet %s: %w", sheet, err)
		}
		maxColumns := 0
		for _, row := range rows {
			if len(row) > maxColumns {
				maxColumns = len(row)
			}
		}
		if maxColumns == 0 {
			continue
		}
		if cfg.Formatting.Normalize {
			if err := normalizeFonts(f, sheet, len(rows), maxColumns, cfg.Formatting); err != nil {
				return err
			}
		}
		if cfg.Formatting.AutoFit {
			if err := autoFit(f, sheet, rows, cfg.Formatting); err != nil {
				return err
			}
		}
		if cfg.ValueMode == string(ValueModeString) {
			if err := stringifyCells(f, sheet, rows); err != nil {
				return err
			}
		}
	}
	return nil
}

func stringifyCells(f *excelize.File, sheet string, rows [][]string) error {
	for rowIndex, row := range rows {
		for column, value := range row {
			cell, err := excelize.CoordinatesToCellName(column+1, rowIndex+1)
			if err != nil {
				return err
			}
			formula, err := f.GetCellFormula(sheet, cell)
			if err != nil {
				return err
			}
			if formula != "" {
				continue
			}
			if value == "" {
				continue
			}
			if err := f.SetCellStr(sheet, cell, value); err != nil {
				return err
			}
		}
	}
	return nil
}

func normalizeFonts(f *excelize.File, sheet string, rows, columns int, cfg FormattingConfig) error {
	cache := make(map[int]int)
	for row := 1; row <= rows; row++ {
		for column := 1; column <= columns; column++ {
			cell, err := excelize.CoordinatesToCellName(column, row)
			if err != nil {
				return err
			}
			styleID, err := f.GetCellStyle(sheet, cell)
			if err != nil {
				return err
			}
			newStyleID, found := cache[styleID]
			if !found {
				style, err := f.GetStyle(styleID)
				if err != nil {
					return err
				}
				if style.Font == nil {
					style.Font = &excelize.Font{}
				}
				if cfg.FontFamily != "" {
					style.Font.Family = cfg.FontFamily
				}
				if cfg.FontSize > 0 {
					style.Font.Size = cfg.FontSize
				}
				newStyleID, err = f.NewStyle(style)
				if err != nil {
					return err
				}
				cache[styleID] = newStyleID
			}
			if err := f.SetCellStyle(sheet, cell, cell, newStyleID); err != nil {
				return err
			}
		}
	}
	return nil
}

func autoFit(f *excelize.File, sheet string, rows [][]string, cfg FormattingConfig) error {
	widths := make([]float64, 0)
	for _, row := range rows {
		if len(row) > len(widths) {
			next := make([]float64, len(row))
			copy(next, widths)
			widths = next
		}
		for column, value := range row {
			width := maxLineWidth(value)
			if width > widths[column] {
				widths[column] = width
			}
		}
	}
	for column, width := range widths {
		width += 2
		if width < cfg.MinColumnWidth {
			width = cfg.MinColumnWidth
		}
		if width > cfg.MaxColumnWidth {
			width = cfg.MaxColumnWidth
		}
		name, err := excelize.ColumnNumberToName(column + 1)
		if err != nil {
			return err
		}
		if err := f.SetColWidth(sheet, name, name, width); err != nil {
			return err
		}
	}
	for rowIndex, row := range rows {
		lines := 1
		for column, value := range row {
			if column >= len(widths) {
				continue
			}
			available := widths[column]
			if available < cfg.MinColumnWidth {
				available = cfg.MinColumnWidth
			}
			lineCount := wrappedLineCount(value, available)
			if lineCount > lines {
				lines = lineCount
			}
		}
		height := cfg.BaseRowHeight * float64(lines)
		if height > cfg.MaxRowHeight {
			height = cfg.MaxRowHeight
		}
		if err := f.SetRowHeight(sheet, rowIndex+1, height); err != nil {
			return err
		}
	}
	return nil
}

func maxLineWidth(value string) float64 {
	maxWidth := 0.0
	for _, line := range strings.Split(value, "\n") {
		width := 0.0
		for _, r := range line {
			if unicode.Is(unicode.Han, r) || unicode.Is(unicode.Hangul, r) || unicode.Is(unicode.Hiragana, r) || unicode.Is(unicode.Katakana, r) {
				width += 2
			} else {
				width++
			}
		}
		if width > maxWidth {
			maxWidth = width
		}
	}
	return maxWidth
}

func wrappedLineCount(value string, width float64) int {
	if width <= 0 {
		return 1
	}
	lines := 0
	for _, line := range strings.Split(value, "\n") {
		count := int(math.Ceil(maxLineWidth(line) / width))
		if count < 1 {
			count = 1
		}
		lines += count
	}
	if lines < 1 {
		return 1
	}
	return lines
}
