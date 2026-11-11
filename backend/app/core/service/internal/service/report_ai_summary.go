package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/sashabaranov/go-openai"

	"go-wind-uba/app/core/service/internal/data"
	"go-wind-uba/app/core/service/internal/data/ent"
)

// 定时报表 AI 摘要：报表引擎跑完分析查询后，把确定性摘要与少量数据行
// 交给 LLM 生成一段自然语言结论，附在推送内容里（站内信/网络钩子共用）。
//
// 语义 best-effort：未配置启用的提供商 / 调用失败 → 静默跳过（只记日志），
// 报表本身照常投递；摘要不落库，只在推送文本里。消耗记入用量流水
// （provider 归属租户计账，userId=0 平台自用）。

const (
	// reportAiSummaryTimeout 单次摘要生成的兜底时长。
	reportAiSummaryTimeout = 60 * time.Second

	// reportAiSummaryMaxRows 喂给模型的 CSV 行数上限（结论只看头部样本）。
	reportAiSummaryMaxRows = 20

	// reportAiSummaryMaxChars CSV 喂给模型的总字符上限。
	reportAiSummaryMaxChars = 4000
)

// appendAiReportSummary 生成 AI 摘要并追加到确定性摘要之后。
// 未配置提供商或生成失败时原样返回 summary（不阻断报表投递）。
func (s *ReportEngine) appendAiReportSummary(ctx context.Context, sub *ent.ReportSubscription, summary, csv string) string {
	if s.aiProviderRepo == nil {
		return summary
	}

	provider, err := s.aiProviderRepo.GetEnabledDefault(ctx)
	if err != nil {
		s.log.Debugf("report ai summary skipped: no enabled provider")
		return summary
	}
	client, err := newOpenAIClientForProvider(provider)
	if err != nil {
		s.log.Debugf("report ai summary skipped: client unavailable: %v", err)
		return summary
	}

	prompt := buildReportAiPrompt(derefStr(sub.Name), derefStr(sub.Model), summary, csv)

	aCtx, cancel := context.WithTimeout(ctx, reportAiSummaryTimeout)
	defer cancel()

	resp, err := client.CreateChatCompletion(aCtx, openai.ChatCompletionRequest{
		Model: ptrStrOr(provider.ModelName, "gpt-4o-mini"),
		Messages: []openai.ChatCompletionMessage{
			{
				Role: openai.ChatMessageRoleSystem,
				Content: "你是用户行为分析的报表助手。根据报表的确定性摘要与数据样本，" +
					"写一段不超过 150 字的中文结论：指出关键数字、趋势方向（升/降/平稳）" +
					"和值得注意的异常；不要复述全部数据，不要编造数据里没有的数字。",
			},
			{Role: openai.ChatMessageRoleUser, Content: prompt},
		},
	})
	if err != nil {
		s.log.Warnf("report ai summary generate failed: sub=%d: %v", sub.ID, err)
		return summary
	}
	if len(resp.Choices) == 0 {
		return summary
	}

	// 记账（失败不阻断）
	if s.aiUsageLogRepo != nil {
		_ = s.aiUsageLogRepo.Create(ctx, &data.UsageRecord{
			ProviderID:  provider.ID,
			TenantID:    derefInt64Local(sub.TenantID),
			ModelName:   ptrStrOr(provider.ModelName, ""),
			TotalTokens: int64(resp.Usage.TotalTokens),
		})
	}

	text := strings.TrimSpace(resp.Choices[0].Message.Content)
	if text == "" {
		return summary
	}
	return summary + "\n\n—— AI 摘要 ——\n" + text
}

// buildReportAiPrompt 组装摘要生成的用户提示词（确定性摘要 + 截断后的 CSV 样本）。
func buildReportAiPrompt(reportName, model, summary, csv string) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "报表：%s\n分析模型：%s\n\n确定性摘要：\n%s\n\n数据样本（CSV，最多 %d 行）：\n",
		reportName, model, summary, reportAiSummaryMaxRows)

	csvText := strings.TrimSpace(csv)
	if runes := []rune(csvText); len(runes) > reportAiSummaryMaxChars {
		csvText = string(runes[:reportAiSummaryMaxChars])
	}
	lines := strings.Split(csvText, "\n")
	if len(lines) > reportAiSummaryMaxRows+1 { // +1 表头
		csvText = strings.Join(lines[:reportAiSummaryMaxRows+1], "\n") + "\n..."
	}
	sb.WriteString(csvText)
	sb.WriteString("\n\n请给出简明结论。")
	return sb.String()
}
