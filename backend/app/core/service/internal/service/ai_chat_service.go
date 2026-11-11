package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/sashabaranov/go-openai"
	entCrud "github.com/tx7do/go-crud/entgo"
	"github.com/tx7do/go-crud/viewer"
	"github.com/tx7do/go-utils/trans"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"google.golang.org/grpc"

	aiV1 "go-wind-uba/api/gen/go/ai/service/v1"
	"go-wind-uba/app/core/service/internal/data"
	clickhouseData "go-wind-uba/app/core/service/internal/data/clickhouse"
	dorisData "go-wind-uba/app/core/service/internal/data/doris"
	"go-wind-uba/app/core/service/internal/data/ent"
	"go-wind-uba/app/core/service/internal/data/ent/aimessage"
	entmw "go-wind-uba/pkg/middleware/ent"
)

const (
	// chatContextMaxMessages LLM 上下文最多携带的历史消息条数。
	chatContextMaxMessages = 20

	// conversationTitleMaxRunes 新会话默认标题截断长度（按字符，中文友好）。
	conversationTitleMaxRunes = 30

	// chatCallTimeout 单轮流式对话的兜底时长：覆盖整轮（历史拼装 + 模型生成 + 落库）。
	chatCallTimeout = 5 * time.Minute
)

// AiChatService AI 对话：core 侧 server-streaming——逐帧吐 chunk/tool，
// done 帧携带完整回复；SSE 推送由 admin BFF 消费该流后完成（见 i_ai.proto）。
type AiChatService struct {
	aiV1.UnimplementedAiChatServiceServer
	log              *log.Helper
	conversationRepo *data.AiConversationRepo
	messageRepo      *data.AiMessageRepo
	providerRepo     *data.AiProviderRepo
	usageLogRepo     *data.AiUsageLogRepo
	knowledgeRepo    *data.AiKnowledgeRepo
	entClient        *entCrud.EntClient[*ent.Client]

	// UBA 分析工具的数据源（免写 SQL 回答活跃度/热门事件/事件语义问题）
	dorisAnalyticsRepo *dorisData.AnalyticsRepo
	ckAnalyticsRepo    *clickhouseData.AnalyticsRepo
	eventSchemaRepo    *data.EventSchemaRepo

	// 租户月度 token 配额（nil 安全；超限 429）
	quotaRepo *data.AiTenantQuotaRepo
}

func NewAiChatService(
	ctx *bootstrap.Context,
	conversationRepo *data.AiConversationRepo,
	messageRepo *data.AiMessageRepo,
	providerRepo *data.AiProviderRepo,
	usageLogRepo *data.AiUsageLogRepo,
	knowledgeRepo *data.AiKnowledgeRepo,
	entClient *entCrud.EntClient[*ent.Client],
	dorisAnalyticsRepo *dorisData.AnalyticsRepo,
	ckAnalyticsRepo *clickhouseData.AnalyticsRepo,
	eventSchemaRepo *data.EventSchemaRepo,
	quotaRepo *data.AiTenantQuotaRepo,
) *AiChatService {
	return &AiChatService{
		log:                ctx.NewLoggerHelper("ai_chat/service/core-service"),
		conversationRepo:   conversationRepo,
		messageRepo:        messageRepo,
		providerRepo:       providerRepo,
		usageLogRepo:       usageLogRepo,
		knowledgeRepo:      knowledgeRepo,
		entClient:          entClient,
		dorisAnalyticsRepo: dorisAnalyticsRepo,
		ckAnalyticsRepo:    ckAnalyticsRepo,
		eventSchemaRepo:    eventSchemaRepo,
		quotaRepo:          quotaRepo,
	}
}

