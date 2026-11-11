package data

import (
	"context"
	"time"

	"entgo.io/ent/dialect/sql"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/go-crud/viewer"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	entCrud "github.com/tx7do/go-crud/entgo"

	"github.com/tx7do/go-utils/copierutil"
	"github.com/tx7do/go-utils/mapper"
	"github.com/tx7do/go-utils/timeutil"
	"github.com/tx7do/go-utils/trans"

	"go-wind-uba/app/core/service/internal/data/ent"
	"go-wind-uba/app/core/service/internal/data/ent/internalmessage"
	"go-wind-uba/app/core/service/internal/data/ent/internalmessagerecipient"
	"go-wind-uba/app/core/service/internal/data/ent/predicate"

	internalMessageV1 "go-wind-uba/api/gen/go/internal_message/service/v1"
)

type InternalMessageRecipientRepo struct {
	entClient *entCrud.EntClient[*ent.Client]
	log       *log.Helper

	mapper          *mapper.CopierMapper[internalMessageV1.InternalMessageRecipient, ent.InternalMessageRecipient]
	statusConverter *mapper.EnumTypeConverter[internalMessageV1.InternalMessageRecipient_Status, internalmessagerecipient.Status]

	repository *entCrud.Repository[
		ent.InternalMessageRecipientQuery, ent.InternalMessageRecipientSelect,
		ent.InternalMessageRecipientCreate, ent.InternalMessageRecipientCreateBulk,
		ent.InternalMessageRecipientUpdate, ent.InternalMessageRecipientUpdateOne,
		ent.InternalMessageRecipientDelete,
		predicate.InternalMessageRecipient,
		internalMessageV1.InternalMessageRecipient, ent.InternalMessageRecipient,
	]
}

func NewInternalMessageRecipientRepo(ctx *bootstrap.Context, entClient *entCrud.EntClient[*ent.Client]) *InternalMessageRecipientRepo {
	repo := &InternalMessageRecipientRepo{
		log:             ctx.NewLoggerHelper("internal-message-recipient/repo/core-service"),
		entClient:       entClient,
		mapper:          mapper.NewCopierMapper[internalMessageV1.InternalMessageRecipient, ent.InternalMessageRecipient](),
		statusConverter: mapper.NewEnumTypeConverter[internalMessageV1.InternalMessageRecipient_Status, internalmessagerecipient.Status](internalMessageV1.InternalMessageRecipient_Status_name, internalMessageV1.InternalMessageRecipient_Status_value),
	}

	repo.init()

	return repo
}

func (r *InternalMessageRecipientRepo) init() {
	r.repository = entCrud.NewRepository[
		ent.InternalMessageRecipientQuery, ent.InternalMessageRecipientSelect,
		ent.InternalMessageRecipientCreate, ent.InternalMessageRecipientCreateBulk,
		ent.InternalMessageRecipientUpdate, ent.InternalMessageRecipientUpdateOne,
		ent.InternalMessageRecipientDelete,
		predicate.InternalMessageRecipient,
		internalMessageV1.InternalMessageRecipient, ent.InternalMessageRecipient,
	](r.mapper)

	r.mapper.AppendConverters(copierutil.NewTimeStringConverterPair())
	r.mapper.AppendConverters(copierutil.NewTimeTimestamppbConverterPair())

	r.mapper.AppendConverters(r.statusConverter.NewConverterPair())
}

func (r *InternalMessageRecipientRepo) Count(ctx context.Context, whereCond []func(s *sql.Selector)) (int, error) {
	builder := r.entClient.Client().InternalMessageRecipient.Query()
	if len(whereCond) != 0 {
		builder.Modify(whereCond...)
	}

	count, err := builder.Count(ctx)
	if err != nil {
		r.log.Errorf("query count failed: %s", err.Error())
		return 0, internalMessageV1.ErrorInternalServerError("query count failed")
	}

	return count, nil
}

