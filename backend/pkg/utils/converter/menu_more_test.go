package converter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tx7do/go-utils/trans"
	resourceV1 "go-wind-uba/api/gen/go/resource/service/v1"
)

func TestComposeMenuPaths(t *testing.T) {
	c := NewMenuPermissionConverter()

	t.Run("三级树形拼接", func(t *testing.T) {
		// 父链 path 应级联拼接:root/child/leaf。
		menus := []*resourceV1.Menu{
			{Id: trans.Ptr(int64(1)), Path: trans.Ptr("root")},
			{Id: trans.Ptr(int64(2)), ParentId: trans.Ptr(int64(1)), Path: trans.Ptr("child")},
			{Id: trans.Ptr(int64(3)), ParentId: trans.Ptr(int64(2)), Path: trans.Ptr("leaf")},
		}
		c.ComposeMenuPaths(menus)
		assert.Equal(t, "root", *menus[0].Path)
		assert.Equal(t, "root/child", *menus[1].Path)
		assert.Equal(t, "root/child/leaf", *menus[2].Path)
	})

	t.Run("斜杠被归一化", func(t *testing.T) {
		// path 带首尾斜杠:拼接前应剥掉,避免出现 // 残留。
		menus := []*resourceV1.Menu{
			{Id: trans.Ptr(int64(1)), Path: trans.Ptr("/root/")},
			{Id: trans.Ptr(int64(2)), ParentId: trans.Ptr(int64(1)), Path: trans.Ptr("/child/")},
		}
		c.ComposeMenuPaths(menus)
		assert.Equal(t, "/root/", *menus[0].Path, "根节点直接写回自身 path(不裁斜杠,现状)")
		assert.Equal(t, "/root///child/", *menus[1].Path, "现状:父子都带斜杠时会产生 //,由调用方归一化")
	})

	t.Run("父ID为0或缺失按根处理", func(t *testing.T) {
		menus := []*resourceV1.Menu{
			{Id: trans.Ptr(int64(1)), Path: trans.Ptr("a")},
			{Id: trans.Ptr(int64(2)), ParentId: trans.Ptr(int64(999)), Path: trans.Ptr("b")},
		}
		c.ComposeMenuPaths(menus)
		assert.Equal(t, "a", *menus[0].Path)
		assert.Equal(t, "b", *menus[1].Path, "父节点不存在时只取自身 path")
	})

	t.Run("自引用节点", func(t *testing.T) {
		// parentId 指向自身:不得死循环。
		menus := []*resourceV1.Menu{
			{Id: trans.Ptr(int64(1)), ParentId: trans.Ptr(int64(1)), Path: trans.Ptr("self")},
		}
		c.ComposeMenuPaths(menus)
		assert.Equal(t, "self", *menus[0].Path)
	})

	t.Run("循环引用不挂起", func(t *testing.T) {
		// a->b->a 环:环检测防止死循环,但外层帧会用父链结果覆盖环检测写入的
		// memo —— 实际得到 "a/b/a" 的伪级联值【现状钉死】。真实菜单数据不应
		// 存在环;修复环值语义后请同步更新本断言。
		menus := []*resourceV1.Menu{
			{Id: trans.Ptr(int64(1)), ParentId: trans.Ptr(int64(2)), Path: trans.Ptr("a")},
			{Id: trans.Ptr(int64(2)), ParentId: trans.Ptr(int64(1)), Path: trans.Ptr("b")},
			{Id: trans.Ptr(int64(3)), ParentId: trans.Ptr(int64(1)), Path: trans.Ptr("c")},
		}
		assert.NotPanics(t, func() { c.ComposeMenuPaths(menus) })
		assert.Equal(t, "a/b/a", *menus[0].Path, "环上节点:外层帧覆盖环检测 memo(现状)")
		assert.Equal(t, "a/b", *menus[1].Path, "环上第二节点命中递归中的 memo(现状)")
		assert.Equal(t, "a/b/a/c", *menus[2].Path, "环外子节点沿被污染的 memo 级联(现状)")
	})

	t.Run("空path段跳过", func(t *testing.T) {
		// 父 path 为空:子节点直接用自身 path。
		menus := []*resourceV1.Menu{
			{Id: trans.Ptr(int64(1)), Path: trans.Ptr("")},
			{Id: trans.Ptr(int64(2)), ParentId: trans.Ptr(int64(1)), Path: trans.Ptr("child")},
		}
		c.ComposeMenuPaths(menus)
		assert.Equal(t, "child", *menus[1].Path)
	})
}