// Chat 一轮对话：落 user 消息 → 流式调模型（chunk/tool 帧实时吐给 BFF）→
// 落 assistant 消息与用量流水 → done 帧返回完整回复。
func (s *AiChatService) Chat(req *aiV1.ChatRequest, stream grpc.ServerStreamingServer[aiV1.ChatStreamFrame]) error {
	ctx := stream.Context()
	// unary 的 ent viewer 中间件不覆盖流式 RPC，这里自行注入 viewer
	// （BFF 转发的 operator 元数据 → UserViewer；异常回落 SystemViewer）。
	ctx = entmw.InjectViewerForStream(ctx)

	// BFF 注入的 operator 字段优先（流式 RPC 的元数据转发不可靠，见 proto 注释）；
	// 缺失时回落 viewer 上下文。
	var userId, tenantId int64
	if uid := req.GetOperatorUserId(); uid > 0 {
		userId = uid
		tenantId = req.GetOperatorTenantId()
	} else {
		if vc, exist := viewer.FromContext(ctx); exist && vc != nil {
			tenantId = int64(vc.TenantID())
		}
		if uid, hasUser := viewerUserIDFromContext(ctx); hasUser {
			userId = uid
		}
	}

	content := req.GetContent()
	if content == "" {
		return aiV1.ErrorBadRequest("content is required")
	}

	// 1. 解析提供商（显式指定或默认）
	provider, err := s.resolveProvider(ctx, req.GetProviderId())
	if err != nil {
		return err
	}

	// 2. 会话归属：复用或新建
	conversation, err := s.resolveConversation(ctx, userId, tenantId, req.GetConversationId(), provider, content)
	if err != nil {
		return err
	}

	// 2.5 配额护栏：租户月度 token 用尽即拒绝（平台自用不限量）
	if err = s.quotaRepo.EnsureWithinQuota(ctx, s.usageLogRepo, tenantId); err != nil {
		return err
	}

	// 3. 落 user 消息
	userEntity, err := s.messageRepo.Create(ctx, &aiV1.AiMessage{
		ConversationId: &conversation.ID,
		Role:           aiV1.AiRole_USER.Enum(),
		Content:        &content,
		UserId:         &userId,
		TenantId:       trans.Ptr(tenantId),
		CreatedBy:      &userId,
	})
	if err != nil {
		return err
	}

	// 4. LLM 上下文：system（知识库检索注入 RAG + 提供商 system prompt）+ 历史 + 本轮。
	messages := make([]openai.ChatCompletionMessage, 0, chatContextMaxMessages+2)
	systemParts := make([]string, 0, 2)
	// ragHits 本轮检索命中（RAG 引用回传用；检索失败/未用知识库为 nil）
	var ragHits []*data.ChunkHit
	if req.GetKnowledgeBaseId() > 0 && s.knowledgeRepo != nil {
		hits, searchErr := searchKnowledgeBase(ctx, s.knowledgeRepo, s.providerRepo, s.usageLogRepo, s.log, userId, tenantId, req.GetKnowledgeBaseId(), content, ragTopK)
		if searchErr != nil {
			// 检索失败不阻断对话：降级为无知识库上下文
			s.log.Errorf("knowledge search failed, degrade to plain chat: base=%d: %v", req.GetKnowledgeBaseId(), searchErr)
		} else if len(hits) > 0 {
			ragHits = hits
			var kb strings.Builder
			kb.WriteString("请优先依据以下知识库片段回答用户问题，片段无关时可忽略：\n")
			for i, hit := range hits {
				fmt.Fprintf(&kb, "[片段%d] %s\n", i+1, hit.Content)
			}
			systemParts = append(systemParts, kb.String())
		}
	}
	if provider.SystemPrompt != nil && *provider.SystemPrompt != "" {
		systemParts = append(systemParts, *provider.SystemPrompt)
	}
	if len(systemParts) > 0 {
		messages = append(messages, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleSystem, Content: strings.Join(systemParts, "\n\n")})
	}

	// 多取一条：第 3 步刚落库的本轮消息必是表内最新一行，会被这次历史查询带出，
	// 循环里按 ID 剔除后恰好剩 N-1 条历史；本轮消息在下方显式 append，不剔就会重复计两次。
	history, err := s.messageRepo.ListRecentByConversation(ctx, conversation.ID, chatContextMaxMessages)
	if err != nil {
		return err
	}
	for _, m := range history {
		if m.Role == nil || m.ID == userEntity.ID {
			continue
		}
		// ent 枚举是大写（USER/ASSISTANT/SYSTEM），OpenAI 协议要求小写 role——
		// 真实厂商（DeepSeek 等）会 422 拒绝大写值，必须映射。
		messages = append(messages, openai.ChatCompletionMessage{
			Role:    aiRoleToOpenAI(*m.Role),
			Content: ptrStr(m.Content),
		})
	}
	messages = append(messages, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: content})

	// 5. 客户端 + Function Call 多轮循环：模型请求工具 → 本地执行 → 结果回传，
	//    直到给出文本答案（最终答案轮才经 chunk 帧吐给前端）。
	client, err := newOpenAIClientForProvider(provider)
	if err != nil {
		return err
	}

	streamCtx, cancel := context.WithTimeout(ctx, chatCallTimeout)
	defer cancel()

	var seq uint32
	var usage openai.Usage
	start := time.Now()

	runner := &aiToolLoop{
		client: client,
		model:  ptrStrOr(provider.ModelName, "gpt-4o-mini"),
		tools:  append(aiBuiltinToolDefs(), ubaToolDefs()...),
		exec: func(tctx context.Context, name, args string) (string, error) {
			if out, handled, err := s.execUBATool(tctx, name, args); handled {
				return out, err
			}
			return execAiTool(tctx, name, args)
		},
		maxRounds: aiToolMaxRounds,
		onDelta: func(delta string) {
			_ = stream.Send(&aiV1.ChatStreamFrame{Frame: &aiV1.ChatStreamFrame_Chunk{
				Chunk: &aiV1.ChatChunkEvent{
					ConversationId: conversation.ID,
					Seq:            seq,
					Delta:          delta,
				},
			}})
			seq++
		},
		onToolCall: func(name, arguments, result string) {
			_ = stream.Send(&aiV1.ChatStreamFrame{Frame: &aiV1.ChatStreamFrame_Tool{
				Tool: &aiV1.ChatToolEvent{
					ConversationId: conversation.ID,
					Name:           name,
					Arguments:      arguments,
					Result:         result,
				},
			}})
		},
	}
	fullText, usage, toolErr := runner.run(streamCtx, messages)
	if toolErr != nil {
		s.log.Errorf("ai chat stream failed: conversation=%d: %v", conversation.ID, toolErr)
		return aiV1.ErrorInternalServerError("ai chat failed: %v", toolErr)
	}
	durationMs := int64(time.Since(start).Milliseconds())

	// 6. 落 assistant 消息与用量流水
	assistantContent := fullText
	modelName := ptrStrOr(provider.ModelName, "")
	promptTokens := int64(usage.PromptTokens)
	completionTokens := int64(usage.CompletionTokens)
	totalTokens := int64(usage.TotalTokens)

	assistantEntity, err := s.messageRepo.Create(ctx, &aiV1.AiMessage{
		ConversationId:   &conversation.ID,
		Role:             aiV1.AiRole_ASSISTANT.Enum(),
		Content:          &assistantContent,
		ModelName:        &modelName,
		PromptTokens:     trans.Ptr(uint32(promptTokens)),
		CompletionTokens: trans.Ptr(uint32(completionTokens)),
		DurationMs:       trans.Ptr(uint32(durationMs)),
		UserId:           &userId,
		TenantId:         trans.Ptr(tenantId),
		CreatedBy:        &userId,
	})
	if err != nil {
		return err
	}

	if err = s.usageLogRepo.Create(ctx, &data.UsageRecord{
		ProviderID:       provider.ID,
		ConversationID:   conversation.ID,
		UserID:           userId,
		TenantID:         tenantId,
		ModelName:        modelName,
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		TotalTokens:      totalTokens,
		DurationMs:       durationMs,
	}); err != nil {
		// 记账失败不阻断对话返回：流水缺失只影响配额统计精度，记日志跟进。
		s.log.Errorf("record ai usage log failed: conversation=%d: %v", conversation.ID, err)
	}

	if err = s.conversationRepo.TouchLastMessage(ctx, conversation.ID); err != nil {
		s.log.Errorf("touch conversation last message failed: conversation=%d: %v", conversation.ID, err)
	}

	conversationDto := s.conversationRepo.ToDTO(conversation)
	messageDto := s.messageRepo.ToDTO(assistantEntity)
	if conversationDto == nil || messageDto == nil {
		return aiV1.ErrorInternalServerError("map ai chat result failed")
	}

	// RAG 引用回传（文档名就地回填；取名失败不阻断对话，退化为 #docId）
	return stream.Send(&aiV1.ChatStreamFrame{Frame: &aiV1.ChatStreamFrame_Done{
		Done: &aiV1.ChatResponse{
			Conversation: conversationDto,
			Message:      messageDto,
			Citations:    s.buildCitations(ctx, ragHits),
		},
	}})
}

