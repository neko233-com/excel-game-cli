package gameexcel

type Format string

const (
	FormatConfig233 Format = "config233"
	FormatLuban     Format = "luban"
	FormatJSON      Format = "json"
	FormatTSV       Format = "tsv"
	FormatCSV       Format = "csv"
)

type Schema string

const (
	SchemaAuto      Schema = "auto"
	SchemaConfig233 Schema = "config233"
	SchemaLuban     Schema = "luban"
)

type ValueMode string

const (
	ValueModeString ValueMode = "string"
	ValueModeTyped  ValueMode = "typed"
)

type HeaderConfig struct {
	CommentRow   int `json:"comment_row"`
	DisplayRow   int `json:"display_row"`
	ClientRow    int `json:"client_row"`
	TypeRow      int `json:"type_row"`
	ServerRow    int `json:"server_row"`
	DataStartRow int `json:"data_start_row"`
	SkipColumns  int `json:"skip_columns"`
}

type FormattingConfig struct {
	Normalize      bool    `json:"normalize"`
	PreserveStyles bool    `json:"preserve_styles"`
	AutoFit        bool    `json:"auto_fit"`
	FontFamily     string  `json:"font_family"`
	FontSize       float64 `json:"font_size"`
	MinColumnWidth float64 `json:"min_column_width"`
	MaxColumnWidth float64 `json:"max_column_width"`
	BaseRowHeight  float64 `json:"base_row_height"`
	MaxRowHeight   float64 `json:"max_row_height"`
}

type Config struct {
	Format     string           `json:"format"`
	Schema     string           `json:"schema"`
	Sheet      string           `json:"sheet"`
	OutputDir  string           `json:"output_dir"`
	Recursive  bool             `json:"recursive"`
	Workers    int              `json:"workers"`
	ValueMode  string           `json:"value_mode"`
	JSONShape  string           `json:"json_shape"`
	Header     HeaderConfig     `json:"header"`
	Formatting FormattingConfig `json:"formatting"`
}

func DefaultConfig() Config {
	return Config{
		Format: string(FormatConfig233),
		Schema: string(SchemaAuto),
		// Empty means a system temporary directory at runtime. This keeps a
		// normal game repository clean unless the caller explicitly uses --out.
		OutputDir: "",
		Recursive: true,
		Workers:   0,
		ValueMode: string(ValueModeString),
		JSONShape: "array",
		Header: HeaderConfig{
			CommentRow:   1,
			DisplayRow:   2,
			ClientRow:    3,
			TypeRow:      4,
			ServerRow:    5,
			DataStartRow: 6,
			SkipColumns:  1,
		},
		Formatting: FormattingConfig{
			Normalize:      true,
			PreserveStyles: true,
			AutoFit:        true,
			FontFamily:     "Microsoft YaHei",
			FontSize:       11,
			MinColumnWidth: 10,
			MaxColumnWidth: 60,
			BaseRowHeight:  18,
			MaxRowHeight:   120,
		},
	}
}

func (c Config) normalized() Config {
	d := DefaultConfig()
	if c.Format == "" {
		c.Format = d.Format
	}
	if c.Schema == "" {
		c.Schema = d.Schema
	}
	if c.OutputDir == "" {
		c.OutputDir = d.OutputDir
	}
	if c.Workers <= 0 {
		c.Workers = d.Workers
	}
	if c.ValueMode == "" {
		c.ValueMode = d.ValueMode
	}
	if c.JSONShape == "" {
		c.JSONShape = d.JSONShape
	}
	if c.Header.CommentRow <= 0 {
		c.Header.CommentRow = d.Header.CommentRow
	}
	if c.Header.DisplayRow <= 0 {
		c.Header.DisplayRow = d.Header.DisplayRow
	}
	if c.Header.ClientRow <= 0 {
		c.Header.ClientRow = d.Header.ClientRow
	}
	if c.Header.TypeRow <= 0 {
		c.Header.TypeRow = d.Header.TypeRow
	}
	if c.Header.ServerRow <= 0 {
		c.Header.ServerRow = d.Header.ServerRow
	}
	if c.Header.DataStartRow <= 0 {
		c.Header.DataStartRow = d.Header.DataStartRow
	}
	if c.Header.SkipColumns < 0 {
		c.Header.SkipColumns = d.Header.SkipColumns
	}
	if c.Formatting.FontFamily == "" {
		c.Formatting.FontFamily = d.Formatting.FontFamily
	}
	if c.Formatting.FontSize <= 0 {
		c.Formatting.FontSize = d.Formatting.FontSize
	}
	if c.Formatting.MinColumnWidth <= 0 {
		c.Formatting.MinColumnWidth = d.Formatting.MinColumnWidth
	}
	if c.Formatting.MaxColumnWidth <= 0 {
		c.Formatting.MaxColumnWidth = d.Formatting.MaxColumnWidth
	}
	if c.Formatting.BaseRowHeight <= 0 {
		c.Formatting.BaseRowHeight = d.Formatting.BaseRowHeight
	}
	if c.Formatting.MaxRowHeight <= 0 {
		c.Formatting.MaxRowHeight = d.Formatting.MaxRowHeight
	}
	return c
}
