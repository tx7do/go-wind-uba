package task

// AiDocReindexTaskType 知识库切片重索引任务的 asynq 任务类型。
// 场景：管理员更换知识库 embedding 模型/provider 后，对既有切片全量重算向量。
const AiDocReindexTaskType = "ai.doc.reindex"

// AiDocReindexPayload 知识库切片重索引任务 payload。
// BaseID=0 表示对全部知识库重索引；>0 只处理指定库。
type AiDocReindexPayload struct {
	BaseID int64 `json:"base_id"`
}
