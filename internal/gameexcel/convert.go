package gameexcel

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"

	"github.com/xuri/excelize/v2"
)

func ConvertDirectory(input string, cfg Config) error {
	cfg = cfg.normalized()
	files, root, err := discoverWorkbooks(input, cfg.Recursive)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("no .xlsx or .xlsm workbooks found in %s", input)
	}
	inputInfo, statErr := os.Stat(input)
	if statErr != nil {
		return statErr
	}
	outputIsFile := !inputInfo.IsDir() && filepath.Ext(cfg.OutputDir) != "" && !strings.HasSuffix(cfg.OutputDir, string(filepath.Separator))
	if !outputIsFile {
		if err := os.MkdirAll(cfg.OutputDir, 0o755); err != nil {
			return fmt.Errorf("create output directory: %w", err)
		}
	}
	workers := cfg.Workers
	if workers <= 0 {
		workers = runtime.NumCPU()
	}
	if workers > len(files) {
		workers = len(files)
	}
	jobs := make(chan string)
	var wg sync.WaitGroup
	var firstErr error
	var errMu sync.Mutex
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for path := range jobs {
				if err := convertOne(path, root, input, cfg); err != nil {
					errMu.Lock()
					if firstErr == nil {
						firstErr = err
					}
					errMu.Unlock()
				}
			}
		}()
	}
	for _, path := range files {
		jobs <- path
	}
	close(jobs)
	wg.Wait()
	return firstErr
}

func convertOne(path, root, input string, cfg Config) error {
	format := Format(strings.ToLower(strings.TrimSpace(cfg.Format)))
	if format != FormatConfig233 && format != FormatLuban && format != FormatJSON && format != FormatTSV && format != FormatCSV {
		return fmt.Errorf("unsupported output format %q", cfg.Format)
	}
	if format == FormatLuban {
		out := outputPath(path, root, input, cfg.OutputDir, format)
		if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
			return err
		}
		if err := ConvertToLubanWorkbook(path, out, cfg); err != nil {
			return err
		}
		fmt.Println(out)
		return nil
	}
	table, err := OpenTable(path, cfg)
	if err != nil {
		return err
	}
	out := outputPath(path, root, input, cfg.OutputDir, format)
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return err
	}
	file, err := os.Create(out)
	if err != nil {
		return fmt.Errorf("create %s: %w", out, err)
	}
	err = WriteTable(file, table, cfg, format)
	closeErr := file.Close()
	if err != nil {
		return fmt.Errorf("write %s: %w", out, err)
	}
	if closeErr != nil {
		return fmt.Errorf("close %s: %w", out, closeErr)
	}
	fmt.Println(out)
	return nil
}

func Inspect(path string, cfg Config) (Summary, error) {
	table, err := OpenTable(path, cfg)
	if err != nil {
		return Summary{}, err
	}
	return Summary{Name: table.Name, SourcePath: table.SourcePath, Sheet: table.Sheet, Schema: table.Schema, Rows: len(table.Rows), Columns: len(table.Fields), Fields: table.Fields}, nil
}

func discoverWorkbooks(input string, recursive bool) ([]string, string, error) {
	info, err := os.Stat(input)
	if err != nil {
		return nil, "", fmt.Errorf("stat input %s: %w", input, err)
	}
	if !info.IsDir() {
		if !isWorkbook(input) {
			return nil, "", fmt.Errorf("input is not an .xlsx or .xlsm workbook: %s", input)
		}
		return []string{input}, filepath.Dir(input), nil
	}
	root, err := filepath.Abs(input)
	if err != nil {
		return nil, "", err
	}
	var files []string
	if recursive {
		err = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if !entry.IsDir() && isWorkbook(path) {
				files = append(files, path)
			}
			return nil
		})
	} else {
		entries, readErr := os.ReadDir(root)
		err = readErr
		for _, entry := range entries {
			if !entry.IsDir() && isWorkbook(filepath.Join(root, entry.Name())) {
				files = append(files, filepath.Join(root, entry.Name()))
			}
		}
	}
	if err != nil {
		return nil, "", err
	}
	sort.Strings(files)
	return files, root, nil
}

func isWorkbook(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".xlsx" || ext == ".xlsm"
}

func outputPath(path, root, input, outputDir string, format Format) string {
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	ext := map[Format]string{FormatConfig233: ".tsv.txt", FormatJSON: ".json", FormatTSV: ".tsv", FormatCSV: ".csv", FormatLuban: ".xlsx"}[format]
	if ext == "" {
		ext = ".out"
	}
	if info, err := os.Stat(outputDir); err == nil && !info.IsDir() {
		return outputDir
	}
	if inputInfo, err := os.Stat(input); err == nil && !inputInfo.IsDir() {
		if filepath.Ext(outputDir) != "" && !strings.HasSuffix(outputDir, string(filepath.Separator)) {
			return outputDir
		}
	}
	relDir := ""
	if root != "" {
		rel, err := filepath.Rel(root, filepath.Dir(path))
		if err == nil && rel != "." {
			relDir = rel
		}
	}
	return filepath.Join(outputDir, relDir, base+ext)
}

func ConvertToLubanWorkbook(input, output string, cfg Config) error {
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	f, err := excelize.OpenFile(input, excelize.Options{RawCellValue: false})
	if err != nil {
		return fmt.Errorf("open workbook %s: %w", input, err)
	}
	defer f.Close()
	table, err := readTable(f, input, cfg.normalized())
	if err != nil {
		return err
	}
	if err := rewriteAsLuban(f, table); err != nil {
		return err
	}
	if cfg.Formatting.Normalize || cfg.Formatting.AutoFit {
		if err := normalizeFile(f, cfg.normalized()); err != nil {
			return err
		}
	}
	if err := f.SaveAs(output); err != nil {
		return fmt.Errorf("save Luban workbook %s: %w", output, err)
	}
	return nil
}
