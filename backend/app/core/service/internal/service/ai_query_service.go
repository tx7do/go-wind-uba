package service

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/sashabaranov/go-openai"
	"github.com/tx7do/go-crud/viewer"
	"github.com/tx7do/go-utils/trans"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"google.golang.org/protobuf/types/known/structpb"

	aiV1 "go-wind-uba/api/gen/go/ai/service/v1"
	auditV1 "go-wind-uba/api/gen/go/audit/service/v1"
	ubaV1 "go-wind-uba/api/gen/go/uba/service/v1"
	"go-wind-uba/app/core/service/internal/data"
	clickhouseData "go-wind-uba/app/core/service/internal/data/clickhouse"
	dorisData "go-wind-uba/app/core/service/internal/data/doris"
	"go-wind-uba/app/core/service/internal/data/ent"
)

// ── 智能问数安全边界 ────────────────────────────────────────────────

const (
	// aiQueryRowLimit 结果行硬上限（缺 LIMIT 自动补；超限的显式 LIMIT 也被改写）。
	// 与自助 SQL 的ExecuteAdhocQuery 行数上限独立——这里是 NL 生成链自己的钳制。
	aiQueryRowLimit = 100

	// aiQueryCallTimeout SQL 生成与结论生成的单次 LLM 调用兜底时长。
	aiQueryCallTimeout = 90 * time.Second
)

// aiQueryTableWhitelist OLAP 表白名单：NL→SQL 只允许触达这些表（Doris 方言）。
// 键为表名，值为给 LLM 的列说明（拼进提示词当数据字典）。租户隔离列 tenant_id
// 每表必有，租户用户生成的 SQL 必须自带过滤（后置校验 fail-closed）。
var aiQueryTableWhitelist = map[string]string{
	"events_fact":   "行为事件事实表（核心）：event_time(事件时间), event_name(事件名), event_category, tenant_id, user_id(>0 为登录用户), device_id, global_user_id, session_id, platform(web/android/ios/…), os, app_version, channel, country, ip_city, object_type/object_id/object_name(事件对象), duration_ms, quantity, amount, properties(MAP<STRING,STRING> 事件属性), metrics(MAP<STRING,DOUBLE> 数值指标), risk_level, error_code。事件分析一律查此表，COUNT(DISTINCT user_id) 即人数",
	"sessions_fact": "访问会话事实表：session_id, tenant_id, user_id, start_time, end_time, session_date, duration_ms(会话时长), event_count, page_view_count, action_count, entry_page, exit_page, is_bounce(跳出), platform, os, country, total_amount, pay_event_count。会话时长/跳出率/停留分析查此表",
	"users_dim":     "用户维度表（一行=一个用户）：tenant_id, user_id, register_time, register_channel, first_active_date, last_active_date, user_level, vip_level, user_role, total_events, total_sessions, total_pay_amount, last_pay_time, risk_score, risk_level, platform, country。用户属性/生命周期/LTV 聚合查此表",
	"objects_dim":   "对象维度表（商品/关卡等）：tenant_id, object_id, object_type, object_name, category_path, price, currency, rarity, status。对象维度分析经 object_id JOIN events_fact",
	"user_tags":     "用户标签表：tenant_id, user_id, tag_id, tag_value, value_label, confidence, source, effective_time, expire_time, is_active。按标签圈人统计查此表",
	"risk_events":   "风险事件表：risk_event_id, tenant_id, user_id, risk_type, risk_level, risk_score, rule_id, rule_name, status(PENDING=待处理/HANDLED=已处理 等), occur_time, description。风控洞察查此表",
	"path_features": "路径特征表：path_id, tenant_id, user_id, session_id, first_event, last_event, path_length, is_converted, conversion_event, start_time, end_time, step_count。转化路径分析查此表",
	"id_mapping":    "ID-Mapping 表：global_user_id, tenant_id, id_type, id_value, confidence, first_seen, last_seen, is_active。跨设备身份关联查此表",
}

