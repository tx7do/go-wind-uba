package data

import (
	"context"
	"time"

	"entgo.io/ent/dialect/sql"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	entCrud "github.com/tx7do/go-crud/entgo"

	"go-wind-uba/app/core/service/internal/data/ent"
	"go-wind-uba/app/core/service/internal/data/ent/application"
	"go-wind-uba/app/core/service/internal/data/ent/predicate"

	"github.com/tx7do/go-utils/copierutil"
	"github.com/tx7do/go-utils/mapper"

	ubaV1 "go-wind-uba/api/gen/go/uba/service/v1"
)

// ApplicationRepo 应用数据仓库
type ApplicationRepo struct {
	entClient *entCrud.EntClient[*ent.Client]
	log       *log.Helper

	mapper *mapper.CopierMapper[ubaV1.Application, ent.Application]

	statusConverter *mapper.EnumTypeConverter[ubaV1.Application_Status, application.Status]
	typeConverter   *mapper.EnumTypeConverter[ubaV1.Platform, application.Type]

	repository *entCrud.Repository[
		ent.ApplicationQuery, ent.ApplicationSelect,
		ent.ApplicationCreate, ent.ApplicationCreateBulk,
		ent.ApplicationUpdate, ent.ApplicationUpdateOne,
		ent.ApplicationDelete,
		predicate.Application,
		ubaV1.Application, ent.Application,
	]
}

func NewApplicationRepo(ctx *bootstrap.Context, entClient *entCrud.EntClient[*ent.Client]) *ApplicationRepo {
	repo := &ApplicationRepo{
		log:       ctx.NewLoggerHelper("application/repo/core-service"),
		entClient: entClient,
		mapper:    mapper.NewCopierMapper[ubaV1.Application, ent.Application](),
		statusConverter: mapper.NewEnumTypeConverter[ubaV1.Application_Status, application.Status](
			ubaV1.Application_Status_name, ubaV1.Application_Status_value,
		),
		typeConverter: mapper.NewEnumTypeConverter[ubaV1.Platform, application.Type](
			ubaV1.Platform_name, ubaV1.Platform_value,
		),
	}

	repo.init()

	return repo
}

func (r *ApplicationRepo) init() {
	r.repository = entCrud.NewRepository[
		ent.ApplicationQuery, ent.ApplicationSelect,
		ent.ApplicationCreate, ent.ApplicationCreateBulk,
		ent.ApplicationUpdate, ent.ApplicationUpdateOne,
		ent.ApplicationDelete,
		predicate.Application,
		ubaV1.Application, ent.Application,
	](r.mapper)

	r.mapper.AppendConverters(copierutil.NewTimeStringConverterPair())
	r.mapper.AppendConverters(copierutil.NewTimeTimestamppbConverterPair())

	r.mapper.AppendConverters(r.statusConverter.NewConverterPair())
	r.mapper.AppendConverters(r.typeConverter.NewConverterPair())
}

func (r *ApplicationRepo) Count(ctx context.Context, req *paginationV1.PagingRequest) (*ubaV1.CountApplicationResponse, error) {
	builder := r.entClient.Client().Application.Query()

	whereSelectors, _, err := r.repository.BuildListSelectorWithPaging(builder, req)
	if len(whereSelectors) != 0 {
		builder.Modify(whereSelectors...)
	}

	count, err := builder.Count(ctx)
	if err != nil {
		r.log.Errorf("query application count failed: %s", err.Error())
		return nil, ubaV1.ErrorInternalServerError("query application count failed")
	}

	return &ubaV1.CountApplicationResponse{
		Count: uint64(count),
	}, nil
}

func (r *ApplicationRepo) List(ctx context.Context, req *paginationV1.PagingRequest) (*ubaV1.ListApplicationResponse, error) {
	if req == nil {
		return nil, ubaV1.ErrorBadRequest("invalid parameter")
	}

	builder := r.entClient.Client().Application.Query()

	ret, err := r.repository.ListWithPaging(ctx, builder, builder.Clone(), req)
	if err != nil {
		return nil, err
	}
	if ret == nil {
		return &ubaV1.ListApplicationResponse{Total: 0, Items: nil}, nil
	}

	return &ubaV1.ListApplicationResponse{
		Total: ret.Total,
		Items: ret.Items,
	}, nil
}

func (r *ApplicationRepo) IsExist(ctx context.Context, id uint32) (bool, error) {
	exist, err := r.entClient.Client().Application.Query().
		Where(application.IDEQ(id)).
		Exist(ctx)
	if err != nil {
		r.log.Errorf("query exist failed: %s", err.Error())
		return false, ubaV1.ErrorInternalServerError("query exist failed")
	}
	return exist, nil
}