func (r *InternalMessageRecipientRepo) IsExist(ctx context.Context, id int64) (bool, error) {
	exist, err := r.entClient.Client().InternalMessageRecipient.Query().
		Where(internalmessagerecipient.IDEQ(id)).
		Exist(ctx)
	if err != nil {
		r.log.Errorf("query exist failed: %s", err.Error())
		return false, internalMessageV1.ErrorInternalServerError("query exist failed")
	}
	return exist, nil
}

func (r *InternalMessageRecipientRepo) List(ctx context.Context, req *paginationV1.PagingRequest) (*internalMessageV1.ListUserInboxResponse, error) {
	if req == nil {
		return nil, internalMessageV1.ErrorBadRequest("invalid parameter")
	}

	builder := r.entClient.Client().InternalMessageRecipient.Query()

	// 收件箱按"我自己的"来读，归属谓词必须由服务端给出：查询条件整个来自调用方的 query 字符串，
	// 不钉住 recipient_user_id 就等于同租户任何登录用户改一个 ID 就能翻别人的收件记录
	// （title/content 随父消息回填，一并跟着走）。三端页面本来就各自在前端塞这个条件，
	// 钉住之后前端传不传都只会拿到自己的行。
	// 平台/系统视图豁免：用户详情页要看指定用户的收件箱，异步任务路径也需要全量读。
	if vc, ok := viewer.FromContext(ctx); ok && !vc.IsPlatformContext() && !vc.IsSystemContext() {
		builder.Where(internalmessagerecipient.RecipientUserIDEQ(int64(vc.UserID())))
	}

	ret, err := r.repository.ListWithPaging(ctx, builder, builder.Clone(), req)
	if err != nil {
		return nil, err
	}
	if ret == nil {
		return &internalMessageV1.ListUserInboxResponse{Total: 0, Items: nil}, nil
	}

	return &internalMessageV1.ListUserInboxResponse{
		Total: ret.Total,
		Items: ret.Items,
	}, nil
}

func (r *InternalMessageRecipientRepo) Get(ctx context.Context, req *internalMessageV1.GetInternalMessageRecipientRequest) (*internalMessageV1.InternalMessageRecipient, error) {
	if req == nil {
		return nil, internalMessageV1.ErrorBadRequest("invalid parameter")
	}

	builder := r.entClient.Client().InternalMessageRecipient.Query()

	var whereCond []func(s *sql.Selector)
	switch req.QueryBy.(type) {
	default:
	case *internalMessageV1.GetInternalMessageRecipientRequest_Id:
		whereCond = append(whereCond, internalmessagerecipient.IDEQ(req.GetId()))
	}

	dto, err := r.repository.Get(ctx, builder, req.GetViewMask(), whereCond...)
	if err != nil {
		return nil, err
	}

	return dto, err
}

func (r *InternalMessageRecipientRepo) Create(ctx context.Context, req *internalMessageV1.InternalMessageRecipient) (*internalMessageV1.InternalMessageRecipient, error) {
	if req == nil {
		return nil, internalMessageV1.ErrorBadRequest("invalid parameter")
	}

	builder := r.entClient.Client().InternalMessageRecipient.Create().
		SetNillableTenantID(req.TenantId).
		SetNillableMessageID(req.MessageId).
		SetNillableRecipientUserID(req.RecipientUserId).
		SetNillableStatus(r.statusConverter.ToEntity(req.Status)).
		SetNillableReceivedAt(timeutil.TimestamppbToTime(req.ReceivedAt)).
		SetNillableReadAt(timeutil.TimestamppbToTime(req.ReadAt)).
		SetCreatedAt(time.Now())

	var err error
	var entity *ent.InternalMessageRecipient
	if entity, err = builder.Save(ctx); err != nil {
		r.log.Errorf("insert internal message recipient failed: %s", err.Error())
		return nil, internalMessageV1.ErrorInternalServerError("insert internal message recipient failed")
	}

	return r.mapper.ToDTO(entity), nil
}