// chatCitationSnippetRunes 引用片段摘要的截断长度（按字符）。
const chatCitationSnippetRunes = 80

// buildCitations 把检索命中转成引用条目（按相似度已在 SQL 排序）。
// 文档名批量回填；查询失败时退化为 "#docId" 占位，不阻断对话。
func (s *AiChatService) buildCitations(ctx context.Context, hits []*data.ChunkHit) []*aiV1.ChatCitation {
	if len(hits) == 0 {
		return nil
	}

	docIds := make([]int64, 0, len(hits))
	for _, hit := range hits {
		docIds = append(docIds, hit.DocID)
	}
	docNames := make(map[int64]string, len(docIds))
	if docs, err := s.knowledgeRepo.ListDocsByIDs(ctx, docIds); err != nil {
		s.log.Warnf("load citation doc names failed: %v", err)
	} else {
		for _, doc := range docs {
			name := doc.Name
			if name == nil || *name == "" {
				name = trans.Ptr(fmt.Sprintf("#%d", doc.ID))
			}
			docNames[doc.ID] = *name
		}
	}

	citations := make([]*aiV1.ChatCitation, 0, len(hits))
	for _, hit := range hits {
		citations = append(citations, &aiV1.ChatCitation{
			DocName:    docNames[hit.DocID],
			ChunkIndex: uint32(hit.ChunkIndex),
			Snippet:    truncateRunes(hit.Content, chatCitationSnippetRunes),
			Score:      hit.Score,
		})
	}
	return citations
}

