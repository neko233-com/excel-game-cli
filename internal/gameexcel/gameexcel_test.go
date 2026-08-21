package gameexcel

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestConfig233ParseKeepsCellValuesAsStrings(t *testing.T) {
	path := writeConfig233Fixture(t)
	table, err := OpenTable(path, DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	if table.Schema != SchemaConfig233 {
		t.Fatalf("schema = %s", table.Schema)
	}
	if got, want := len(table.Fields), 3; got != want {
		t.Fatalf("fields = %d, want %d", got, want)
	}
	if got, want := len(table.Rows), 2; got != want {
		t.Fatalf("rows = %d, want %d", got, want)
	}
	if got, want := table.Rows[0][0], "1001"; got != want {
		t.Fatalf("id = %q, want %q", got, want)
	}
	if got, want := table.Rows[0][2], "TRUE"; got != want {
		t.Fatalf("bool = %q, want %q", got, want)
	}
	if got, want := table.SourceRowIndices[1], 6; got != want {
		t.Fatalf("second source row = %d, want %d", got, want)
	}
}

func TestJSONDefaultIsStringAndTypedIsOptIn(t *testing.T) {
	table := &Table{
		Fields: []Field{{Name: "id", Type: "long"}, {Name: "enabled", Type: "bool"}},
		Rows:   [][]string{{"1001", "true"}},
	}
	var raw bytes.Buffer
	if err := WriteTable(&raw, table, DefaultConfig(), FormatJSON); err != nil {
		t.Fatal(err)
	}
	var values []map[string]any
	if err := json.Unmarshal(raw.Bytes(), &values); err != nil {
		t.Fatal(err)
	}
	if _, ok := values[0]["id"].(string); !ok {
		t.Fatalf("default id type = %T, want string", values[0]["id"])
	}
	if _, ok := values[0]["enabled"].(string); !ok {
		t.Fatalf("default bool type = %T, want string", values[0]["enabled"])
	}

	typed := DefaultConfig()
	typed.ValueMode = string(ValueModeTyped)
	raw.Reset()
	if err := WriteTable(&raw, table, typed, FormatJSON); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw.Bytes(), &values); err != nil {
		t.Fatal(err)
	}
	if _, ok := values[0]["id"].(float64); !ok {
		t.Fatalf("typed id type = %T, want number", values[0]["id"])
	}
	if _, ok := values[0]["enabled"].(bool); !ok {
		t.Fatalf("typed bool type = %T, want bool", values[0]["enabled"])
	}
}

func TestConfig233TSVHasNameAndTypeRows(t *testing.T) {
	table := &Table{Fields: []Field{{Name: "id", Type: "long"}}, Rows: [][]string{{"1001"}}}
	var raw bytes.Buffer
	if err := WriteTable(&raw, table, DefaultConfig(), FormatConfig233); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(raw.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("line count = %d, want 3: %q", len(lines), raw.String())
	}
	if lines[0] != "id" || lines[1] != "long" || lines[2] != "1001" {
		t.Fatalf("unexpected config233 TSV: %q", raw.String())
	}
}

func TestFormatWorkbookPreservesFillAndWritesTextCells(t *testing.T) {
	input := writeConfig233Fixture(t)
	output := filepath.Join(t.TempDir(), "formatted.xlsx")
	cfg := DefaultConfig()
	cfg.Formatting.Normalize = true
	cfg.Formatting.AutoFit = true
	if err := FormatWorkbook(input, output, cfg); err != nil {
		t.Fatal(err)
	}
	f, err := excelize.OpenFile(output)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	value, err := f.GetCellValue("Sheet1", "B6")
	if err != nil {
		t.Fatal(err)
	}
	if value != "1001" {
		t.Fatalf("formatted value = %q", value)
	}
	styleID, err := f.GetCellStyle("Sheet1", "B6")
	if err != nil {
		t.Fatal(err)
	}
	style, err := f.GetStyle(styleID)
	if err != nil {
		t.Fatal(err)
	}
	if len(style.Fill.Color) == 0 || style.Fill.Color[0] != "FFFF00" {
		t.Fatalf("fill was not preserved: %#v", style.Fill)
	}
	if style.Font == nil || style.Font.Family != "Microsoft YaHei" || style.Font.Size != 11 {
		t.Fatalf("font not normalized: %#v", style.Font)
	}
}

func TestConvertConfig233ToLubanUsesCorrectHeaderRows(t *testing.T) {
	input := writeConfig233Fixture(t)
	output := filepath.Join(t.TempDir(), "luban.xlsx")
	if err := ConvertToLubanWorkbook(input, output, DefaultConfig()); err != nil {
		t.Fatal(err)
	}
	f, err := excelize.OpenFile(output)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	for cell, want := range map[string]string{"A1": "##var", "B1": "id", "A2": "##type", "B2": "long", "A3": "##comment", "B3": "物品id", "B4": "1001"} {
		got, err := f.GetCellValue("Sheet1", cell)
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("%s = %q, want %q", cell, got, want)
		}
	}
}

func writeConfig233Fixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "ItemConfig.xlsx")
	f := excelize.NewFile()
	defer f.Close()
	styleID, err := f.NewStyle(&excelize.Style{
		Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"FFFF00"}},
		Font: &excelize.Font{Family: "Arial", Size: 9},
	})
	if err != nil {
		t.Fatal(err)
	}
	values := map[string]string{
		"A1": "", "B1": "注释", "C1": "名称注释", "D1": "开关注释",
		"A2": "", "B2": "物品id", "C2": "物品名称", "D2": "是否启用",
		"A3": "Client", "B3": "id", "C3": "itemName", "D3": "enabled",
		"A4": "type", "B4": "long", "C4": "string", "D4": "bool",
		"A5": "Server", "B5": "id", "C5": "itemName", "D5": "enabled",
		"A6": "", "B6": "1001", "C6": "金币", "D6": "TRUE",
		"A7": "", "B7": "1002", "C7": "钻石", "D7": "FALSE",
	}
	for cell, value := range values {
		if err := f.SetCellStr("Sheet1", cell, value); err != nil {
			t.Fatal(err)
		}
	}
	if err := f.SetCellStyle("Sheet1", "B6", "B7", styleID); err != nil {
		t.Fatal(err)
	}
	if err := f.SaveAs(path); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestDefaultConfigCanBeMarshaled(t *testing.T) {
	data, err := json.Marshal(DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"value_mode":"string"`) {
		t.Fatalf("default config lost string mode: %s", data)
	}
	if _, err := os.Stat(filepath.Join(t.TempDir(), "missing")); !os.IsNotExist(err) {
		t.Fatal("test setup unexpectedly exists")
	}
}

func BenchmarkWriteJSON(b *testing.B) {
	fields := []Field{{Name: "id", Type: "long"}, {Name: "name", Type: "string"}, {Name: "enabled", Type: "bool"}}
	rows := make([][]string, 1000)
	for i := range rows {
		rows[i] = []string{"1001", "item", "true"}
	}
	table := &Table{Fields: fields, Rows: rows}
	cfg := DefaultConfig()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var output bytes.Buffer
		if err := WriteTable(&output, table, cfg, FormatJSON); err != nil {
			b.Fatal(err)
		}
	}
}