func (r *ApplicationRepo) Get(ctx context.Context, req *ubaV1.GetApplicationRequest) (*ubaV1.Application, error) {
	if req == nil {
		return nil, ubaV1.ErrorBadRequest("invalid parameter")
	}

	builder := r.entClient.Client().Application.Query()

	var whereCond []func(s *sql.Selector)
	switch req.QueryBy.(type) {
	default:
	case *ubaV1.GetApplicationRequest_Id:
		whereCond = append(whereCond, application.IDEQ(req.GetId()))
	}

	dto, err := r.repository.Get(ctx, builder, req.GetViewMask(), whereCond...)
	if err != nil {
		return nil, err
	}

	return dto, err
}

func (r *ApplicationRepo) Create(ctx context.Context, req *ubaV1.CreateApplicationRequest) error {
	if req == nil || req.Data == nil {
		return ubaV1.ErrorBadRequest("invalid parameter")
	}

	builder := r.newApplicationCreate(req.Data)

	if err := builder.Exec(ctx); err != nil {
		r.log.Errorf("insert application failed: %s", err.Error())
		return ubaV1.ErrorInternalServerError("insert application failed")
	}

	return nil
}

func (r *ApplicationRepo) newApplicationCreate(data *ubaV1.Application) *ent.ApplicationCreate {
	builder := r.entClient.Client().Application.Create().
		SetNillableName(data.Name).
		SetNillableAppID(data.AppId).
		SetNillableAppKey(data.AppKey).
		SetNillableAppSecret(data.AppSecret).
		SetNillableType(r.typeConverter.ToEntity(data.Type)).
		SetNillableStatus(r.statusConverter.ToEntity(data.Status)).
		SetNillableRemark(data.Remark).
		SetNillableDesensitize(data.Desensitize).
		SetNillableWebhookURL(data.WebhookUrl).
		SetNillableWebhookSecret(data.WebhookSecret).
		SetNillableCreatedBy(data.CreatedBy).
		SetCreatedAt(time.Now())

	return builder
}

func (r *ApplicationRepo) BatchCreate(ctx context.Context, apps []*ubaV1.Application) error {
	if len(apps) == 0 {
		return nil
	}

	bulk := make([]*ent.ApplicationCreate, 0, len(apps))
	for _, dto := range apps {
		builder := r.newApplicationCreate(dto)
		bulk = append(bulk, builder)
	}

	bulkBuilder := r.entClient.Client().Application.CreateBulk(bulk...)

	if err := bulkBuilder.Exec(ctx); err != nil {
		r.log.Errorf("batch insert applications failed: %s", err.Error())
		return ubaV1.ErrorInternalServerError("batch insert applications failed")
	}

	return nil
}

func (r *ApplicationRepo) Update(ctx context.Context, req *ubaV1.UpdateApplicationRequest) error {
	if req == nil || req.Data == nil {
		return ubaV1.ErrorBadRequest("invalid parameter")
	}

	if req.GetAllowMissing() {
		exist, err := r.IsExist(ctx, req.GetId())
		if err != nil {
			return err
		}
		if !exist {
			createReq := &ubaV1.CreateApplicationRequest{Data: req.Data}
			createReq.Data.CreatedBy = createReq.Data.UpdatedBy
			createReq.Data.UpdatedBy = nil
			return r.Create(ctx, createReq)
		}
	}

	builder := r.entClient.Client().Debug().Application.Update()
	err := r.repository.UpdateX(ctx, builder, req.Data, req.GetUpdateMask(),
		func(dto *ubaV1.Application) {
			builder.
				SetNillableName(req.Data.Name).
				SetNillableAppID(req.Data.AppId).
				SetNillableAppKey(req.Data.AppKey).
				SetNillableAppSecret(req.Data.AppSecret).
				SetNillableType(r.typeConverter.ToEntity(req.Data.Type)).
				SetNillableStatus(r.statusConverter.ToEntity(req.Data.Status)).
				SetNillableRemark(req.Data.Remark).
				SetNillableDesensitize(req.Data.Desensitize).
				SetNillableWebhookURL(req.Data.WebhookUrl).
				SetNillableWebhookSecret(req.Data.WebhookSecret).
				SetNillableUpdatedBy(req.Data.UpdatedBy).
				SetUpdatedAt(time.Now())
		},
		func(s *sql.Selector) {
			s.Where(sql.EQ(application.FieldID, req.GetId()))
		},
	)

	return err
}

func (r *ApplicationRepo) Delete(ctx context.Context, req *ubaV1.DeleteApplicationRequest) error {
	if req == nil {
		return ubaV1.ErrorBadRequest("invalid parameter")
	}

	builder := r.entClient.Client().Debug().Application.Delete()

	_, err := r.repository.Delete(ctx, builder, func(s *sql.Selector) {
		s.Where(sql.EQ(application.FieldID, req.GetId()))
	})
	if err != nil {
		r.log.Errorf("delete application failed: %s", err.Error())
		return ubaV1.ErrorInternalServerError("delete application failed")
	}

	return nil
}

func (r *ApplicationRepo) Truncate(ctx context.Context) error {
	if _, err := r.entClient.Client().Application.Delete().Exec(ctx); err != nil {
		r.log.Errorf("failed to truncate applications table: %s", err.Error())
		return ubaV1.ErrorInternalServerError("truncate failed")
	}
	return nil
}