// resolveProvider 解析本次对话使用的提供商。
func (s *AiChatService) resolveProvider(ctx context.Context, providerId int64) (*ent.AiProvider, error) {
	if providerId > 0 {
		entity, err := s.providerRepo.GetEntityByID(ctx, providerId)
		if err != nil {
			return nil, err
		}
		if entity.IsEnabled == nil || !*entity.IsEnabled {
			return nil, aiV1.ErrorBadRequest("ai provider is disabled")
		}
		return entity, nil
	}
	return s.providerRepo.GetEnabledDefault(ctx)
}

// resolveConversation 会话复用或新建（新建时标题取首条消息摘要）。
func (s *AiChatService) resolveConversation(ctx context.Context, userId, tenantId, conversationId int64, provider *ent.AiProvider, content string) (*ent.AiConversation, error) {
	if conversationId > 0 {
		entity, err := s.conversationRepo.GetEntityByID(ctx, conversationId)
		if err != nil {
			return nil, err
		}
		if entity.UserID == nil || *entity.UserID != userId {
			return nil, aiV1.ErrorNotFound("ai conversation not found")
		}
		return entity, nil
	}

	title := truncateRunes(content, conversationTitleMaxRunes)
	created, err := s.conversationRepo.Create(ctx, &aiV1.AiConversation{
		Title:      &title,
		ProviderId: &provider.ID,
		UserId:     &userId,
		TenantId:   trans.Ptr(tenantId),
		CreatedBy:  &userId,
	})
	if err != nil {
		return nil, err
	}
	return created, nil
}

// aiRoleToOpenAI ent 大写枚举 → OpenAI 小写 role。
func aiRoleToOpenAI(role aimessage.Role) string {
	switch role {
	case aimessage.RoleUSER:
		return openai.ChatMessageRoleUser
	case aimessage.RoleASSISTANT:
		return openai.ChatMessageRoleAssistant
	case aimessage.RoleSYSTEM:
		return openai.ChatMessageRoleSystem
	default:
		return openai.ChatMessageRoleUser
	}
}

// truncateRunes 按字符截断（中文友好）。
func truncateRunes(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max])
}

func ptrStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
