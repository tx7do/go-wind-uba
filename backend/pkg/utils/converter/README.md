# 权限码转换器

## 概述

本工具把**菜单**与 **API** 两类资源映射成同一种权限码，权限码用于权限控制与校验。
权限码由模块（`Module`）、子模块（`SubModule`）与动作（`Action`）组成，用 `:` 串联，
均为单数、小写，多个单词用 `-` 连接：`{module}:{submodule}:{action}`（无子模块时省略）。

两个转换器分别负责一半：

| 入口 | 谁在用 | 说明 |
|---|---|---|
| `MenuPermissionConverter.ConvertCode(path, title, type)` | `app/admin/service/.../permission_service.go:279` | 从菜单树产出权限码 |
| `ApiPermissionConverter.ConvertCodeByPath(method, path)` | `app/core/service/.../permission_service.go:232` | 从 API 产出权限码，用于把 API 绑到权限上 |
| `ApiPermissionConverter.ConvertCodeByOperationID(operationID)` | 生产路径**未使用**（同文件上一行被注释掉） | 行为仅由测试锁定 |

## 关键不变式：动作词表由菜单侧决定

`core` 的 `appendAPis` 用**字符串相等**把 API 码与菜单产出的权限码配对
（`codes[code]` ↔ `perm.GetCode()`）。因此 API 侧能产出的 `action` 必须落在菜单侧的词表里
（`dir / view / create / edit / delete / import / export / jump / act`），
否则这条 API **永远绑不到任何权限，而且是静默的**。

这就是 `api.go` 里把 `list / get / retrieve / query / exist` 一律收敛成 `view` 的原因
（`menu.go` 的 `Menu_MENU`/`Menu_EMBEDDED` 产出的是 `view`，不产出 `list`）。
新增动词映射前先想清楚它在菜单侧有没有对应项。

## 命名与格式规则

- 路径到模块规则（菜单侧 `ConvertCode`）：
    - `strings.TrimSpace` 后去掉首尾 `/`；为空则返回空码。
    - 按 `/` 切段；**段数 > 1 时丢掉第一段**（约定：菜单树顶层是布局段，不参与权限命名）。
      例：`/admin/settings` → `setting`，而单段的 `/users` → `user`（不会被误丢）。
    - 每段 `inflection.Singular` 单数化，跳过空白段与以 `:` 开头的段。
    - 用 `:` 连接为权限主体，再接动作。

- 路径到模块规则（API 侧 `pathToResource`）：
    - `stripVersionPrefix`：去掉开头的 `api` 段与第一个版本段（`v1`、`v1.9`、`v10`…，
      仅当它出现在前两段时）。`/api/v1/admin/settings` → `admin/settings`。
    - `removePathParams`：丢掉 `{id}` 形式的参数段（允许 `{ id }` 带空白）。
    - 只取剩下的**第一段**，单数化后再取 `:` 之前的部分。
      例：`/admin/v1/tasks:type-names` → `task`。
    - 若资源名为空（路径退化成纯参数段，如 `/v1/{id}`），`ConvertCodeByPath` 返回**空串**，
      调用方按 `code == ""` 跳过 —— 不产出 `":view"` 这种畸形码。

- 动作（Action）命名：短小英文标识，统一小写，多词用 `-`。

## Menu_Type 到 Action 的映射

- `Menu_CATALOG` -> `dir`
- `Menu_MENU` -> `view`
- `Menu_BUTTON` -> 按按钮标题分类（见下表）
- `Menu_EMBEDDED` -> `view`
- `Menu_LINK` -> `jump`
- 未知类型 -> 空字符串（此时 `ConvertCode` 返回**不带动作后缀**的权限主体）

> 早期文档把 `Menu_MENU` 写成 `access`，代码从未产出过 `access`，以本表为准。

## 按钮标题到 Action 的映射（`buttonAction`）

判定顺序：`create` → `edit` → `delete` → `import` → `export`，都不命中则 `act`；
标题为空（含只有空白）直接 `act`。

| 结果 | 关键词 |
|---|---|
| `create` | `add`、`addto`、`add+`、`create`、`new`、`plus`、`append`、`新增`、`添加`、`创建` |
| `edit` | `edit`、`update`、`modify`、`save`、`patch`、`保存`、`修改`、`更新`、`编辑` |
| `delete` | `delete`、`del`、`remove`、`destroy`、`drop`、`discard`、`trash`、`删除`、`移除`、`弃用`、`清除` |
| `import` | `import`、`importcsv`、`importexcel`、`导入`、`导入为` |
| `export` | `export`、`download`、`exportcsv`、`exportexcel`、`导出`、`下载`、`导出为` |

匹配策略（`matchAnyKeyword`）：标题先 `TrimSpace` + 转小写；先按分词做**精确或前缀**匹配
（`Add-to-list` 因 token `add` 前缀命中 `create`，尽管里面有 `list`），
再回退到整句 `Contains`（`一键导出为Excel` 靠这条命中 `导出` → `export`）。

## HTTP 方法到 Action 的映射（`methodToAction`）

- 路径以 `/list` 结尾时**优先**判为 `view`（覆盖方法本身，`POST /v1/users/list` 也是 `view`）。
- 其余按 `GET→view`、`POST→create`、`PUT/PATCH→edit`、`DELETE→delete`；方法名大小写不敏感。
- 未列出的方法回退成小写方法名（如 `OPTIONS` → `options`）。这类码按上面的不变式注定绑不上，
  属预期行为：这类端点本来也不该被当成业务权限。

## 示例

| 输入 | 输出 |
|---|---|
| 路径 `/users`，`Menu_BUTTON`，标题 `新增` | `user:create` |
| 路径 `/admin/settings`，`Menu_MENU` | `setting:view` |
| 路径 `/reports`，`Menu_BUTTON`，标题 `一键导出为Excel` | `report:export` |
| `GET /v1/users` | `user:view` |
| `DELETE /v1/users/{id}` | `user:delete` |
| `TaskService_ListTaskTypeName` | `task:task-type-name:view` |
| `Task_DeleteTask` | `task:task:delete` |

## 单元测试

- `menu_test.go`：`ConvertCode` 的路径处理（含丢弃首段）、单数化与各 `Menu_Type` 分支、
  `typeToAction` / `buttonAction` 的标题映射。
- `api_test.go`：`ConvertCodeByPath` / `ConvertCodeByOperationID`、
  `stripVersionPrefix` / `removePathParams` / `singularizeSegments` / `methodToAction`。

运行（在 `backend/` 下）：

```bash
go test ./pkg/utils/converter -v
```
