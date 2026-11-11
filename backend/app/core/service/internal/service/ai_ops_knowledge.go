package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/tx7do/go-utils/trans"

	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"

	aiV1 "go-wind-uba/api/gen/go/ai/service/v1"
)

// 平台运维知识同步：把内置的接入指南与事件字典灌入知识库，供 AI 对话
// RAG 引用——让「怎么接入 SDK」「xxx 事件什么含义」这类运维/使用问题
// 在对话里直接得到基于平台事实的回答，而非模型自由发挥。
//
// 幂等语义：知识库按固定名「复用或新建」，同名文档「先删后建」全量重建
// 向量；事件字典每次从 sys_event_schemas 现取（随埋点演进保持最新）。

// opsKnowledgeBaseName 运维知识库的固定名称（复用依据）。
const opsKnowledgeBaseName = "平台运维知识库"

// opsGuideDocName 接入指南文档名（幂等删除重建的依据）。
const opsGuideDocName = "平台接入指南"

// opsEventDictDocName 事件字典文档名。
const opsEventDictDocName = "事件字典"

// opsEmbeddingModel 未指定时创建运维知识库使用的 embedding 模型。
// 须与 data.AiEmbeddingDimensions（1536）配套，换模型须全量重索引。
const opsEmbeddingModel = "text-embedding-3-small"

// SyncOpsKnowledge 同步平台运维知识。前置：已配置启用的 AI 提供商
// （向量化要走它的 /v1/embeddings 端点）。
func (s *AiKnowledgeService) SyncOpsKnowledge(ctx context.Context, req *aiV1.SyncOpsKnowledgeRequest) (*aiV1.SyncOpsKnowledgeResponse, error) {
	userId, tenantId := s.operator(ctx)

	provider, err := s.providerRepo.GetEnabledDefault(ctx)
	if err != nil {
		return nil, aiV1.ErrorBadRequest("sync ops knowledge requires an enabled ai provider: %v", err)
	}

	// 知识库复用或新建（viewer 范围内：平台用户建平台库，租户用户建本租户库）
	base, err := s.repo.FindByName(ctx, opsKnowledgeBaseName)
	if err != nil {
		return nil, err
	}
	if base == nil {
		base, err = s.repo.Create(ctx, &aiV1.AiKnowledgeBase{
			Name:           trans.Ptr(opsKnowledgeBaseName),
			Description:    trans.Ptr("AI 对话 RAG 引用的平台内置知识（接入指南/事件字典），由「同步平台运维知识」维护"),
			ProviderId:     &provider.ID,
			EmbeddingModel: trans.Ptr(opsEmbeddingModel),
			UserId:         &userId,
			TenantId:       trans.Ptr(tenantId),
		})
		if err != nil {
			return nil, err
		}
	}

	// 内置文档清单：接入指南（代码内维护）+ 事件字典（现取 sys_event_schemas）
	eventDict, err := s.buildEventDictionary(ctx)
	if err != nil {
		return nil, err
	}
	docs := []struct {
		name    string
		content string
	}{
		{opsGuideDocName, opsGuideContent()},
		{opsEventDictDocName, eventDict},
	}

	totalChunks := int64(0)
	for _, doc := range docs {
		// 幂等：同名文档先删后建（删除级联清掉旧切片，向量随重建刷新）
		existing, findErr := s.repo.FindDocByName(ctx, base.ID, doc.name)
		if findErr != nil {
			return nil, findErr
		}
		if existing != nil {
			if delErr := s.repo.DeleteDoc(ctx, existing.ID); delErr != nil {
				return nil, delErr
			}
		}

		ingested, ingestErr := s.ingestDoc(ctx, userId, tenantId, base, doc.name, doc.content)
		if ingestErr != nil {
			return nil, ingestErr
		}
		totalChunks += int64(ingested.GetChunkCount())
	}

	s.log.Infof("sync ops knowledge done: base=%d docs=%d chunks=%d", base.ID, len(docs), totalChunks)
	return &aiV1.SyncOpsKnowledgeResponse{
		BaseId:     base.ID,
		DocCount:   uint32(len(docs)),
		ChunkCount: uint32(totalChunks),
	}, nil
}