func (r *InternalMessageRecipientRepo) Update(ctx context.Context, req *internalMessageV1.UpdateInternalMessageRecipientRequest) error {
	if req == nil || req.Data == nil {
		return internalMessageV1.ErrorBadRequest("invalid parameter")
	}

	// 如果不存在则创建
	if req.GetAllowMissing() {
		exist, err := r.IsExist(ctx, req.GetId())
		if err != nil {
			return err
		}
		if !exist {
			req.Data.CreatedBy = req.Data.UpdatedBy
			req.Data.UpdatedBy = nil
			_, err = r.Create(ctx, req.Data)
			return err
		}
	}

	builder := r.entClient.Client().InternalMessageRecipient.Update()
	err := r.repository.UpdateX(ctx, builder, req.Data, req.GetUpdateMask(),
		func(dto *internalMessageV1.InternalMessageRecipient) {
			builder.
				SetNillableMessageID(req.Data.MessageId).
				SetNillableRecipientUserID(req.Data.RecipientUserId).
				SetNillableStatus(r.statusConverter.ToEntity(req.Data.Status)).
				SetNillableReceivedAt(timeutil.TimestamppbToTime(req.Data.ReceivedAt)).
				SetNillableReadAt(timeutil.TimestamppbToTime(req.Data.ReadAt)).
				SetUpdatedAt(time.Now())
		},
		func(s *sql.Selector) {
			s.Where(sql.EQ(internalmessagerecipient.FieldID, req.GetId()))
		},
	)

	return err
}

func (r *InternalMessageRecipientRepo) Delete(ctx context.Context, id int64) error {
	if id == 0 {
		return internalMessageV1.ErrorBadRequest("invalid parameter")
	}

	if err := r.entClient.Client().InternalMessageRecipient.DeleteOneID(id).Exec(ctx); err != nil {
		if ent.IsNotFound(err) {
			return internalMessageV1.ErrorNotFound("internal message recipient not found")
		}

		r.log.Errorf("delete one data failed: %s", err.Error())

		return internalMessageV1.ErrorInternalServerError("delete failed")
	}

	return nil
}

// inboxScopedUserID 收件箱操作的目标用户以服务端上下文为准：
// 请求体里的 user_id 是客户端可任填的字段，直接采信等于任何登录用户
// 都能标记/删除/撤销别人的收件记录。平台/系统视图豁免（管理端代操作）。
func (r *InternalMessageRecipientRepo) inboxScopedUserID(ctx context.Context, reqUserID int64) int64 {
	if vc, ok := viewer.FromContext(ctx); ok && !vc.IsPlatformContext() && !vc.IsSystemContext() {
		return int64(vc.UserID())
	}

	return reqUserID
}

// MarkNotificationAsRead 将通知标记为已读
func (r *InternalMessageRecipientRepo) MarkNotificationAsRead(ctx context.Context, req *internalMessageV1.MarkNotificationAsReadRequest) error {
	if len(req.GetRecipientIds()) == 0 {
		return internalMessageV1.ErrorBadRequest("invalid parameter")
	}
	if req.GetUserId() == 0 {
		return internalMessageV1.ErrorBadRequest("invalid parameter")
	}

	userID := r.inboxScopedUserID(ctx, req.GetUserId())

	now := time.Now()
	_, err := r.entClient.Client().InternalMessageRecipient.Update().
		Where(
			internalmessagerecipient.IDIn(req.GetRecipientIds()...),
			internalmessagerecipient.RecipientUserIDEQ(userID),
			internalmessagerecipient.StatusNEQ(internalmessagerecipient.StatusRead),
		).
		SetStatus(internalmessagerecipient.StatusRead).
		SetNillableReadAt(trans.Ptr(now)).
		SetNillableUpdatedAt(trans.Ptr(now)).
		Save(ctx)
	return err
}

