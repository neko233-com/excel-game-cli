package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/neko233-com/excel-game-cli/internal/gameexcel"
)

const version = "0.1.0"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		printUsage()
		return nil
	}
	if args[0] == "version" || args[0] == "--version" {
		fmt.Printf("excel-game-cli %s (%s/%s)\n", version, runtime.GOOS, runtime.GOARCH)
		return nil
	}

	switch args[0] {
	case "convert":
		return runConvert(args[1:])
	case "inspect":
		return runInspect(args[1:])
	case "format":
		return runFormat(args[1:])
	case "init":
		return runInit(args[1:])
	default:
		return fmt.Errorf("unknown command %q; run 'excel-game-cli help'", args[0])
	}
}

type commonFlags struct {
	configPath string
	outPath    string
	format     string
	schema     string
	sheet      string
	valueMode  string
	workers    int
	recursive  bool
	jsonShape  string
	normalize  bool
	fontFamily string
	fontSize   float64
	autoFit    bool
}

func bindCommonFlags(fs *flag.FlagSet) *commonFlags {
	f := &commonFlags{}
	fs.StringVar(&f.configPath, "config", "", "JSON configuration file (default: excel-game.config.json when present)")
	fs.StringVar(&f.outPath, "out", "", "output file or directory")
	fs.StringVar(&f.format, "format", "", "config233|luban|json|tsv|csv")
	fs.StringVar(&f.schema, "schema", "", "auto|config233|luban")
	fs.StringVar(&f.sheet, "sheet", "", "worksheet name (default: first worksheet)")
	fs.StringVar(&f.valueMode, "value-mode", "", "string|typed (default: string)")
	fs.IntVar(&f.workers, "workers", 0, "parallel workbook workers (default: CPU count)")
	fs.BoolVar(&f.recursive, "recursive", false, "scan input directories recursively")
	fs.StringVar(&f.jsonShape, "json-shape", "", "array|map (default: array)")
	fs.BoolVar(&f.normalize, "normalize", false, "unify font and auto-fit when writing an xlsx workbook")
	fs.StringVar(&f.fontFamily, "font-family", "", "font family for workbook normalization")
	fs.Float64Var(&f.fontSize, "font-size", -1, "font size for workbook normalization")
	fs.BoolVar(&f.autoFit, "auto-fit", false, "auto-fit workbook column widths and row heights")
	return f
}

func runConvert(args []string) error {
	fs := flag.NewFlagSet("convert", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	common := bindCommonFlags(fs)
	if err := fs.Parse(reorderArgs(args)); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("convert requires exactly one input file or directory")
	}

	cfg, err := loadConfig(common.configPath)
	if err != nil {
		return err
	}
	applyCommonFlags(&cfg, common, fs)
	if cfg.Format == "" {
		cfg.Format = string(gameexcel.FormatConfig233)
	}
	if cfg.OutputDir == "" {
		cfg.OutputDir = "generated"
	}
	if cfg.ValueMode == "" {
		cfg.ValueMode = string(gameexcel.ValueModeString)
	}

	input := fs.Arg(0)
	if cfg.Format == string(gameexcel.FormatLuban) {
		return gameexcel.ConvertDirectory(input, cfg)
	}
	return gameexcel.ConvertDirectory(input, cfg)
}