// buildEventDictionary 从 sys_event_schemas 现取事件定义，生成给 RAG 用的
// Markdown 字典。无事件定义时返回占位说明（同步不因此失败）。
func (s *AiKnowledgeService) buildEventDictionary(ctx context.Context) (string, error) {
	resp, err := s.eventSchemaRepo.List(ctx, noPagingRequest())
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	sb.WriteString("# 事件字典\n\n")
	sb.WriteString("本平台行为事件的定义清单（源自事件接入配置，随埋点演进自动更新）。")
	sb.WriteString("每个事件给出名称、显示名、分类、描述与属性。回答「xxx 事件是什么/怎么接入」类问题时以此为准。\n\n")

	items := resp.GetItems()
	if len(items) == 0 {
		sb.WriteString("（当前尚未定义任何事件；接入 SDK 上报后，在「数据配置 → 事件接入」里登记事件定义，再回到本页同步。）\n")
		return sb.String(), nil
	}

	fmt.Fprintf(&sb, "共 %d 个事件：\n\n", len(items))
	for _, item := range items {
		fmt.Fprintf(&sb, "## %s", item.GetEventName())
		if dn := item.GetDisplayName(); dn != "" {
			fmt.Fprintf(&sb, "（%s）", dn)
		}
		sb.WriteString("\n")
		if cat := item.GetCategory(); cat != "" {
			fmt.Fprintf(&sb, "- 分类：%s\n", cat)
		}
		if desc := item.GetDescription(); desc != "" {
			fmt.Fprintf(&sb, "- 描述：%s\n", desc)
		}
		if props := item.GetProperties(); len(props) > 0 {
			sb.WriteString("- 属性：\n")
			for _, p := range props {
				required := ""
				if p.GetRequired() {
					required = "，必填"
				}
				fmt.Fprintf(&sb, "  - %s（%s%s）", p.GetName(), p.GetType(), required)
				if dn := p.GetDisplayName(); dn != "" {
					fmt.Fprintf(&sb, " %s", dn)
				}
				if desc := p.GetDescription(); desc != "" {
					fmt.Fprintf(&sb, "：%s", desc)
				}
				sb.WriteString("\n")
			}
		}
		sb.WriteString("\n")
	}
	return sb.String(), nil
}

// opsGuideContent 平台接入指南（内置文档正文，随代码版本演进）。
// 只写平台事实与入口指引；逐字段细节以 docs/ 与界面文案为准。
func opsGuideContent() string {
	var sb strings.Builder
	sb.WriteString("# 平台接入指南\n\n")
	sb.WriteString("GoWind UBA 是用户行为分析平台：终端（Web/App/小程序）接入埋点 SDK 上报行为事件，")
	sb.WriteString("事件经采集服务入仓后，即可在分析、分群、看板、智能问数等模块使用。\n\n")

	sb.WriteString("## 接入步骤\n\n")
	sb.WriteString("1. 在「数据配置 → 应用管理」创建应用，获得 AppID / AppKey / AppSecret（采集鉴权凭证）。\n")
	sb.WriteString("2. 在终端接入对应 SDK 并初始化：Web 端使用 @wind/uba 浏览器埋点 SDK（npm 包，源码见仓库 frontend/sdk/web/uba，集成说明见 docs/ 与 README）；服务端/游戏可用 C# SDK（仓库 sdk/csharp）。\n")
	sb.WriteString("3. SDK 初始化时上报用户登录（account/user id）后，事件即可关联到人；未登录阶段按设备维度匿名统计。\n")
	sb.WriteString("4. 在「数据配置 → 事件接入」登记事件定义（事件名、属性、描述）——本平台的「事件字典」与对话问答均以此为准。\n\n")

	sb.WriteString("## 事件模型\n\n")
	sb.WriteString("一条事件 = 事件名 + 属性（properties）+ 数值指标（metrics）+ 通用维度（平台/OS/渠道/国家/页面等）。")
	sb.WriteString("通用维度由 SDK 自动采集；业务属性随事件上报。全量字段见「数据配置 → 自助 SQL」中 events_fact 表结构。\n\n")

	sb.WriteString("## 常用入口\n\n")
	sb.WriteString("- 分析模型：趋势/漏斗/留存/分组等 25 种，见「数据分析」「深度洞察」菜单。\n")
	sb.WriteString("- 智能问数：用自然语言查行为数据（只读 SQL，结果上限 100 行），见「AI 助手 → 智能问数」。\n")
	sb.WriteString("- AI 对话：本页面；引用知识库（RAG）时先在「知识库」上传或同步运维知识。\n")
	sb.WriteString("- 自助 SQL：对 OLAP 的受控只读查询通道，见「数据配置 → 自助 SQL」。\n\n")

	sb.WriteString("## 注意事项\n\n")
	sb.WriteString("- 事件定义登记后，回到「AI 助手 → 知识库」点「同步平台运维知识」，对话即可引用最新事件字典。\n")
	sb.WriteString("- 更换知识库的 embedding 模型后需全量重索引（知识库页按钮）。\n")
	return sb.String()
}

// noPagingRequest 全量拉取（不分页）的分页请求。
func noPagingRequest() *paginationV1.PagingRequest {
	return &paginationV1.PagingRequest{NoPaging: trans.Ptr(true)}
}
