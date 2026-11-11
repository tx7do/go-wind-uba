package service

import (
	"context"
	"github.com/go-kratos/kratos/v2/log"
	"time"

	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/tx7do/kratos-bootstrap/bootstrap"

	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	"github.com/tx7do/go-utils/trans"

	notificationV1 "go-wind-uba/api/gen/go/notification/service/v1"

	"go-wind-uba/app/core/service/internal/data"
	"go-wind-uba/pkg/middleware/auth"
)

// 静音时段默认窗口：22:00 → 次日 08:00（分钟数自当日 00:00 起）。
// 与 ent schema 的列默认值保持一致；用户从未配置过偏好时按此返回。
const (
	defaultQuietStartMinute = 1320
	defaultQuietEndMinute   = 480

	quietMinuteUpperBound = 24*60 - 1 // 1439
)

// NotificationPreferenceService 用户通知偏好（自助服务）：
// 读写的都是当前操作人自己的偏好，无管理面、无平台守卫。
type NotificationPreferenceService struct {
	notificationV1.UnimplementedNotificationPreferenceServiceServer

	prefRepo     *data.NotificationPreferenceRepo
	categoryRepo *data.InternalMessageCategoryRepo
	log          *log.Helper
}

func NewNotificationPreferenceService(
	ctx *bootstrap.Context,
	prefRepo *data.NotificationPreferenceRepo,
	categoryRepo *data.InternalMessageCategoryRepo,
) *NotificationPreferenceService {
	return &NotificationPreferenceService{
		prefRepo:     prefRepo,
		categoryRepo: categoryRepo,
		log:          ctx.NewLoggerHelper("notification-preference/service/admin-service"),
	}
}

// GetMyNotificationPreference 返回当前用户的偏好；从未配置过时返回默认值
// （未启用静音、无退订），前端据此渲染表单，落库发生在第一次保存。
func (s *NotificationPreferenceService) GetMyNotificationPreference(ctx context.Context, _ *emptypb.Empty) (*notificationV1.NotificationPreference, error) {
	operator, err := auth.FromContext(ctx)
	if err != nil {
		return nil, err
	}

	pref, err := s.prefRepo.GetByUserID(ctx, operator.GetUserId())
	if err != nil {
		return nil, err
	}
	if pref == nil {
		return &notificationV1.NotificationPreference{
			UserId:           trans.Ptr(operator.GetUserId()),
			QuietEnabled:     trans.Ptr(false),
			QuietStartMinute: trans.Ptr(int32(defaultQuietStartMinute)),
			QuietEndMinute:   trans.Ptr(int32(defaultQuietEndMinute)),
		}, nil
	}

	return pref, nil
}

// UpdateMyNotificationPreference 保存当前用户的偏好。user_id 从操作人钉定，
// 客户端传什么都不影响写入对象。
func (s *NotificationPreferenceService) UpdateMyNotificationPreference(ctx context.Context, req *notificationV1.UpdateNotificationPreferenceRequest) (*notificationV1.NotificationPreference, error) {
	operator, err := auth.FromContext(ctx)
	if err != nil {
		return nil, err
	}

	if req == nil {
		return nil, notificationV1.ErrorNotificationBadRequest("invalid parameter")
	}
	if err := validateQuietWindow(req.GetQuietEnabled(), req.GetQuietStartMinute(), req.GetQuietEndMinute()); err != nil {
		return nil, err
	}
	if len(req.GetMutedCategoryIds()) > 100 {
		return nil, notificationV1.ErrorNotificationBadRequest("too many muted categories")
	}

	pref, err := s.prefRepo.Upsert(ctx, operator.GetUserId(), req, operator.GetUserId())
	if err != nil {
		return nil, err
	}

	return pref, nil
}

// ListMyNotificationCategories 当前用户可退订的启用分类清单。
// 分类是租户实体，viewer 租户谓词天然把清单筛成本租户；跨租户 ID 即使被塞进
// 退订列表也永不命中广播比对，无需在此额外校验。
func (s *NotificationPreferenceService) ListMyNotificationCategories(ctx context.Context, _ *emptypb.Empty) (*notificationV1.ListMyNotificationCategoriesResponse, error) {
	categories, err := s.categoryRepo.List(ctx, &paginationV1.PagingRequest{
		NoPaging: trans.Ptr(true),
	})
	if err != nil {
		return nil, err
	}

	items := make([]*notificationV1.NotificationCategoryItem, 0, len(categories.GetItems()))
	for _, c := range categories.GetItems() {
		if !c.GetIsEnabled() {
			continue
		}
		items = append(items, &notificationV1.NotificationCategoryItem{
			Id:   c.Id,
			Name: c.Name,
		})
	}

	return &notificationV1.ListMyNotificationCategoriesResponse{Items: items}, nil
}

// validateQuietWindow 校验静音窗口：分钟数在 0..1439 且启用时 start != end
// （start == end 的窗口宽为零，等价于不静音，按配置错误拒绝而不是默默不生效）。
// start > end 是合法的跨零点窗口（如 22:00→08:00）。
func validateQuietWindow(enabled bool, startMinute, endMinute int32) error {
	for _, m := range []int32{startMinute, endMinute} {
		if m < 0 || m > quietMinuteUpperBound {
			return notificationV1.ErrorNotificationBadRequest("quiet minutes must be within 0..1439")
		}
	}
	if enabled && startMinute == endMinute {
		return notificationV1.ErrorNotificationBadRequest("quiet start and end must differ")
	}
	return nil
}

// inQuietWindow 判定 now 是否落在偏好定义的静音窗口内（服务器本地时间）。
// pref 为 nil 或未启用时返回 false。窗口跨零点（start > end）按"晚于 start 或早于 end"判定。
func inQuietWindow(pref *notificationV1.NotificationPreference, now time.Time) bool {
	if pref == nil || !pref.GetQuietEnabled() {
		return false
	}
	minutesOfDay := now.Hour()*60 + now.Minute()
	start, end := pref.GetQuietStartMinute(), pref.GetQuietEndMinute()
	if start == end {
		return false // 零宽窗口不静音（服务端兜底，与校验层一致）
	}
	if start < end {
		return minutesOfDay >= int(start) && minutesOfDay < int(end)
	}
	// 跨零点：22:00→08:00 覆盖 [start, 24h) ∪ [0, end)
	return minutesOfDay >= int(start) || minutesOfDay < int(end)
}