// aiQueryClickHouseDialectNotes / aiQueryDorisDialectNotes 引擎方言提示：
// 同一份表结构在两个引擎上 SQL 方言不同，按编译期开关给 LLM 对应提示。
var (
	aiQueryClickHouseDialectNotes = []string{
		"当前数据库是 ClickHouse。",
		"当前时间用 now()；时间差用 now() - INTERVAL 7 DAY；日期格式化用 toDate()/toStartOfDay()/toStartOfDay(now())。",
		"MAP 列（properties 等）取值用 properties['key']；聚合 MAP 值用 mapApply/mapFilter 谨慎使用，优先普通列。",
		"去重计数优先用 uniqExact()（等价 COUNT(DISTINCT x)）。",
	}
	aiQueryDorisDialectNotes = []string{
		"当前数据库是 Apache Doris（MySQL 方言）。",
		"当前时间用 NOW()；时间差用 DATE_SUB(NOW(), INTERVAL 7 DAY)；日期格式化用 DATE_FORMAT()。",
		"MAP 列（properties 等）取值用 element_at(properties, 'key')，优先普通列。",
		"去重计数用 COUNT(DISTINCT x)。",
	}
)

// tenantIDPatternFor 构造"tenant_id = {id}"存在性校验的正则（容忍别名与空白）。
func tenantIDPatternFor(tenantID int64) *regexp.Regexp {
	return regexp.MustCompile(`(?i)\btenant_id\s*=\s*` + fmt.Sprintf("%d", tenantID) + `\b`)
}

