package gameexcel

type Field struct {
	Name         string `json:"name"`
	ClientName   string `json:"client_name,omitempty"`
	DisplayName  string `json:"display_name,omitempty"`
	Type         string `json:"type"`
	SourceColumn int    `json:"source_column"`
}

type SourceLayout struct {
	DataStartRow int   `json:"data_start_row"`
	HeaderRows   []int `json:"header_rows"`
	MaxRows      int   `json:"max_rows"`
	MaxColumns   int   `json:"max_columns"`
}

type Table struct {
	Name             string       `json:"name"`
	SourcePath       string       `json:"source_path"`
	Sheet            string       `json:"sheet"`
	Schema           Schema       `json:"schema"`
	Fields           []Field      `json:"fields"`
	Rows             [][]string   `json:"rows"`
	SourceRowIndices []int        `json:"source_row_indices,omitempty"`
	Source           SourceLayout `json:"source"`
}

type Summary struct {
	Name       string  `json:"name"`
	SourcePath string  `json:"source_path"`
	Sheet      string  `json:"sheet"`
	Schema     Schema  `json:"schema"`
	Rows       int     `json:"rows"`
	Columns    int     `json:"columns"`
	Fields     []Field `json:"fields"`
}
