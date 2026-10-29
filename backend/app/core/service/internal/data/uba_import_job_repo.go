package data

import (
	"context"
	"time"

	"entgo.io/ent/dialect/sql"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/go-utils/copierutil"
	"github.com/tx7do/go-utils/mapper"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	entCrud "github.com/tx7do/go-crud/entgo"

	"go-wind-uba/app/core/service/internal/data/ent"
	"go-wind-uba/app/core/service/internal/data/ent/importjob"
	"go-wind-uba/app/core/service/internal/data/ent/predicate"

	ubaV1 "go-wind-uba/api/gen/go/uba/service/v1"
)

// ImportJobRepo 历史数据导入任务数据仓库
type ImportJobRepo struct {
	entClient *entCrud.EntClient[*ent.Client]
	log       *log.Helper

	statusConverter *mapper.EnumTypeConverter[ubaV1.ImportJobStatus, string]

	mapper *mapper.CopierMapper[ubaV1.ImportJob, ent.ImportJob]

	repository *entCrud.Repository[
		ent.ImportJobQuery, ent.ImportJobSelect,
		ent.ImportJobCreate, ent.ImportJobCreateBulk,
		ent.ImportJobUpdate, ent.ImportJobUpdateOne,
		ent.ImportJobDelete,
		predicate.ImportJob,
		ubaV1.ImportJob, ent.ImportJob,
	]
}

func NewImportJobRepo(ctx *bootstrap.Context, entClient *entCrud.EntClient[*ent.Client]) *ImportJobRepo {
	repo := &ImportJobRepo{
		log:             ctx.NewLoggerHelper("import-job/repo/core-service"),
		entClient:       entClient,
		mapper:          mapper.NewCopierMapper[ubaV1.ImportJob, ent.ImportJob](),
		statusConverter: mapper.NewEnumTypeConverter[ubaV1.ImportJobStatus, string](ubaV1.ImportJobStatus_name, ubaV1.ImportJobStatus_value),
	}

	repo.init()

	return repo
}

func (r *ImportJobRepo) init() {
	r.repository = entCrud.NewRepository[
		ent.ImportJobQuery, ent.ImportJobSelect,
		ent.ImportJobCreate, ent.ImportJobCreateBulk,
		ent.ImportJobUpdate, ent.ImportJobUpdateOne,
		ent.ImportJobDelete,
		predicate.ImportJob,
		ubaV1.ImportJob, ent.ImportJob,
	](r.mapper)

	r.mapper.AppendConverters(copierutil.NewTimeStringConverterPair())
	r.mapper.AppendConverters(copierutil.NewTimeTimestamppbConverterPair())
	r.mapper.AppendConverters(r.statusConverter.NewConverterPair())
}

// ToDTO 实体转 DTO（自动含 status 枚举转换）。
func (r *ImportJobRepo) ToDTO(entity *ent.ImportJob) *ubaV1.ImportJob {
	dto := r.mapper.ToDTO(entity)
	dto.Status = statusToDTO(entity.Status)
	return dto
}

func (r *ImportJobRepo) Count(ctx context.Context, req *paginationV1.PagingRequest) (*ubaV1.CountImportJobResponse, error) {
	builder := r.entClient.Client().ImportJob.Query()

	whereSelectors, _, err := r.repository.BuildListSelectorWithPaging(builder, req)
	if len(whereSelectors) != 0 {
		builder.Modify(whereSelectors...)
	}

	count, err := builder.Count(ctx)
	if err != nil {
		r.log.Errorf("query import job count failed: %s", err.Error())
		return nil, ubaV1.ErrorImportInternalError("query import job count failed")
	}

	return &ubaV1.CountImportJobResponse{Count: uint64(count)}, nil
}

func (r *ImportJobRepo) List(ctx context.Context, req *paginationV1.PagingRequest) (*ubaV1.ListImportJobResponse, error) {
	if req == nil {
		return nil, ubaV1.ErrorImportBadRequest("invalid parameter")
	}

	builder := r.entClient.Client().ImportJob.Query()

	ret, err := r.repository.ListWithPaging(ctx, builder, builder.Clone(), req)
	if err != nil {
		return nil, err
	}
	if ret == nil {
		return &ubaV1.ListImportJobResponse{Total: 0, Items: nil}, nil
	}

	return &ubaV1.ListImportJobResponse{Total: ret.Total, Items: ret.Items}, nil
}