// aiQueryForbiddenPatterns 生成 SQL 的硬拒绝模式：写操作与危险语句。
// （执行侧 ExecuteAdhocQuery 另有首关键字白名单与注释/多语句拒绝，双保险。）
var aiQueryForbiddenPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\b(INSERT|UPDATE|DELETE|DROP|TRUNCATE|ALTER|CREATE|GRANT|REVOKE|SET)\b`),
	regexp.MustCompile(`\;`), // 多语句
}

// aiQuerySelectPattern 必须以 SELECT 或 WITH 开头（含前导空白/括号容忍）。
var aiQuerySelectPattern = regexp.MustCompile(`(?is)^\s*\(?\s*(SELECT|WITH)\b`)

// ── 服务 ───────────────────────────────────────────────────────────

// AiQueryService 智能问数：NL → 只读 SQL → 结构化结果 →（可选）自然语言结论。
// 查询目标为 OLAP 行为分析库（Doris/ClickHouse 编译期二选一），执行走与
// 自助 SQL 相同的受控只读通道（adhocAnalytics：首关键字白名单/禁注释/
// 禁多语句/行数上限/超时），生成侧另有表白名单、危险模式拒绝、LIMIT 钳制、
// 租户谓词复核四重防线。
type AiQueryService struct {
	aiV1.UnimplementedAiQueryServiceServer
	log             *log.Helper
	providerRepo    *data.AiProviderRepo
	usageRepo       *data.AiUsageLogRepo
	doris           *dorisData.AnalyticsRepo
	clickhouse      *clickhouseData.AnalyticsRepo
	auditRepo       *data.DataAccessAuditLogRepo
	eventSchemaRepo *data.EventSchemaRepo
	quotaRepo       *data.AiTenantQuotaRepo
}

func NewAiQueryService(
	ctx *bootstrap.Context,
	providerRepo *data.AiProviderRepo,
	usageRepo *data.AiUsageLogRepo,
	doris *dorisData.AnalyticsRepo,
	clickhouse *clickhouseData.AnalyticsRepo,
	auditRepo *data.DataAccessAuditLogRepo,
	eventSchemaRepo *data.EventSchemaRepo,
	quotaRepo *data.AiTenantQuotaRepo,
) *AiQueryService {
	return &AiQueryService{
		log:             ctx.NewLoggerHelper("ai_query/service/core-service"),
		providerRepo:    providerRepo,
		usageRepo:       usageRepo,
		doris:           doris,
		clickhouse:      clickhouse,
		auditRepo:       auditRepo,
		eventSchemaRepo: eventSchemaRepo,
		quotaRepo:       quotaRepo,
	}
}

// analyticsImpl 按编译期引擎开关路由（与 alertEvaluator/reportEngine 同一约定）。
func (s *AiQueryService) analyticsImpl() adhocAnalytics {
	if data.UseClickHouse && s.clickhouse != nil {
		return s.clickhouse
	}
	return s.doris
}

// Ask 智能问数主链路：NL→SQL（第一次调用）→ 护栏校验 → 只读执行 → NL 结论（第二次调用）。
func (s *AiQueryService) Ask(ctx context.Context, req *aiV1.AskAiQueryRequest) (*aiV1.AskAiQueryResponse, error) {
	// 双防线：租户用户生成的 SQL 必须自带 tenant_id = 自身租户 过滤（后置校验 fail-closed）；
	// 平台用户（tenantId=0）可跨租户分析。
	tenantID, _ := s.operatorTenant(ctx)
	tenantPattern := (*regexp.Regexp)(nil)
	if tenantID != 0 {
		tenantPattern = tenantIDPatternFor(tenantID)
	}
	question := strings.TrimSpace(req.GetQuestion())
	if question == "" {
		return nil, aiV1.ErrorBadRequest("question is required")
	}

	// 配额护栏：租户月度 token 用尽即拒绝（平台自用不限量）
	if qErr := s.quotaRepo.EnsureWithinQuota(ctx, s.usageRepo, tenantID); qErr != nil {
		return nil, qErr
	}

	provider, err := s.providerRepo.GetEnabledDefault(ctx)
	if err != nil {
		return nil, err
	}
	client, err := newOpenAIClientForProvider(provider)
	if err != nil {
		return nil, err
	}

	lang := req.GetLang()
	if lang == "" {
		lang = "zh-CN"
	}

	// 1. NL → SQL（携带对话历史以消解追问里的指代，如"那只看安卓的"）
	history := make([]aiV1.AiQueryHistoryItem, 0, len(req.GetHistory()))
	for _, h := range req.GetHistory() {
		history = append(history, aiV1.AiQueryHistoryItem{
			Question:      h.GetQuestion(),
			Sql:           h.GetSql(),
			ResultSummary: h.GetResultSummary(),
		})
	}
	sqlGenerated, genTokens, err := s.generateSQL(ctx, client, provider, question, lang, history, tenantID)
	if err != nil {
		return nil, err
	}

	// 2. 护栏：只读语句 / 危险模式拒绝 / LIMIT 钳制 / 租户谓词复核
	cleaned, err := sanitizeSQLFor(sqlGenerated, tenantPattern)
	if err != nil {
		return nil, aiV1.ErrorBadRequest("generated sql rejected: %v", err)
	}

	// 3. OLAP 只读通道执行（引擎侧再校验 + 行数上限 + 30s 超时）
	olapResp, execErr := s.analyticsImpl().ExecuteAdhocQuery(ctx, cleaned, aiQueryRowLimit)

	// 与自助 SQL 同一治理水位：模型生成的 SQL 也必须留数据访问审计
	s.writeAudit(ctx, cleaned, olapResp, execErr)

	resp := &aiV1.AskAiQueryResponse{
		Sql: cleaned,
	}
	if execErr != nil {
		// 执行失败把数据库错误回给前端（便于用户改问题重试），SQL 照样返回
		s.log.Errorf("ai query execute failed: %v", execErr)
		resp.ErrorMessage = trans.Ptr(execErr.Error())
		return resp, nil
	}

	resp.Columns = olapResp.GetColumns()
	resp.RowCount = uint32(olapResp.GetRowCount())
	resp.Rows = structpbRowsToStringRows(olapResp.GetRows())

	// 4.（可选）结果 → 自然语言结论
	if !req.GetWithAnswer() || len(resp.Rows) == 0 {
		return resp, nil
	}
	answer, answerTokens, err := s.generateAnswer(ctx, client, provider, question, cleaned, resp.Columns, resp.Rows, lang)
	if err != nil {
		s.log.Errorf("ai query answer failed, returning rows only: %v", err)
	} else {
		resp.Answer = trans.Ptr(answer)
	}

	resp.TotalTokens = uint32(genTokens + answerTokens)

	// 记账（两次调用合并总量；失败不阻断）
	if s.usageRepo != nil {
		modelName := ptrStrOr(provider.ModelName, "")
		_ = s.usageRepo.Create(ctx, &data.UsageRecord{
			UserID:      s.operatorID(ctx),
			TenantID:    tenantID,
			ModelName:   modelName,
			TotalTokens: int64(resp.GetTotalTokens()),
		})
	}

	return resp, nil
}

// operatorTenant / operatorID：viewer 上下文取操作者（unary 中间件注入）。
func (s *AiQueryService) operatorTenant(ctx context.Context) (int64, bool) {
	if vc, exist := viewer.FromContext(ctx); exist && vc != nil {
		return int64(vc.TenantID()), true
	}
	return 0, false
}

func (s *AiQueryService) operatorID(ctx context.Context) int64 {
	if uid, ok := viewerUserIDFromContext(ctx); ok {
		return uid
	}
	return 0
}

// generateSQL 数据字典 + 方言提示 + 对话历史 + 问题 → 只读 SQL。
func (s *AiQueryService) generateSQL(ctx context.Context, client *openai.Client, provider *ent.AiProvider, question, lang string, history []aiV1.AiQueryHistoryItem, tenantID int64) (string, int64, error) {
	dialectNotes := aiQueryDorisDialectNotes
	if data.UseClickHouse {
		dialectNotes = aiQueryClickHouseDialectNotes
	}

	var sb strings.Builder
	sb.WriteString("你是本系统的用户行为数据分析引擎。用户的提问永远是对上面列出的行为分析数据库表发起查询——哪怕措辞听起来像个人账号问题，一律直接生成 SQL，绝不拒绝。\n")
	sb.WriteString("根据表结构与用户问题，生成一条只读 SELECT 语句。\n")
	sb.WriteString("规则：\n")
	sb.WriteString("1. 只输出 SQL 本身，不要任何解释、不要 Markdown 代码块标记；\n")
	sb.WriteString("2. 只能使用下面列出的表；涉及多表时用 JOIN；\n")
	sb.WriteString("3. 必须带 LIMIT（最大 100）；时间比较用 event_time/start_time/occur_time 与当前时间；\n")
	fmt.Fprintf(&sb, "4. 结论文字用%s；\n", langName(lang))
	sb.WriteString("5. 用户问题可能包含对上一轮查询的指代（如\"那只看安卓的\"、\"换成按天分组\"），结合对话历史理解其完整含义后生成独立可执行的 SQL。\n")
	for _, note := range dialectNotes {
		sb.WriteString("6. " + note + "\n")
	}
	sb.WriteString("\n可用表：\n")
	for table, desc := range aiQueryTableWhitelist {
		sb.WriteString("- " + table + "：" + desc + "\n")
	}
	sb.WriteString(s.buildEventSemanticSection(ctx))
	if tenantID != 0 {
		fmt.Fprintf(&sb, "强制规则：所有查询必须包含 tenant_id = %d 过滤条件（本租户数据边界），缺失视为无效输出。\n\n", tenantID)
	}

	// few-shot 用真实消息序列锚定行为（上下文示例对行为的约束力远强于 system 指令）。
	// 示例 SQL 与方言提示同源：时间函数在 Doris/ClickHouse 上写法不同。
	tenantCond := ""
	if tenantID != 0 {
		tenantCond = fmt.Sprintf("WHERE tenant_id = %d AND ", tenantID)
	}
	dayExpr, dayWindow := "toDate(event_time)", "event_time >= now() - INTERVAL 7 DAY"
	if !data.UseClickHouse {
		dayExpr = "DATE_FORMAT(event_time, '%Y-%m-%d')"
		dayWindow = "event_time >= DATE_SUB(NOW(), INTERVAL 7 DAY)"
	}
	messages := make([]openai.ChatCompletionMessage, 0, len(history)*2+6)
	messages = append(messages, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleSystem,
		Content: sb.String(),
	})
	// few-shot 1：近 7 天每日活跃用户（锚定 events_fact 的时间/去重写法）
	messages = append(messages,
		openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleUser,
			Content: "最近 7 天每天的活跃用户数",
		},
		openai.ChatCompletionMessage{
			Role: openai.ChatMessageRoleAssistant,
			Content: "SELECT " + dayExpr + " AS day, COUNT(DISTINCT user_id) AS active_users " +
				"FROM events_fact " + tenantCond +
				dayWindow + " " +
				"GROUP BY day ORDER BY day LIMIT 100",
		},
	)
	// few-shot 2：事件排行——按事件名统计次数与人数
	messages = append(messages,
		openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleUser,
			Content: "统计每个事件的发生次数和触达人数",
		},
		openai.ChatCompletionMessage{
			Role: openai.ChatMessageRoleAssistant,
			Content: "SELECT event_name, COUNT(*) AS event_count, COUNT(DISTINCT user_id) AS user_count " +
				"FROM events_fact " + tenantCond +
				"user_id > 0 " +
				"GROUP BY event_name ORDER BY event_count DESC LIMIT 100",
		},
	)
	// 下标访问而非按值 range：AiQueryHistoryItem 内嵌 protoimpl.MessageState
	// （含 sync.Mutex），按值拷贝会连带复制锁。
	for i := range history {
		messages = append(messages,
			openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: history[i].GetQuestion()},
			openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, Content: history[i].GetSql()},
		)
	}
	messages = append(messages, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: question})

	qCtx, cancel := context.WithTimeout(ctx, aiQueryCallTimeout)
	defer cancel()
	resp, err := client.CreateChatCompletion(qCtx, openai.ChatCompletionRequest{
		Model:    ptrStrOr(provider.ModelName, "gpt-4o-mini"),
		Messages: messages,
	})
	if err != nil {
		return "", 0, aiV1.ErrorInternalServerError("ai query generate failed: %v", err)
	}
	tokens := int64(resp.Usage.TotalTokens)
	if len(resp.Choices) == 0 {
		return "", tokens, aiV1.ErrorInternalServerError("ai query generate empty")
	}
	raw := resp.Choices[0].Message.Content
	s.log.Infof("ai query raw model output: %s", raw)
	return raw, tokens, nil
}

// sanitizeSQLFor 四重护栏：SQL 提取 → 只读校验 → 危险模式拒绝 → LIMIT 钳制 → 租户谓词复核。
// （表白名单即整份字典—— ExecuteAdhocQuery 的引擎侧校验之外，这里拒绝
// 一切 FROM/JOIN 到白名单外表的语句。）
func sanitizeSQLFor(raw string, tenantPattern *regexp.Regexp) (string, error) {
	sqlText := extractSQL(raw)
	if sqlText == "" {
		return "", fmt.Errorf("no SQL statement found in model output")
	}

	if !aiQuerySelectPattern.MatchString(sqlText) {
		return "", fmt.Errorf("only SELECT/WITH statements are allowed")
	}
	for _, pat := range aiQueryForbiddenPatterns {
		if pat.MatchString(sqlText) {
			return "", fmt.Errorf("forbidden pattern detected")
		}
	}
	// 表白名单：提取 FROM/JOIN 后的表名逐一校验；模型偶发丢前缀或写单复数，依候选序列纠正
	tablePattern := regexp.MustCompile(`(?i)\b(FROM|JOIN)\s+([a-zA-Z_][a-zA-Z0-9_]*)`)
	matches := tablePattern.FindAllStringSubmatch(sqlText, -1)
	for _, m := range matches {
		table := strings.ToLower(m[2])
		if _, ok := aiQueryTableWhitelist[table]; ok {
			continue
		}
		corrected, ok := correctTableName(table, aiQueryTableWhitelist)
		if !ok {
			return "", fmt.Errorf("table %q is not in the whitelist", m[2])
		}
		fixPattern := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(table) + `\b`)
		sqlText = fixPattern.ReplaceAllStringFunc(sqlText, func(string) string {
			return corrected
		})
	}
	if tenantPattern != nil && !tenantPattern.MatchString(sqlText) {
		return "", fmt.Errorf("missing mandatory tenant_id filter")
	}
	// LIMIT 钳制：无 LIMIT 补 aiQueryRowLimit；LIMIT N>上限改写
	lower := strings.ToLower(sqlText)
	if !regexp.MustCompile(`(?i)\blimit\s+\d+`).MatchString(lower) {
		sqlText = fmt.Sprintf("SELECT * FROM (%s) AS _q LIMIT %d", sqlText, aiQueryRowLimit)
	} else {
		reLimit := regexp.MustCompile(`(?i)\blimit\s+(\d+)`)
		sqlText = reLimit.ReplaceAllStringFunc(sqlText, func(m string) string {
			n := 0
			// 正则已保证 m[5:] 是十进制数字，解析不会失败；显式忽略以过 errcheck
			_, _ = fmt.Sscanf(strings.TrimSpace(m[5:]), "%d", &n)
			if n > aiQueryRowLimit {
				return fmt.Sprintf("LIMIT %d", aiQueryRowLimit)
			}
			return m
		})
	}
	return sqlText, nil
}

// generateAnswer 结果集 → 自然语言结论（第二次调用）。
func (s *AiQueryService) generateAnswer(ctx context.Context, client *openai.Client, provider *ent.AiProvider, question, sqlText string, columns []string, rows []*aiV1.AiQueryRow, lang string) (string, int64, error) {
	var sb strings.Builder
	fmt.Fprintf(&sb, "用户问题：%s\n\n执行的 SQL：%s\n\n查询结果（列：%s）：\n", question, sqlText, strings.Join(columns, ", "))
	maxRows := len(rows)
	if maxRows > 20 {
		maxRows = 20 // 结论只看前 20 行，避免超长
	}
	for _, r := range rows[:maxRows] {
		sb.WriteString(strings.Join(r.Values, " | "))
		sb.WriteString("\n")
	}

	langLine := "用中文回答。"
	if strings.HasPrefix(lang, "en") {
		langLine = "Answer in English."
	}

	aCtx, cancel := context.WithTimeout(ctx, aiQueryCallTimeout)
	defer cancel()
	resp, err := client.CreateChatCompletion(aCtx, openai.ChatCompletionRequest{
		Model: ptrStrOr(provider.ModelName, "gpt-4o-mini"),
		Messages: []openai.ChatCompletionMessage{
			{
				Role: openai.ChatMessageRoleSystem,
				Content: "你是数据分析助手。根据用户问题、SQL 与查询结果，" +
					"给出简明的结论：直接回答问题本身，引用具体数字；" +
					"结果为空就明确说明没有匹配数据。" + langLine,
			},
			{Role: openai.ChatMessageRoleUser, Content: sb.String()},
		},
	})
	if err != nil {
		return "", 0, err
	}
	tokens := int64(resp.Usage.TotalTokens)
	if len(resp.Choices) == 0 {
		return "", tokens, fmt.Errorf("empty answer")
	}
	return strings.TrimSpace(resp.Choices[0].Message.Content), tokens, nil
}

// aiQueryExtractPattern 提取用：非锚定定位第一条 SELECT/WITH。
var aiQueryExtractPattern = regexp.MustCompile(`(?is)\b(SELECT|WITH)\b`)

// extractSQL 从模型输出中提取 SQL（剥围栏、截分号、剥行注释）。
func extractSQL(raw string) string {
	s := stripSQLFence(raw)
	loc := aiQueryExtractPattern.FindStringIndex(s)
	if loc == nil {
		return ""
	}
	s = s[loc[0]:]
	if idx := strings.Index(s, ";"); idx >= 0 {
		s = s[:idx]
	}
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		if idx := strings.Index(line, "--"); idx >= 0 {
			lines[i] = line[:idx]
		}
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

// stripSQLFence 剥掉模型可能输出的 Markdown 代码块围栏。
func stripSQLFence(s string) string {
	s = strings.TrimSpace(s)
	for _, fence := range []string{"```sql", "```SQL", "```"} {
		if strings.HasPrefix(s, fence) {
			s = strings.TrimPrefix(s, fence)
			if idx := strings.LastIndex(s, "```"); idx >= 0 {
				s = s[:idx]
			}
			break
		}
	}
	return strings.TrimSpace(s)
}

