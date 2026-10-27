package service

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/tx7do/go-utils/trans"

	notificationV1 "go-wind-uba/api/gen/go/notification/service/v1"
)

func unix(sec int64) time.Time {
	return time.Unix(sec, 0)
}

func bytesReader(b []byte) *bytes.Reader {
	return bytes.NewReader(b)
}

func TestBuildExportPagingAndTruncation(t *testing.T) {
	// 2500 行数据、每页 1000：应拉 3 页，且 MaxRows=2000 时截断。
	total := 2500
	pages := map[uint32]int{}
	opt := ExportOptions[*notificationV1.NotificationDelivery]{
		FilenameBase: "notification-deliveries",
		SheetName:    "投递台账",
		Format:       "xlsx",
		MaxRows:      2000,
		Columns: []ExportColumn[*notificationV1.NotificationDelivery]{
			{"投递时间", func(d *notificationV1.NotificationDelivery) string { return exportTimestamp(d.CreatedAt) }},
			{"投递目标", func(d *notificationV1.NotificationDelivery) string { return exportStr(d.Target) }},
		},
		FetchPage: func(ctx context.Context, page, pageSize uint32) ([]*notificationV1.NotificationDelivery, error) {
			pages[page]++
			out := make([]*notificationV1.NotificationDelivery, 0, pageSize)
			start := int((page - 1) * pageSize)
			for i := start; i < start+int(pageSize) && i < total; i++ {
				out = append(out, &notificationV1.NotificationDelivery{
					Id:        trans.Ptr(uint32(i + 1)),
					Target:    trans.Ptr("user***@example.com"),
					CreatedAt: timestamppb.New(unix(int64(i))),
				})
			}
			return out, nil
		},
	}

	payload, err := BuildExport(context.Background(), opt)
	if err != nil {
		t.Fatalf("BuildExport: %v", err)
	}
	if !payload.Truncated {
		t.Error("expected truncated=true at MaxRows=2000 with 2500 rows")
	}
	if len(pages) != 3 {
		t.Errorf("expected 3 pages fetched, got %d", len(pages))
	}
	if payload.Mime != "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" {
		t.Errorf("unexpected mime: %s", payload.Mime)
	}
	if !strings.HasPrefix(payload.Filename, "notification-deliveries-") ||
		!strings.HasSuffix(payload.Filename, ".xlsx") {
		t.Errorf("unexpected filename: %s", payload.Filename)
	}

	// XLSX 回读：标题行 + 2000 数据行，最后一行是第 2000 条（id=2000 → 时间锚 1999s）。
	f, err := excelize.OpenReader(bytesReader(payload.Data))
	if err != nil {
		t.Fatalf("open xlsx: %v", err)
	}
	defer f.Close()
	rows, err := f.GetRows("投递台账")
	if err != nil {
		t.Fatalf("get rows: %v", err)
	}
	if len(rows) != 2001 {
		t.Fatalf("expected 2001 rows (header+2000), got %d", len(rows))
	}
	if rows[0][0] != "投递时间" || rows[0][1] != "投递目标" {
		t.Errorf("unexpected header: %v", rows[0])
	}
	if rows[1][1] != "user***@example.com" {
		t.Errorf("unexpected first data row: %v", rows[1])
	}
}

func TestBuildExportCSVSanitizeAndBOM(t *testing.T) {
	opt := ExportOptions[*notificationV1.NotificationDelivery]{
		FilenameBase: "t",
		Format:       "csv",
		Columns: []ExportColumn[*notificationV1.NotificationDelivery]{
			{"目标", func(d *notificationV1.NotificationDelivery) string { return exportStr(d.Target) }},
			{"原因", func(d *notificationV1.NotificationDelivery) string { return exportStr(d.LastError) }},
		},
		FetchPage: func(ctx context.Context, page, pageSize uint32) ([]*notificationV1.NotificationDelivery, error) {
			return []*notificationV1.NotificationDelivery{
				{Target: trans.Ptr("=cmd|'/c calc'!A0"), LastError: trans.Ptr("-1 行失败")},
			}, nil
		},
	}
	payload, err := BuildExport(context.Background(), opt)
	if err != nil {
		t.Fatalf("BuildExport: %v", err)
	}
	text := string(payload.Data)
	// UTF-8 BOM = EF BB BF（不在源码里写不可见字符）
	if len(payload.Data) < 3 || payload.Data[0] != 0xEF || payload.Data[1] != 0xBB || payload.Data[2] != 0xBF {
		t.Error("CSV must start with UTF-8 BOM")
	}
	if !strings.Contains(text, "'=cmd") || !strings.Contains(text, "'-1 行失败") {
		t.Errorf("formula-injection guard missing: %q", text)
	}
	if payload.Mime != "text/csv; charset=utf-8" {
		t.Errorf("unexpected mime: %s", payload.Mime)
	}
}

func TestExportValueHelpers(t *testing.T) {
	if exportStr(nil) != "" || exportUint32(nil) != "" ||
		exportEnumText[notificationV1.DeliveryStatus](nil) != "" {
		t.Error("nil pointers must export as empty string")
	}
	if exportEnumText[notificationV1.DeliveryStatus](trans.Ptr(notificationV1.DeliveryStatus_SENT)) != "SENT" {
		t.Error("enum must export raw name")
	}
	if exportTimestamp(nil) != "" {
		t.Error("nil timestamp must export as empty string")
	}
}
