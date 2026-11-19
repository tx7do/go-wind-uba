package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	"github.com/tx7do/go-utils/trans"

	auditV1 "go-wind-uba/api/gen/go/audit/service/v1"
)

// exportDevice 拼页面"操作平台"槽位:OS + 浏览器,缺失段以空串参与拼接。
func TestExportDevice(t *testing.T) {
	assert.Equal(t, "", exportDevice(nil), "nil 设备信息 → 空单元格")

	// 全空:TrimSpace 后应为空串,不得产出 " " 这种脏单元格。
	assert.Equal(t, "", exportDevice(&auditV1.DeviceInfo{}))

	// 双段齐全:OS + 空格 + 浏览器。
	assert.Equal(t, "Windows Chrome", exportDevice(&auditV1.DeviceInfo{
		OsName:      trans.Ptr("Windows"),
		BrowserName: trans.Ptr("Chrome"),
	}))

	// 只有 OS:右空白应被裁掉。
	assert.Equal(t, "iOS", exportDevice(&auditV1.DeviceInfo{OsName: trans.Ptr("iOS")}))

	// 只有浏览器:左空白应被裁掉。
	assert.Equal(t, "Safari", exportDevice(&auditV1.DeviceInfo{BrowserName: trans.Ptr("Safari")}))
}

// exportGeo 拼页面"地理位置"槽位:省 + 市。
func TestExportGeo(t *testing.T) {
	assert.Equal(t, "", exportGeo(nil))

	assert.Equal(t, "", exportGeo(&auditV1.GeoLocation{}))

	assert.Equal(t, "浙江省 杭州市", exportGeo(&auditV1.GeoLocation{
		Province: trans.Ptr("浙江省"),
		City:     trans.Ptr("杭州市"),
	}))

	// 只有省或只有市:空白裁剪,不留脏格。
	assert.Equal(t, "浙江省", exportGeo(&auditV1.GeoLocation{Province: trans.Ptr("浙江省")}))
	assert.Equal(t, "杭州市", exportGeo(&auditV1.GeoLocation{City: trans.Ptr("杭州市")}))
}

// exportOrderBy 排序参数缺省回退 -created_at,请求里的排序必须透传
// (导出与页面列表同序,否则"所见非所得")。
func TestExportOrderBy(t *testing.T) {
	assert.Equal(t, "-created_at", exportOrderBy(nil), "nil 请求 → 默认倒序")
	assert.Equal(t, "-created_at", exportOrderBy(&paginationV1.PagingRequest{}))
	assert.Equal(t, "-created_at", exportOrderBy(&paginationV1.PagingRequest{OrderBy: nil}))
	assert.Equal(t, "-created_at", exportOrderBy(&paginationV1.PagingRequest{OrderBy: trans.Ptr("")}),
		"空串视为未指定")
	assert.Equal(t, "-ip_address", exportOrderBy(&paginationV1.PagingRequest{OrderBy: trans.Ptr("-ip_address")}))
}

// 五类审计导出的列定义必须带表头(列定义在注册时静态构造,
// 这里钉住"列数与表头非空"防止误删导致导出空表)。
func TestAuditLogExportColumnsShape(t *testing.T) {
	require.NotEmpty(t, apiAuditLogExportColumns)
	require.NotEmpty(t, loginAuditLogExportColumns)
	require.NotEmpty(t, operationAuditLogExportColumns)
	require.NotEmpty(t, dataAccessAuditLogExportColumns)
	require.NotEmpty(t, permissionAuditLogExportColumns)

	// 每列必须有非空标题与取值函数。
	for name, cols := range map[string]int{
		"api":        len(apiAuditLogExportColumns),
		"login":      len(loginAuditLogExportColumns),
		"operation":  len(operationAuditLogExportColumns),
		"dataAccess": len(dataAccessAuditLogExportColumns),
		"permission": len(permissionAuditLogExportColumns),
	} {
		assert.GreaterOrEqual(t, cols, 7, "%s 审计导出列数不应被意外删减", name)
	}
	for _, c := range apiAuditLogExportColumns {
		assert.NotEmpty(t, c.Title)
		assert.NotNil(t, c.Value)
	}
	for _, c := range loginAuditLogExportColumns {
		assert.NotEmpty(t, c.Title)
		assert.NotNil(t, c.Value)
	}
	for _, c := range operationAuditLogExportColumns {
		assert.NotEmpty(t, c.Title)
		assert.NotNil(t, c.Value)
	}
	for _, c := range dataAccessAuditLogExportColumns {
		assert.NotEmpty(t, c.Title)
		assert.NotNil(t, c.Value)
	}
	for _, c := range permissionAuditLogExportColumns {
		assert.NotEmpty(t, c.Title)
		assert.NotNil(t, c.Value)
	}
}