// structpbRowsToStringRows OLAP 通道的结构值行 → 问数响应的字符串行
// （时间/数字均已格式化为字符串，与 proto 契约一致）。
func structpbRowsToStringRows(rows []*ubaV1.SqlResultRow) []*aiV1.AiQueryRow {
	out := make([]*aiV1.AiQueryRow, 0, len(rows))
	for _, row := range rows {
		aiRow := &aiV1.AiQueryRow{Values: make([]string, 0, len(row.GetValues()))}
		for _, v := range row.GetValues() {
			aiRow.Values = append(aiRow.Values, stringifyStructValue(v))
		}
		out = append(out, aiRow)
	}
	return out
}

// stringifyStructValue structpb.Value → 展示字符串（nil → 空串，结构化值走 JSON）。
func stringifyStructValue(v *structpb.Value) string {
	if v == nil {
		return ""
	}
	switch nv := v.GetKind().(type) {
	case *structpb.Value_NullValue:
		return ""
	case *structpb.Value_StringValue:
		return nv.StringValue
	case *structpb.Value_NumberValue:
		return fmt.Sprintf("%v", nv.NumberValue)
	case *structpb.Value_BoolValue:
		if nv.BoolValue {
			return "true"
		}
		return "false"
	default:
		return fmt.Sprintf("%v", v.AsInterface())
	}
}