// MarkNotificationsStatus 标记特定用户的某些或所有通知的状态
func (r *InternalMessageRecipientRepo) MarkNotificationsStatus(ctx context.Context, req *internalMessageV1.MarkNotificationsStatusRequest) error {
	if len(req.GetRecipientIds()) == 0 {
		return internalMessageV1.ErrorBadRequest("invalid parameter")
	}
	if req.GetUserId() == 0 {
		return internalMessageV1.ErrorBadRequest("invalid parameter")
	}

	userID := r.inboxScopedUserID(ctx, req.GetUserId())

	now := time.Now()
	var readAt *time.Time
	var receiveAt *time.Time
	switch req.GetNewStatus() {
	case internalMessageV1.InternalMessageRecipient_READ:
		readAt = trans.Ptr(now)
	case internalMessageV1.InternalMessageRecipient_RECEIVED:
		receiveAt = trans.Ptr(now)
	}

	_, err := r.entClient.Client().InternalMessageRecipient.Update().
		Where(
			internalmessagerecipient.IDIn(req.GetRecipientIds()...),
			internalmessagerecipient.RecipientUserIDEQ(userID),
			internalmessagerecipient.StatusNEQ(*r.statusConverter.ToEntity(trans.Ptr(req.GetNewStatus()))),
		).
		SetNillableStatus(r.statusConverter.ToEntity(trans.Ptr(req.GetNewStatus()))).
		SetNillableReadAt(readAt).
		SetNillableReceivedAt(receiveAt).
		SetNillableUpdatedAt(trans.Ptr(now)).
		Save(ctx)
	return err
}

// RevokeMessage 撤销某条消息
func (r *InternalMessageRecipientRepo) RevokeMessage(ctx context.Context, req *internalMessageV1.RevokeMessageRequest) error {
	userID := r.inboxScopedUserID(ctx, req.GetUserId())

	_, err := r.entClient.Client().InternalMessageRecipient.Delete().
		Where(
			internalmessagerecipient.MessageIDEQ(req.GetMessageId()),
			internalmessagerecipient.RecipientUserIDEQ(userID),
		).
		Exec(ctx)
	return err
}

// DeleteMessageWithRecipients 事务删除消息与其全部收件行，
// 防止只删消息本体后在收件箱/通知面板留下无标题的幽灵行。
func (r *InternalMessageRecipientRepo) DeleteMessageWithRecipients(ctx context.Context, messageID int64) (err error) {
	if messageID == 0 {
		return internalMessageV1.ErrorBadRequest("invalid parameter")
	}

	var tx *ent.Tx
	tx, err = r.entClient.Client().Tx(ctx)
	if err != nil {
		r.log.Errorf("start transaction failed: %s", err.Error())
		return internalMessageV1.ErrorInternalServerError("start transaction failed")
	}
	defer func() {
		if err != nil {
			if rollbackErr := tx.Rollback(); rollbackErr != nil {
				r.log.Errorf("transaction rollback failed: %s", rollbackErr.Error())
			}
			return
		}
		if commitErr := tx.Commit(); commitErr != nil {
			r.log.Errorf("transaction commit failed: %s", commitErr.Error())
			err = internalMessageV1.ErrorInternalServerError("transaction commit failed")
		}
	}()

	var deleted int
	if deleted, err = tx.InternalMessage.Delete().
		Where(internalmessage.IDEQ(messageID)).
		Exec(ctx); err != nil {
		r.log.Errorf("delete message failed: %s", err.Error())
		return internalMessageV1.ErrorInternalServerError("delete message failed")
	}
	if deleted == 0 {
		err = internalMessageV1.ErrorNotFound("internal message not found")
		return err
	}

	if _, err = tx.InternalMessageRecipient.Delete().
		Where(internalmessagerecipient.MessageIDEQ(messageID)).
		Exec(ctx); err != nil {
		r.log.Errorf("delete recipients failed: %s", err.Error())
		return internalMessageV1.ErrorInternalServerError("delete recipients failed")
	}

	return nil
}

func (r *InternalMessageRecipientRepo) DeleteNotificationFromInbox(ctx context.Context, req *internalMessageV1.DeleteNotificationFromInboxRequest) error {
	userID := r.inboxScopedUserID(ctx, req.GetUserId())

	_, err := r.entClient.Client().InternalMessageRecipient.Delete().
		Where(
			internalmessagerecipient.IDIn(req.GetRecipientIds()...),
			internalmessagerecipient.RecipientUserIDEQ(userID),
		).
		Exec(ctx)
	return err
}
