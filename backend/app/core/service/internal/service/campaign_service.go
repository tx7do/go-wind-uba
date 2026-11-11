package service

import (
	"context"
	"fmt"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/hibiken/asynq"
	"github.com/tx7do/go-utils/trans"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"google.golang.org/protobuf/types/known/emptypb"

	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"

	"go-wind-uba/app/core/service/internal/data"

	"go-wind-uba/pkg/task"

	notificationV1 "go-wind-uba/api/gen/go/notification/service/v1"
	ubaV1 "go-wind-uba/api/gen/go/uba/service/v1"
	appViewer "go-wind-uba/pkg/entgo/viewer"
)

const (
	// campaignSendBatchSize 群发时每次从分群成员表取出的用户数。
	campaignSendBatchSize = 500

	// campaignLedgerEventMode 群发走站内信渠道。
	campaignChannel = notificationV1.Channel_INTERNAL
)

// CampaignService 营销触达：分群人群 × 文案 × 站内信渠道的群发执行层。
// 逐用户经通知域 SendDirect 投递（台账/脱敏/幂等继承通知域）；支持立即发送与
// asynq ProcessAt 定时发送；取消靠状态机（任务触发时按状态跳过）。
type CampaignService struct {
	ubaV1.UnimplementedCampaignServiceServer

	log *log.Helper

	campaignRepo    *data.CampaignRepo
	segmentRepo     *data.SegmentRepo
	segmentUserRepo *data.SegmentUserRepo
	notifier        Notifier

	// taskEnqueuer 异步派发能力（asynq 未配置时 Send 回退同步执行）。
	taskEnqueuer TaskEnqueuer
}

func NewCampaignService(
	ctx *bootstrap.Context,
	campaignRepo *data.CampaignRepo,
	segmentRepo *data.SegmentRepo,
	segmentUserRepo *data.SegmentUserRepo,
) *CampaignService {
	return &CampaignService{
		log:             ctx.NewLoggerHelper("campaign/service/core-service"),
		campaignRepo:    campaignRepo,
		segmentRepo:     segmentRepo,
		segmentUserRepo: segmentUserRepo,
	}
}

// RegisterTaskEnqueuer 注入 asynq 入队能力（asynq_server 装配期调用）。
func (s *CampaignService) RegisterTaskEnqueuer(enqueuer TaskEnqueuer) {
	s.taskEnqueuer = enqueuer
}

// RegisterNotifier 注入通知出口（通知域 NotificationService 实现该接口）。
func (s *CampaignService) RegisterNotifier(notifier Notifier) {
	s.notifier = notifier
}

func (s *CampaignService) List(ctx context.Context, req *paginationV1.PagingRequest) (*ubaV1.ListCampaignResponse, error) {
	return s.campaignRepo.List(ctx, req)
}

func (s *CampaignService) Count(ctx context.Context, req *paginationV1.PagingRequest) (*ubaV1.CountCampaignResponse, error) {
	return s.campaignRepo.Count(ctx, req)
}

func (s *CampaignService) Get(ctx context.Context, req *ubaV1.GetCampaignRequest) (*ubaV1.Campaign, error) {
	return s.campaignRepo.Get(ctx, req)
}

func (s *CampaignService) Create(ctx context.Context, req *ubaV1.CreateCampaignRequest) (*ubaV1.Campaign, error) {
	if req == nil || req.Data == nil {
		return nil, ubaV1.ErrorCampaignBadRequest("invalid parameter")
	}
	if err := s.validateData(req.Data); err != nil {
		return nil, err
	}

	created, err := s.campaignRepo.Create(ctx, req)
	if err != nil {
		return nil, err
	}

	// 计划模式：创建即按 scheduled_at 入队延迟任务
	if created.GetSendMode() == ubaV1.CampaignSendMode_CAMPAIGN_SEND_MODE_SCHEDULED {
		if err := s.scheduleSend(ctx, created); err != nil {
			return created, err
		}
	}

	return created, nil
}