func (r *ImportJobRepo) IsExist(ctx context.Context, id uint32) (bool, error) {
	exist, err := r.entClient.Client().ImportJob.Query().
		Where(importjob.IDEQ(id)).
		Exist(ctx)
	if err != nil {
		r.log.Errorf("query exist failed: %s", err.Error())
		return false, ubaV1.ErrorImportInternalError("query exist failed")
	}
	return exist, nil
}

func (r *ImportJobRepo) Get(ctx context.Context, req *ubaV1.GetImportJobRequest) (*ubaV1.ImportJob, error) {
	if req == nil {
		return nil, ubaV1.ErrorImportBadRequest("invalid parameter")
	}

	entity, err := r.entClient.Client().ImportJob.Query().
		Where(importjob.IDEQ(req.GetId())).
		Only(ctx)
	if err != nil {
		return nil, ubaV1.ErrorImportJobNotFound("import job [%d] not found", req.GetId())
	}

	dto := r.ToDTO(entity)
	dto.EventsJson = nil
	return dto, nil
}

// GetEntityWithEvents 读取含 events_json 的原始实体（导入执行器用）。
func (r *ImportJobRepo) GetEntityWithEvents(ctx context.Context, id uint32) (*ent.ImportJob, error) {
	return r.entClient.Client().ImportJob.Query().
		Where(importjob.IDEQ(id)).
		Only(ctx)
}

func (r *ImportJobRepo) Create(ctx context.Context, name string, appID *string, eventsJSON string, totalCount uint32, tenantID, createdBy uint32) (*ent.ImportJob, error) {
	now := time.Now()
	return r.entClient.Client().ImportJob.Create().
		SetNillableTenantID(&tenantID).
		SetNillableAppID(appID).
		SetNillableName(&name).
		SetEventsJSON(eventsJSON).
		SetTotalCount(totalCount).
		SetStatus("PENDING").
		SetNillableCreatedBy(&createdBy).
		SetCreatedAt(now).
		SetUpdatedAt(now).
		Save(ctx)
}

// UpdateProgress 回写进度与结论。
func (r *ImportJobRepo) UpdateProgress(ctx context.Context, id uint32, imported, failed uint32, status string, errMsgs []string, startedAt, finishedAt time.Time) error {
	builder := r.entClient.Client().ImportJob.UpdateOneID(id).
		SetImportedCount(imported).
		SetFailedCount(failed).
		SetStatus(status).
		SetStartedAt(startedAt).
		SetFinishedAt(finishedAt).
		SetDurationMs(uint32(finishedAt.Sub(startedAt).Milliseconds())).
		SetUpdatedAt(time.Now())

	if len(errMsgs) > 0 {
		builder.SetErrors(errMsgs)
	}

	if err := builder.Exec(ctx); err != nil {
		r.log.Errorf("update import job progress failed: %s", err.Error())
		return ubaV1.ErrorImportInternalError("update import job progress failed")
	}
	return nil
}

// UpdateStatus 仅改状态（取消等）。
func (r *ImportJobRepo) UpdateStatus(ctx context.Context, id uint32, status string) error {
	err := r.entClient.Client().ImportJob.UpdateOneID(id).
		SetStatus(status).
		SetUpdatedAt(time.Now()).
		Exec(ctx)
	if err != nil {
		r.log.Errorf("update import job status failed: %s", err.Error())
		return ubaV1.ErrorImportInternalError("update import job status failed")
	}
	return nil
}

// ClearEventsJSON 导入完成后清空 events_json（释放存储）。
func (r *ImportJobRepo) ClearEventsJSON(ctx context.Context, id uint32) {
	_ = r.entClient.Client().ImportJob.UpdateOneID(id).
		ClearEventsJSON().
		Exec(ctx)
}

func (r *ImportJobRepo) Delete(ctx context.Context, req *ubaV1.DeleteImportJobRequest) error {
	if req == nil {
		return ubaV1.ErrorImportBadRequest("invalid parameter")
	}

	builder := r.entClient.Client().ImportJob.Delete()

	_, err := r.repository.Delete(ctx, builder, func(s *sql.Selector) {
		s.Where(sql.EQ(importjob.FieldID, req.GetId()))
	})
	if err != nil {
		r.log.Errorf("delete import job failed: %s", err.Error())
		return ubaV1.ErrorImportInternalError("delete import job failed")
	}

	return nil
}

// statusToDTO 存储字符串转枚举。
func statusToDTO(s string) *ubaV1.ImportJobStatus {
	if v, ok := ubaV1.ImportJobStatus_value[s]; ok {
		st := ubaV1.ImportJobStatus(v)
		return &st
	}
	st := ubaV1.ImportJobStatus_IMPORT_JOB_STATUS_PENDING
	return &st
}