// correctTableName 表名纠正：依候选序列匹配白名单（前缀 / 复数 s / 组合）。
func correctTableName(table string, whitelist map[string]string) (string, bool) {
	if _, ok := whitelist[table]; ok {
		return table, true
	}
	for _, candidate := range []string{
		table + "s",
		"fact_" + table,
		strings.TrimSuffix(table, "s"),
		strings.TrimSuffix(table, "s") + "es",
	} {
		if _, ok := whitelist[candidate]; ok {
			return candidate, true
		}
	}
	return "", false
}

// dataSource 当前引擎名，写进数据访问审计日志（与 SqlQueryService 同一口径）。
func (s *AiQueryService) dataSource() string {
	if data.UseClickHouse && s.clickhouse != nil {
		return "clickhouse"
	}
	return "doris"
}

// writeAudit 问数执行落数据访问审计日志；审计失败不影响查询结果返回。
// AI 生成的 SQL 与人工自助 SQL 同审计水位——这是问数安全边界的留痕环节。
func (s *AiQueryService) writeAudit(ctx context.Context, query string, resp *ubaV1.ExecuteSqlQueryResponse, execErr error) {
	if s.auditRepo == nil {
		return
	}

	entry := &auditV1.DataAccessAuditLog{
		DataSource: trans.Ptr(s.dataSource()),
		TableName:  trans.Ptr("adhoc"),
		SqlText:    trans.Ptr(query),
		SqlDigest:  trans.Ptr(digestSQL(query)),
		Success:    trans.Ptr(execErr == nil),
	}
	if resp != nil {
		entry.AffectedRows = trans.Ptr(uint32(resp.GetRowCount()))
		entry.LatencyMs = trans.Ptr(uint32(resp.GetDurationMs()))
	}
	entry.UserId = trans.Ptr(s.operatorID(ctx))

	if err := s.auditRepo.Create(ctx, &auditV1.CreateDataAccessAuditLogRequest{Data: entry}); err != nil {
		s.log.Errorf("write data access audit log failed: %v", err)
	}
}

