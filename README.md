# excel-game-cli

面向 `server-project-sf-go` 与 `tuanjie-project-sf` 配表的 Go 单二进制工具。

默认策略是 `config233`：读取五行表头，保留 `Server` 字段名和 `type` 行，导出 config233 兼容 TSV。单元格值默认始终按字符串处理；不会因为 Excel 单元格看起来像数字或布尔值，就在 JSON 中隐式变成数字或布尔值。需要强类型时显式传 `--value-mode typed`。

## 功能

- config233 五行表头自动识别：注释、中文名、Client、type、Server；首列默认跳过。
- 批量并行转换 `.xlsx` / `.xlsm`，worker 数量可配置。
- 导出 `config233`、`json`、`tsv`、`csv`。
- Luban 是独立的可选策略，使用 `##var` / `##type` / `##comment` 基础表头，不影响 config233 默认路径。
- `format` 原地读写工作簿：默认按字符串写回非公式单元格，统一字体/大小，自适应列宽和行高。
- 改样式时只替换字体字段，尽量保留原有背景色、边框、填充、对齐、数字格式和合并关系。
- JSON 配置文件驱动，命令行参数可以覆盖配置。

## 快速开始

```powershell
go build -trimpath -ldflags="-s -w" -o bin\excel-game-cli.exe .\cmd\excel-game-cli

# 默认 config233 TSV：generated\ItemConfig.tsv.txt
.\bin\excel-game-cli.exe convert .\BusinessConfig --recursive --out .\generated

# JSON / TSV / CSV。默认值仍是字符串
.\bin\excel-game-cli.exe convert .\ItemConfig.xlsx --format json --out .\generated
.\bin\excel-game-cli.exe convert .\ItemConfig.xlsx --format tsv  --out .\generated
.\bin\excel-game-cli.exe convert .\ItemConfig.xlsx --format csv  --out .\generated

# 显式开启 JSON 数字/布尔类型
.\bin\excel-game-cli.exe convert .\ItemConfig.xlsx --format json --value-mode typed --out .\generated

# 查看表结构
.\bin\excel-game-cli.exe inspect .\ItemConfig.xlsx

# 生成一个可编辑配置
.\bin\excel-game-cli.exe init .

# 统一字体、尺寸并自适应宽高；必须显式指定输出或 --in-place
.\bin\excel-game-cli.exe format .\ItemConfig.xlsx --out .\ItemConfig.normalized.xlsx
```

## 配置

运行 `excel-game-cli init` 生成 `excel-game.config.json`。默认配置等价于 `excel-game.config.example.json`。

`workers: 0` 表示使用 CPU 数量。`schema: auto` 会先探测 Luban 的 `##var`，否则按 config233 处理。真实项目如有不同表头，只改 `header`，不需要改代码。

## 输出约定

- `config233`：第一行字段名、第二行类型、后续为字符串数据，扩展名 `.tsv.txt`。
- `tsv` / `csv`：第一行为字段名，后续为数据；空值保持空字符串。
- `json`：默认输出对象数组，所有值为 JSON string；`json_shape: map` 时用 `id` 或 `key` 字段做键。
- `luban`：输出新的 `.xlsx`，用 `##var`、`##type`、`##comment` 三行基础表头。

## 验证

```powershell
go test ./...
go test ./internal/gameexcel -bench=BenchmarkWriteJSON -benchmem
go vet ./...
```

这个工具使用 `excelize` 读写 OOXML，不依赖本机安装 Office。Excel 的复杂公式、图表或外部连接不属于 config233 导表主路径，格式化时公式单元格会跳过字符串化。

设计上参考了 OfficeCLI 的单二进制、结构化 JSON、批处理和样式合并思路；本项目只聚焦游戏配置表导出与轻量 Excel 格式整理。