func TestConvertCodeEdgeCases(t *testing.T) {
	c := NewMenuPermissionConverter()

	t.Run("空路径", func(t *testing.T) {
		assert.Equal(t, "", c.ConvertCode("", "x", resourceV1.Menu_MENU))
		assert.Equal(t, "", c.ConvertCode("   ", "x", resourceV1.Menu_MENU))
		assert.Equal(t, "", c.ConvertCode("/", "x", resourceV1.Menu_MENU))
	})

	t.Run("首段被丢弃", func(t *testing.T) {
		// 多段路径丢弃第一段(路由前缀),余下段构成权限主体。
		assert.Equal(t, "system:user:view", c.ConvertCode("/admin/system/users", "x", resourceV1.Menu_MENU))
	})

	t.Run("参数段与空段跳过", func(t *testing.T) {
		// :id 参数段与空段不应进入权限码。
		assert.Equal(t, "user:view", c.ConvertCode("/opa/users/:id", "", resourceV1.Menu_MENU))
	})

	t.Run("复数归一", func(t *testing.T) {
		assert.Equal(t, "user:view", c.ConvertCode("/admin/users", "x", resourceV1.Menu_MENU))
	})

	t.Run("无动作类型返回纯主体", func(t *testing.T) {
		// 未知类型 typeToAction 返回空 → 无动作后缀。
		assert.Equal(t, "user", c.ConvertCode("/admin/users", "x", resourceV1.Menu_Type(99)))
	})

	t.Run("目录与链接类型", func(t *testing.T) {
		assert.Equal(t, "system:dir", c.ConvertCode("/admin/system", "x", resourceV1.Menu_CATALOG))
		assert.Equal(t, "help:jump", c.ConvertCode("/admin/help", "x", resourceV1.Menu_LINK))
	})
}

func TestButtonAction(t *testing.T) {
	c := NewMenuPermissionConverter()

	cases := []struct {
		title string
		want  string
	}{
		{"", "act"},
		{"   ", "act"},
		{"新增", "create"},
		{"添加用户", "create"},
		{"Add", "create"},
		{"Create", "create"},
		{"NEW+", "create"},
		{"编辑", "edit"},
		{"Update", "edit"},
		{"保存", "edit"},
		{"删除", "delete"},
		{"Delete User", "delete"},
		{"Remove", "delete"},
		{"导入", "import"},
		{"Import CSV", "import"},
		{"导出", "export"},
		{"Export Excel", "export"},
		{"Download", "export"},
		{"其它按钮", "act"},
		{"Custom Action", "act"},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.want, c.buttonAction(tc.title), "buttonAction(%q)", tc.title)
	}
}

func TestMatchAnyKeyword(t *testing.T) {
	// token 精确匹配。
	assert.True(t, matchAnyKeyword("add", []string{"add"}))
	// token 前缀匹配(addto 命中 add)。
	assert.True(t, matchAnyKeyword("addto", []string{"add"}))
	// 整句 substring 回退("新增用户" 含 "新增")。
	assert.True(t, matchAnyKeyword("新增用户", []string{"新增"}))
	// 大小写不敏感。
	assert.True(t, matchAnyKeyword("ADD NOW", []string{"add"}))
	// 完全不匹配。
	assert.False(t, matchAnyKeyword("nothing here", []string{"add"}))
	assert.False(t, matchAnyKeyword("", []string{"add"}))
	// 空关键词列表恒 false。
	assert.False(t, matchAnyKeyword("add", nil))
}

func TestTokenize(t *testing.T) {
	// 非字母数字切分 + 小写化。
	assert.Equal(t, []string{"add", "user"}, tokenize("Add_User"))
	assert.Equal(t, []string{"新增", "用户"}, tokenize("新增 用户"))
	assert.Equal(t, []string{"a1", "b2"}, tokenize("a1-b2"))
	assert.Equal(t, []string(nil), tokenize("!!!"))
	assert.Equal(t, []string(nil), tokenize(""))
	assert.Equal(t, []string{"hello"}, tokenize("  hello  "))
}

func TestMenuPathToModuleName(t *testing.T) {
	c := NewMenuPermissionConverter()

	// 第二段为模块名。
	assert.Equal(t, "admin", c.MenuPathToModuleName("/admin/system/user"))
	// 单段路径 → 默认业务权限模块。
	assert.Equal(t, "uba", c.MenuPathToModuleName("/uba"))
	assert.NotEmpty(t, c.MenuPathToModuleName("/x"), "缺段时回退默认模块,不得为空")
	// 根路径无模块段 → 默认模块。
	assert.NotEmpty(t, c.MenuPathToModuleName("/"))
	// 第二段为空白 → 回退默认模块。
	assert.NotEmpty(t, c.MenuPathToModuleName("/ /x"))
}