// aiQueryMaxDictEvents 提示词里事件字典的条数上限（防提示词膨胀）。
const aiQueryMaxDictEvents = 100

// buildEventSemanticSection 事件字典语义段（sys_event_schemas → 提示词）。
// 只有表结构时模型不知道业务事件名（如"下单"对应哪个 event_name 值），
// 这段把登记的事件语义喂给它；加载失败降级为空，不阻断问数。
func (s *AiQueryService) buildEventSemanticSection(ctx context.Context) string {
	if s.eventSchemaRepo == nil {
		return ""
	}
	resp, err := s.eventSchemaRepo.List(ctx, noPagingRequest())
	if err != nil {
		s.log.Warnf("load event schemas for ai query prompt failed: %v", err)
		return ""
	}
	items := resp.GetItems()
	if len(items) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n业务事件字典（event_name 的业务语义；用户提问里的业务词大概率对应这里的事件）：\n")
	for i, item := range items {
		if i >= aiQueryMaxDictEvents {
			fmt.Fprintf(&sb, "（其余 %d 个事件略）\n", len(items)-aiQueryMaxDictEvents)
			break
		}
		line := "- " + item.GetEventName()
		if dn := item.GetDisplayName(); dn != "" {
			line += "（" + dn + "）"
		}
		if cat := item.GetCategory(); cat != "" {
			line += " [" + cat + "]"
		}
		if desc := item.GetDescription(); desc != "" {
			line += "：" + desc
		}
		sb.WriteString(line + "\n")
	}
	return sb.String()
}