func runInspect(args []string) error {
	fs := flag.NewFlagSet("inspect", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	common := bindCommonFlags(fs)
	pretty := fs.Bool("pretty", true, "pretty-print JSON output")
	if err := fs.Parse(reorderArgs(args)); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("inspect requires exactly one input workbook")
	}
	cfg, err := loadConfig(common.configPath)
	if err != nil {
		return err
	}
	applyCommonFlags(&cfg, common, fs)
	summary, err := gameexcel.Inspect(fs.Arg(0), cfg)
	if err != nil {
		return err
	}
	var data []byte
	if *pretty {
		data, err = json.MarshalIndent(summary, "", "  ")
	} else {
		data, err = json.Marshal(summary)
	}
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

func runFormat(args []string) error {
	fs := flag.NewFlagSet("format", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	common := bindCommonFlags(fs)
	inPlace := fs.Bool("in-place", false, "replace the input workbook; use only when explicitly intended")
	if err := fs.Parse(reorderArgs(args)); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("format requires exactly one input workbook")
	}
	cfg, err := loadConfig(common.configPath)
	if err != nil {
		return err
	}
	applyCommonFlags(&cfg, common, fs)
	if common.fontFamily == "" && cfg.Formatting.FontFamily == "" {
		cfg.Formatting.FontFamily = "Microsoft YaHei"
	}
	if common.fontSize < 0 && cfg.Formatting.FontSize <= 0 {
		cfg.Formatting.FontSize = 11
	}
	if !flagWasSet(fs, "auto-fit") && !cfg.Formatting.AutoFit {
		cfg.Formatting.AutoFit = true
	}

	input := fs.Arg(0)
	output := common.outPath
	if *inPlace {
		output = input
	} else if output == "" {
		return errors.New("format requires --out or explicit --in-place")
	}
	return gameexcel.FormatWorkbook(input, output, cfg)
}

func runInit(args []string) error {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	force := fs.Bool("force", false, "overwrite an existing config file")
	if err := fs.Parse(reorderArgs(args)); err != nil {
		return err
	}
	dir := "."
	if fs.NArg() == 1 {
		dir = fs.Arg(0)
	} else if fs.NArg() > 1 {
		return errors.New("init accepts at most one directory")
	}
	return writeDefaultConfig(dir, *force)
}

func loadConfig(path string) (gameexcel.Config, error) {
	cfg := gameexcel.DefaultConfig()
	if path == "" {
		candidate := filepath.Join(".", "excel-game.config.json")
		if _, err := os.Stat(candidate); err != nil {
			if os.IsNotExist(err) {
				return cfg, nil
			}
			return cfg, err
		}
		path = candidate
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, fmt.Errorf("read config %s: %w", path, err)
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parse config %s: %w", path, err)
	}
	return cfg, nil
}

func applyCommonFlags(cfg *gameexcel.Config, common *commonFlags, fs *flag.FlagSet) {
	if flagWasSet(fs, "out") {
		cfg.OutputDir = common.outPath
	}
	if flagWasSet(fs, "format") {
		cfg.Format = common.format
	}
	if flagWasSet(fs, "schema") {
		cfg.Schema = common.schema
	}
	if flagWasSet(fs, "sheet") {
		cfg.Sheet = common.sheet
	}
	if flagWasSet(fs, "value-mode") {
		cfg.ValueMode = common.valueMode
	}
	if flagWasSet(fs, "workers") {
		cfg.Workers = common.workers
	}
	if flagWasSet(fs, "recursive") {
		cfg.Recursive = common.recursive
	}
	if flagWasSet(fs, "json-shape") {
		cfg.JSONShape = common.jsonShape
	}
	if flagWasSet(fs, "normalize") {
		cfg.Formatting.Normalize = common.normalize
	}
	if flagWasSet(fs, "font-family") {
		cfg.Formatting.FontFamily = common.fontFamily
	}
	if flagWasSet(fs, "font-size") {
		cfg.Formatting.FontSize = common.fontSize
	}
	if flagWasSet(fs, "auto-fit") {
		cfg.Formatting.AutoFit = common.autoFit
	}
}

func flagWasSet(fs *flag.FlagSet, name string) bool {
	wasSet := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == name {
			wasSet = true
		}
	})
	return wasSet
}

// The standard library flag package stops parsing at the first positional
// argument. Keep the CLI friendly for both `convert file --format json` and
// `convert --format json file` without pulling in a large flag dependency.
func reorderArgs(args []string) []string {
	valueFlags := map[string]bool{
		"config": true, "out": true, "format": true, "schema": true,
		"sheet": true, "value-mode": true, "workers": true,
		"json-shape": true, "font-family": true, "font-size": true,
	}
	flags := make([]string, 0, len(args))
	positional := make([]string, 0, 1)
	for index := 0; index < len(args); index++ {
		arg := args[index]
		if arg == "--" {
			positional = append(positional, args[index+1:]...)
			break
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			positional = append(positional, arg)
			continue
		}
		flags = append(flags, arg)
		name := strings.TrimLeft(arg, "-")
		if cut := strings.IndexByte(name, '='); cut >= 0 {
			name = name[:cut]
		}
		if !strings.Contains(arg, "=") && valueFlags[name] && index+1 < len(args) {
			flags = append(flags, args[index+1])
			index++
		}
	}
	return append(flags, positional...)
}

func writeDefaultConfig(dir string, force bool) error {
	path := filepath.Join(dir, "excel-game.config.json")
	if _, err := os.Stat(path); err == nil && !force {
		return fmt.Errorf("config already exists: %s (use --force to overwrite)", path)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(gameexcel.DefaultConfig(), "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return err
	}
	fmt.Println(path)
	return nil
}

func printUsage() {
	const usage = `excel-game-cli - high-performance game Excel converter

Usage:
  excel-game-cli convert <file-or-directory> [flags]
  excel-game-cli inspect <workbook.xlsx> [flags]
  excel-game-cli format <workbook.xlsx> --out <workbook.xlsx> [flags]
  excel-game-cli init [directory]
  excel-game-cli version

Defaults:
  schema:     auto (config233 five-row headers, or Luban ##var/##type)
  format:     config233 (TSV with field-name and type rows)
  value-mode: string (cell values never become JSON numbers/bools implicitly)

Examples:
  excel-game-cli convert ./BusinessConfig --out ./generated --recursive
  excel-game-cli convert ItemConfig.xlsx --format json --out ./generated
  excel-game-cli convert ItemConfig.xlsx --format luban --out ./generated
  excel-game-cli format ItemConfig.xlsx --out ItemConfig.normalized.xlsx --auto-fit
`
	fmt.Print(strings.TrimSpace(usage) + "\n")
}
