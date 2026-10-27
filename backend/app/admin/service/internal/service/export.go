package service

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// 服务端全量导出的统一装配（:export 手动锚点的 service 侧）。
//
// 前端约定见管理台 api/composables/server-export.ts：
// GET <资源>:export?format=xlsx&query=<与列表页同源的 go-crud 查询 JSON>，
// 导出的即当前搜索条件看到的全部行（而非当前页）。鉴权/权限与各资源 List
// 同语义，在具体 service 方法里闸（requirePlatformAdmin 等），路由装配在
// server/export_http.go。
const (
	// ExportPageSize 分页拉取的页大小：与前端客户端聚合导出一致，List 端点已验证支持到 1000。
	ExportPageSize = 1000
	// DefaultExportMaxRows 单次导出行数上限（后端统一闸，防误操作拉爆内存，非权限边界）。
	DefaultExportMaxRows = 100000
)

// ExportPayload 导出文件产物：由 server 层手动注册的 :export handler 写回 HTTP 裸字节。
type ExportPayload struct {
	Filename  string // 含扩展名（ASCII slug + 时间戳）
	Mime      string
	Data      []byte
	Truncated bool // 达到行数上限被截断（handler 置 X-Export-Truncated 响应头）
}

// ExportColumn 一列 = 标题 + 从行记录取值（返回空串即空单元格）。
type ExportColumn[T any] struct {
	Title string
	Value func(T) string
}

// ExportOptions 一次导出的全部参数。
type ExportOptions[T any] struct {
	FilenameBase string // 文件名主体（不含时间戳与扩展名）
	SheetName    string // xlsx 工作表名
	Format       string // xlsx（默认）/ csv
	MaxRows      int    // <=0 用 DefaultExportMaxRows
	Columns      []ExportColumn[T]
	// FetchPage 拉第 page 页（1 起）。实现方必须复用列表接口的同一查询语义
	// （query 过滤、orderBy 排序），"导出的即当前搜索看到的"才成立，行序也有稳定含义。
	FetchPage func(ctx context.Context, page, pageSize uint32) ([]T, error)
}

// BuildExport 按页拉满全部数据并编码为 xlsx/csv。
// 行序稳定性：按页翻取，导出期间新写入的行可能造成跨页漂移（一切分页导出的通病），
// 以列表同款 orderBy 锚定可尽量减小。
func BuildExport[T any](ctx context.Context, opt ExportOptions[T]) (*ExportPayload, error) {
	maxRows := opt.MaxRows
	if maxRows <= 0 {
		maxRows = DefaultExportMaxRows
	}
	if opt.Format == "" {
		opt.Format = "xlsx"
	}

	titles := make([]string, len(opt.Columns))
	for i, c := range opt.Columns {
		titles[i] = c.Title
	}

	rows := make([][]string, 0, ExportPageSize)
	truncated := false
	for page := uint32(1); ; page++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		items, err := opt.FetchPage(ctx, page, ExportPageSize)
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			if len(rows) >= maxRows {
				truncated = true
				break
			}
			row := make([]string, len(opt.Columns))
			for i, c := range opt.Columns {
				if c.Value != nil {
					row[i] = c.Value(item)
				}
			}
			rows = append(rows, row)
		}
		if truncated || uint32(len(items)) < ExportPageSize {
			break
		}
	}

	payload := &ExportPayload{
		Filename:  fmt.Sprintf("%s-%s", opt.FilenameBase, time.Now().Format("20060102150405")),
		Truncated: truncated,
	}
	switch opt.Format {
	case "csv":
		payload.Filename += ".csv"
		payload.Mime = "text/csv; charset=utf-8"
		data, err := encodeExportCSV(titles, rows)
		if err != nil {
			return nil, err
		}
		payload.Data = data
	default:
		payload.Filename += ".xlsx"
		payload.Mime = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
		data, err := encodeExportXLSX(opt.SheetName, titles, rows)
		if err != nil {
			return nil, err
		}
		payload.Data = data
	}
	return payload, nil
}

// encodeExportCSV 带 UTF-8 BOM（无 BOM 时 Excel 按 ANSI 解码，中文全乱码）。
// CSV 被 Excel 打开时 =、+、-、@ 开头的单元格会被当公式求值（CSV 注入），
// 按 OWASP 建议加 ' 前缀；xlsx 路径不需要：StreamWriter 的纯字符串写的是
// inlineStr，不会成为公式。
func encodeExportCSV(titles []string, rows [][]string) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString("\uFEFF")
	w := csv.NewWriter(&buf)
	if err := w.Write(sanitizeExportCSVCells(titles)); err != nil {
		return nil, err
	}
	for _, row := range rows {
		if err := w.Write(sanitizeExportCSVCells(row)); err != nil {
			return nil, err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func sanitizeExportCSVCells(row []string) []string {
	out := make([]string, len(row))
	for i, cell := range row {
		if strings.HasPrefix(cell, "=") || strings.HasPrefix(cell, "+") ||
			strings.HasPrefix(cell, "-") || strings.HasPrefix(cell, "@") {
			cell = "'" + cell
		}
		out[i] = cell
	}
	return out
}

// encodeExportXLSX 用 StreamWriter 边写边序列化，十万行量级也不把整表再复制一份进内存。
func encodeExportXLSX(sheet string, titles []string, rows [][]string) ([]byte, error) {
	f := excelize.NewFile()
	defer f.Close()

	if sheet != "" {
		if err := f.SetSheetName("Sheet1", sheet); err != nil {
			return nil, err
		}
	} else {
		sheet = "Sheet1"
	}

	headerStyle, err := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
	if err != nil {
		return nil, err
	}

	sw, err := f.NewStreamWriter(sheet)
	if err != nil {
		return nil, err
	}

	if err := sw.SetRow("A1", exportStringsToValues(titles), excelize.RowOpts{StyleID: headerStyle}); err != nil {
		return nil, err
	}
	for i, row := range rows {
		cell, err := excelize.CoordinatesToCellName(1, i+2)
		if err != nil {
			return nil, err
		}
		if err := sw.SetRow(cell, exportStringsToValues(row)); err != nil {
			return nil, err
		}
	}

	if err := sw.Flush(); err != nil {
		return nil, err
	}
	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func exportStringsToValues(row []string) []interface{} {
	vals := make([]interface{}, len(row))
	for i, v := range row {
		vals[i] = v
	}
	return vals
}

// —— 行取值辅助：optional 指针字段未设置导出空串（区别于 0 / 空文本）——

func exportStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func exportUint32(p *uint32) string {
	if p == nil {
		return ""
	}
	return fmt.Sprintf("%d", *p)
}

func exportBool(p *bool) string {
	if p == nil {
		return ""
	}
	if *p {
		return "true"
	}
	return "false"
}

func exportEnumText[T interface {
	~int32
	String() string
}](p *T) string {
	if p == nil {
		return ""
	}
	return (*p).String()
}

// exportTimestamp 本地时区的时间列，与列表页 formatDateTime 的展示口径一致。
func exportTimestamp(ts *timestamppb.Timestamp) string {
	if ts == nil || ts.AsTime().IsZero() {
		return ""
	}
	return ts.AsTime().Local().Format(time.DateTime)
}
