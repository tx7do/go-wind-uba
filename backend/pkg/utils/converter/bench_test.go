package converter

import (
	"fmt"
	"testing"

	"github.com/tx7do/go-utils/trans"

	resourceV1 "go-wind-uba/api/gen/go/resource/service/v1"
)

// BenchmarkConvertCodeByPath 权限码转换在批量同步 API 定义时逐条执行。
func BenchmarkConvertCodeByPath(b *testing.B) {
	c := NewApiPermissionConverter()
	paths := make([][2]string, 0, 100)
	for i := 0; i < 100; i++ {
		paths = append(paths, [2]string{"GET", fmt.Sprintf("/v1/user-groups/%d/items", i)})
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		p := paths[i%len(paths)]
		if c.ConvertCodeByPath(p[0], p[1]) == "" {
			b.Fatal("unexpected empty code")
		}
	}
}

// BenchmarkConvertCodeByOperationID gRPC operationID 转换(含驼峰拆分/复数归一/词表映射)。
func BenchmarkConvertCodeByOperationID(b *testing.B) {
	c := NewApiPermissionConverter()
	ops := make([]string, 0, 100)
	for i := 0; i < 100; i++ {
		ops = append(ops, fmt.Sprintf("TaskService_ListTaskTypeName%d", i))
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if c.ConvertCodeByOperationID(ops[i%len(ops)]) == "" {
			b.Fatal("unexpected empty code")
		}
	}
}

// BenchmarkComposeMenuPaths 三级树(3×5×5=125 节点)的全量 path 拼接。
func BenchmarkComposeMenuPaths(b *testing.B) {
	c := NewMenuPermissionConverter()
	menus := makeMenuTree(3, 5, 5)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		c.ComposeMenuPaths(menus)
	}
}

// makeMenuTree 构造 depth 层、每层 fan 个孩子的菜单树(id 从 1 递增)。
func makeMenuTree(depth, fan, _ int) []*resourceV1.Menu {
	var menus []*resourceV1.Menu
	nextID := int64(1)
	root := &resourceV1.Menu{Id: trans.Ptr(nextID), Path: trans.Ptr("root")}
	nextID++
	menus = append(menus, root)

	parents := []*resourceV1.Menu{root}
	for d := 1; d < depth; d++ {
		var next []*resourceV1.Menu
		for _, p := range parents {
			for i := 0; i < fan; i++ {
				m := &resourceV1.Menu{
					Id:       trans.Ptr(nextID),
					ParentId: p.Id,
					Path:     trans.Ptr(fmt.Sprintf("seg-%d", nextID)),
				}
				nextID++
				menus = append(menus, m)
				next = append(next, m)
			}
		}
		parents = next
	}
	return menus
}

// BenchmarkButtonAction 按钮标题 → 动作词(菜单权限生成逐条执行)。
func BenchmarkButtonAction(b *testing.B) {
	c := NewMenuPermissionConverter()
	titles := []string{"新增用户", "Edit Profile", "删除", "导出报表", "Custom Action"}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		c.buttonAction(titles[i%len(titles)])
	}
}

// BenchmarkSingularizeSegments 复数归一(路径/operationID 转换的公共热点)。
func BenchmarkSingularizeSegments(b *testing.B) {
	c := NewApiPermissionConverter()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		c.singularizeSegments("tasks:type-names--user-groups")
	}
}