func (s *CampaignService) Update(ctx context.Context, req *ubaV1.UpdateCampaignRequest) (*ubaV1.Campaign, error) {
	if req == nil || req.Data == nil {
		return nil, ubaV1.ErrorCampaignBadRequest("invalid parameter")
	}

	entity, err := s.campaignRepo.GetEntity(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	// 仅 DRAFT / CANCELLED 可编辑；已进入发送流程的活动不可改
	if st := entity.Status; st != "DRAFT" && st != "CANCELLED" {
		return nil, ubaV1.ErrorCampaignInvalidState("campaign in status %s cannot be edited", st)
	}
	if err := s.validateData(req.Data); err != nil {
		return nil, err
	}

	updated, err := s.campaignRepo.Update(ctx, req)
	if err != nil {
		return nil, err
	}

	// 计划模式更新：重新入队
	if updated.GetSendMode() == ubaV1.CampaignSendMode_CAMPAIGN_SEND_MODE_SCHEDULED {
		if err := s.scheduleSend(ctx, updated); err != nil {
			return updated, err
		}
	}

	return updated, nil
}

func (s *CampaignService) Delete(ctx context.Context, req *ubaV1.DeleteCampaignRequest) (*emptypb.Empty, error) {
	if req == nil || req.GetId() == 0 {
		return nil, ubaV1.ErrorCampaignBadRequest("invalid parameter")
	}

	if err := s.campaignRepo.Delete(ctx, req); err != nil {
		return nil, err
	}

	return &emptypb.Empty{}, nil
}

// Send 立即发送：入队群发任务（asynq 可用时异步，否则同步执行）。
func (s *CampaignService) Send(ctx context.Context, req *ubaV1.SendCampaignRequest) (*ubaV1.Campaign, error) {
	if req == nil || req.GetId() == 0 {
		return nil, ubaV1.ErrorCampaignBadRequest("invalid parameter")
	}

	entity, err := s.campaignRepo.GetEntity(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	if st := entity.Status; st != "DRAFT" && st != "CANCELLED" && st != "FAILED" {
		return nil, ubaV1.ErrorCampaignInvalidState("campaign in status %s cannot be sent", st)
	}

	if err := s.campaignRepo.UpdateStatus(ctx, entity.ID, "SENDING", time.Now()); err != nil {
		return nil, err
	}

	if s.taskEnqueuer != nil {
		if err = s.taskEnqueuer.NewTask(task.CampaignSendTaskType, &task.CampaignSendTaskData{CampaignID: entity.ID}); err != nil {
			s.log.Errorf("enqueue campaign send failed, fallback to sync: %s", err.Error())
		} else {
			return s.campaignRepo.Get(ctx, &ubaV1.GetCampaignRequest{
				QueryBy: &ubaV1.GetCampaignRequest_Id{Id: entity.ID},
			})
		}
	}

	if err = s.executeSend(ctx, entity.ID); err != nil {
		return nil, err
	}

	return s.campaignRepo.Get(ctx, &ubaV1.GetCampaignRequest{
		QueryBy: &ubaV1.GetCampaignRequest_Id{Id: entity.ID},
	})
}

// Cancel 取消：仅 DRAFT / SCHEDULED 可取消；群发任务触发时按状态跳过。
func (s *CampaignService) Cancel(ctx context.Context, req *ubaV1.CancelCampaignRequest) (*ubaV1.Campaign, error) {
	if req == nil || req.GetId() == 0 {
		return nil, ubaV1.ErrorCampaignBadRequest("invalid parameter")
	}

	entity, err := s.campaignRepo.GetEntity(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	if st := entity.Status; st != "DRAFT" && st != "SCHEDULED" {
		return nil, ubaV1.ErrorCampaignInvalidState("campaign in status %s cannot be cancelled", st)
	}

	now := time.Now()
	if err = s.campaignRepo.UpdateStatus(ctx, entity.ID, "CANCELLED", now); err != nil {
		return nil, err
	}

	return s.campaignRepo.Get(ctx, &ubaV1.GetCampaignRequest{
		QueryBy: &ubaV1.GetCampaignRequest_Id{Id: entity.ID},
	})
}

// ──────────── 群发执行 ────────────

// scheduleSend 计划发送：asynq ProcessAt 延迟任务，到点触发 AsyncCampaignSend。
func (s *CampaignService) scheduleSend(ctx context.Context, campaign *ubaV1.Campaign) error {
	if campaign.GetScheduledAt() == nil {
		return ubaV1.ErrorCampaignBadRequest("scheduled_at is required for SCHEDULED mode")
	}
	if s.taskEnqueuer == nil {
		// asynq 未配置：无法延迟，保持 SCHEDULED 状态由人工触发
		s.log.Warnf("asynq not configured, campaign [%d] stays scheduled until manual send", campaign.GetId())
		return nil
	}

	if err := s.taskEnqueuer.NewTask(
		task.CampaignSendTaskType,
		&task.CampaignSendTaskData{CampaignID: campaign.GetId()},
		asynq.ProcessAt(campaign.GetScheduledAt().AsTime()),
	); err != nil {
		s.log.Errorf("schedule campaign send failed: %s", err.Error())
		return ubaV1.ErrorCampaignInternalError("schedule campaign send failed")
	}
	return nil
}

// executeSend 群发执行：分页遍历分群成员，逐用户经通知域 SendDirect 投递。
func (s *CampaignService) executeSend(ctx context.Context, campaignID int64) error {
	entity, err := s.campaignRepo.GetEntity(ctx, campaignID)
	if err != nil {
		return err
	}

	now := time.Now()
	if err = s.campaignRepo.UpdateStatus(ctx, campaignID, "SENDING", now); err != nil {
		return err
	}

	if entity.SegmentID == 0 {
		return s.finishSend(ctx, entity.ID, 0, 0, 1, now)
	}

	var (
		total  int64
		sent   int64
		failed int64
	)

	page := 1
	for {
		users, _, listErr := s.segmentUserRepo.ListBySegment(ctx, entity.SegmentID, page, campaignSendBatchSize)
		if listErr != nil {
			return listErr
		}
		if len(users) == 0 {
			break
		}

		for _, su := range users {
			total++
			_, sendErr := s.notifier.SendDirect(ctx, &notificationV1.SendDirectNotificationRequest{
				EventType:       notificationV1.EventType_CAMPAIGN,
				Channel:         campaignChannel.Enum(),
				RecipientUserId: &su.UserID,
				Title:           derefStr(entity.Title),
				Content:         derefStr(entity.Content),
				TemplateCode:    entity.TemplateCode,
				RelatedId:       &entity.ID,
				RequestId:       trans.Ptr(fmt.Sprintf("campaign:%d:%d", entity.ID, su.UserID)),
			})
			if sendErr != nil {
				s.log.Errorf("campaign [%d] send to user [%d] failed: %s", entity.ID, su.UserID, sendErr.Error())
				failed++
				continue
			}
			sent++
		}

		page++
	}

	return s.finishSend(ctx, entity.ID, total, sent, failed, time.Now())
}

// finishSend 回写统计并按结果定案状态。
func (s *CampaignService) finishSend(ctx context.Context, id int64, total, sent, failed int64, now time.Time) error {
	status := "SENT"
	if sent == 0 && failed > 0 {
		status = "FAILED"
	} else if failed > 0 {
		status = "PARTIAL"
	}
	return s.campaignRepo.UpdateSendProgress(ctx, id, total, sent, failed, status, now)
}

// validateData 创建/更新时的基础校验。
func (s *CampaignService) validateData(data *ubaV1.Campaign) error {
	if data.GetName() == "" {
		return ubaV1.ErrorCampaignBadRequest("campaign name is required")
	}
	if data.GetSegmentId() == 0 {
		return ubaV1.ErrorCampaignBadRequest("segment_id is required")
	}
	if data.GetTitle() == "" && data.GetTemplateCode() == "" {
		return ubaV1.ErrorCampaignBadRequest("title or template_code is required")
	}
	return nil
}

// AsyncCampaignSend asynq 群发任务消费者：延迟任务（计划发送）与即时任务都走这里。
// 签名与上游同构（taskType 在前），经 kratos-transport 的泛型订阅器适配。
func (s *CampaignService) AsyncCampaignSend(taskType string, payload *task.CampaignSendTaskData) error {
	return s.asyncCampaignSend(appViewer.NewSystemViewerContext(context.Background()), payload)
}

// asyncCampaignSend 实际执行（系统上下文绕过租户隐私层）。
func (s *CampaignService) asyncCampaignSend(ctx context.Context, payload *task.CampaignSendTaskData) error {
	if payload == nil || payload.CampaignID == 0 {
		return nil
	}

	// 已取消/已发送的活动直接跳过（状态机把关）
	entity, err := s.campaignRepo.GetEntity(ctx, payload.CampaignID)
	if err != nil {
		return err
	}
	if st := entity.Status; st != "DRAFT" && st != "SCHEDULED" && st != "SENDING" && st != "FAILED" {
		s.log.Infof("campaign [%d] in status %s, skip async send", entity.ID, st)
		return nil
	}

	return s.executeSend(ctx, payload.CampaignID)
}
