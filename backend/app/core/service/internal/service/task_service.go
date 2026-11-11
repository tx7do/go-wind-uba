package service

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/hibiken/asynq"
	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	"github.com/tx7do/go-utils/trans"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"google.golang.org/protobuf/types/known/emptypb"

	"go-wind-uba/app/core/service/internal/data"

	taskV1 "go-wind-uba/api/gen/go/task/service/v1"

	"go-wind-uba/pkg/task"
)

// TaskScheduler 任务调度接口
type TaskScheduler interface {
	TaskTypeExists(taskType string) bool
	GetRegisteredTaskTypes() []string

	NewTask(typeName string, msg any, opts ...asynq.Option) error
	NewWaitResultTask(typeName string, msg any, opts ...asynq.Option) error
	NewPeriodicTask(cronSpec, typeName string, msg any, opts ...asynq.Option) (string, error)

	RemovePeriodicTask(id string) error
	RemoveAllPeriodicTask()
}

// TaskService 任务服务
type TaskService struct {
	taskV1.UnimplementedTaskServiceServer

	log *log.Helper

	taskScheduler TaskScheduler

	userRepo data.UserRepo
	taskRepo *data.TaskRepo
}

func NewTaskService(
	ctx *bootstrap.Context,
	taskRepo *data.TaskRepo,
	userRepo data.UserRepo,
) *TaskService {
	svc := &TaskService{
		log:      ctx.NewLoggerHelper("task/service/core-service"),
		taskRepo: taskRepo,
		userRepo: userRepo,
	}

	return svc
}

// NewTask 实现通知域的 TaskEnqueuer：向 asynq 队列投递一条任务。
func (s *TaskService) NewTask(typeName string, msg any, opts ...asynq.Option) error {
	return s.taskScheduler.NewTask(typeName, msg, opts...)
}

func (s *TaskService) RegisterTaskScheduler(taskScheduler TaskScheduler) {
	s.taskScheduler = taskScheduler
}

func (s *TaskService) List(ctx context.Context, req *paginationV1.PagingRequest) (*taskV1.ListTaskResponse, error) {
	return s.taskRepo.List(ctx, req)
}

func (s *TaskService) Get(ctx context.Context, req *taskV1.GetTaskRequest) (*taskV1.Task, error) {
	return s.taskRepo.Get(ctx, req)
}

func (s *TaskService) ListTaskTypeName(_ context.Context, _ *emptypb.Empty) (*taskV1.ListTaskTypeNameResponse, error) {
	typeNames := s.taskScheduler.GetRegisteredTaskTypes()
	return &taskV1.ListTaskTypeNameResponse{
		TypeNames: typeNames,
	}, nil
}

func (s *TaskService) Create(ctx context.Context, req *taskV1.CreateTaskRequest) (*emptypb.Empty, error) {
	if req.Data == nil {
		return nil, taskV1.ErrorBadRequest("invalid parameter")
	}

	var t *taskV1.Task
	var err error
	if t, err = s.taskRepo.Create(ctx, req); err != nil {
		return nil, err
	}

	// 禁用状态的任务只需落库，不进入调度器
	if !t.GetEnable() {
		return &emptypb.Empty{}, nil
	}

	if err = s.startTask(t); err != nil {
		// 调度失败不掩盖：DB 记录已建，但任务实际不会运行，需明确告知
		s.log.Errorf("create task [%s] succeeded but scheduling failed: %s", t.GetTypeName(), err.Error())
		return nil, taskV1.ErrorInternalServerError("task scheduling failed: %s", err.Error())
	}

	return &emptypb.Empty{}, nil
}

func (s *TaskService) Update(ctx context.Context, req *taskV1.UpdateTaskRequest) (*emptypb.Empty, error) {
	if req.Data == nil {
		return nil, taskV1.ErrorBadRequest("invalid parameter")
	}

	// 获取更新前的任务，用于判断调度器中是否有正在运行的注册项。
	// 此处不可吞错：若 Get 失败则 oldTask==nil，下方 remove 会因 oldTask==nil 被跳过，
	// 导致旧调度项残留（任务"停用/更新后仍运行"）。故失败直接返回。
	oldTask, err := s.taskRepo.Get(ctx, &taskV1.GetTaskRequest{QueryBy: &taskV1.GetTaskRequest_Id{Id: req.GetId()}})
	if err != nil {
		return nil, err
	}

	var t *taskV1.Task
	if t, err = s.taskRepo.Update(ctx, req); err != nil {
		return nil, err
	}

	// 先移除调度器中旧的注册项（若存在），避免停用后仍运行、或启用时重复注册。
	// 直接调用 RemovePeriodicTask 绕过 stopTask 内部的 enable 保护，
	// 因为 oldTask 可能已是禁用状态，但其注册项可能仍残留在调度器中。
	if oldTask != nil && oldTask.GetType() == taskV1.Task_PERIODIC && oldTask.GetTypeName() != "" {
		if removeErr := s.taskScheduler.RemovePeriodicTask(oldTask.GetTypeName()); removeErr != nil {
			s.log.Warnf("移除旧定时任务注册项失败[%s]: %v", oldTask.GetTypeName(), removeErr)
		}
	}

	// 根据更新后的 enable 状态决定是否重新启动。
	// 注意：disable 也是一种合法的更新意图（更新即停用），不应视为调度失败。
	// startTask 内部对 enable==false 直接返回 error，若此处无差别调用会把
	// "成功停用"误报成 InternalServerError。因此仅在 enable 时才启动。
	if t.GetEnable() {
		if err = s.startTask(t); err != nil {
			s.log.Error(err.Error())
			return nil, taskV1.ErrorInternalServerError("task scheduling failed: %s", err.Error())
		}
	}

	return &emptypb.Empty{}, nil
}

func (s *TaskService) Delete(ctx context.Context, req *taskV1.DeleteTaskRequest) (*emptypb.Empty, error) {
	var err error
	var t *taskV1.Task
	// 获取待删除任务用于清理调度项。失败必须返回：否则 t==nil 会跳过调度清理，
	// 造成 DB 已删但调度器里周期任务仍触发的"幽灵任务"泄漏。
	if t, err = s.taskRepo.Get(ctx, &taskV1.GetTaskRequest{QueryBy: &taskV1.GetTaskRequest_Id{Id: req.GetId()}}); err != nil {
		return nil, err
	}

	if err = s.taskRepo.Delete(ctx, req); err != nil {
		return nil, err
	}

	// DB 删除成功后清理调度项。stopTask 在 enable=false 时会返回错误并跳过清理，
	// 但 disabled 任务的调度项仍可能残留（经由直接改库或历史 Update 路径），
	// 因此这里绕过 enable 保护，直接按 typeName 注销。
	if t != nil && t.GetType() == taskV1.Task_PERIODIC && t.GetTypeName() != "" {
		if removeErr := s.taskScheduler.RemovePeriodicTask(t.GetTypeName()); removeErr != nil {
			// 注销失败仅告警，不阻断删除（DB 记录已删）
			s.log.Warnf("删除任务后注销调度项失败[%s]: %v", t.GetTypeName(), removeErr)
		}
	}

	return &emptypb.Empty{}, nil
}

// ControlTask 控制调度任务
func (s *TaskService) ControlTask(ctx context.Context, req *taskV1.ControlTaskRequest) (*emptypb.Empty, error) {
	t, err := s.taskRepo.Get(ctx, &taskV1.GetTaskRequest{QueryBy: &taskV1.GetTaskRequest_TypeName{TypeName: req.GetTypeName()}})
	if err != nil {
		s.log.Errorf("获取任务失败[%s]", err.Error())
		return nil, err
	}

	switch req.GetControlType() {
	case taskV1.ControlTaskRequest_Restart:
		if err = s.stopTask(t); err != nil {
			return nil, err
		}

		if err = s.startTask(t); err != nil {
			return nil, err
		}

	case taskV1.ControlTaskRequest_Stop:
		err = s.stopTask(t)
		return nil, err

	case taskV1.ControlTaskRequest_Start:
		err = s.startTask(t)
		return nil, err
	}

	return &emptypb.Empty{}, nil
}

// StopAllTask 停止所有的调度任务
func (s *TaskService) StopAllTask(_ context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	s.stopAllTask()
	return &emptypb.Empty{}, nil
}

// StartAllTask 启动所有的调度任务
func (s *TaskService) StartAllTask(ctx context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	_, err := s.startAllTask(ctx)
	if err != nil {
		return nil, err
	}

	return &emptypb.Empty{}, nil
}

// RestartAllTask 重启所有的调度任务
func (s *TaskService) RestartAllTask(ctx context.Context, _ *emptypb.Empty) (*taskV1.RestartAllTaskResponse, error) {
	// 停止所有的任务
	s.stopAllTask()

	// 重新启动所有的任务
	count, err := s.startAllTask(ctx)

	return &taskV1.RestartAllTaskResponse{
		Count: count,
	}, err
}

// StartAllTask 启动所有的任务
func (s *TaskService) startAllTask(ctx context.Context) (int32, error) {
	//_, _ = s.asynqServer.NewPeriodicTask("*/1 * * * ?", task.BackupTaskType, task.BackupTaskData{Name: "test"})

	resp, err := s.List(ctx, &paginationV1.PagingRequest{
		NoPaging: trans.Ptr(true),
	})
	if err != nil {
		s.log.Errorf("获取任务列表失败[%s]", err.Error())
		return 0, err
	}

	s.log.Infof("开始开启定时任务，总计[%d]个", resp.GetTotal())

	// 重新启动任务
	var count int32
	for _, t := range resp.GetItems() {
		if s.startTask(t) != nil {
			continue
		} else {
			count++
		}
	}

	s.log.Infof("总共成功开启定时任务[%d]个", count)

	return count, nil
}

// stopAllTask 停止所有的任务
func (s *TaskService) stopAllTask() {
	s.log.Infof("开始清除所有的定时任务...")

	// 清除所有的定时任务
	s.taskScheduler.RemoveAllPeriodicTask()

	s.log.Infof("完成清除所有的定时任务")
}

// stopTask 停止一个任务
func (s *TaskService) stopTask(t *taskV1.Task) error {
	if t == nil {
		return errors.New("task is nil")
	}

	if t.GetEnable() == false {
		return errors.New("task is not enable")
	}

	// nil 调度器防护
	if s.taskScheduler == nil {
		return errors.New("task scheduler is not configured")
	}

	switch t.GetType() {
	case taskV1.Task_PERIODIC:
		return s.taskScheduler.RemovePeriodicTask(t.GetTypeName())

	// DELAY/WAIT_RESULT 是已投递到 asynq 队列的一次性任务，底层 transport
	// 未暴露 Inspector（删除排队任务/取消在途任务的能力），无法安全停止。
	// 此前空 case 静默返回成功，用户会以为已停止——必须诚实报错。
	case taskV1.Task_DELAY:
		return errors.New("queued one-shot task cannot be stopped, it will run at its scheduled time")
	case taskV1.Task_WAIT_RESULT:
		return errors.New("in-flight task cannot be stopped, please wait for it to finish")
	}

	return nil
}

// convertTaskOption 转换任务选项
func (s *TaskService) convertTaskOption(t *taskV1.Task) (opts []asynq.Option, payload any) {
	if t == nil {
		return
	}

	if len(t.GetTaskPayload()) > 0 {
		_ = json.Unmarshal([]byte(t.GetTaskPayload()), &payload)
	}

	// asynq 拒绝 nil message（"message is nil"）；任务数据留空时兜底为空对象
	if payload == nil {
		payload = map[string]any{}
	}

	if t.TaskOptions != nil {
		if t.GetTaskOptions().GetMaxRetry() > 0 {
			opts = append(opts, asynq.MaxRetry(int(t.GetTaskOptions().GetMaxRetry())))
		}
		if t.GetTaskOptions().Timeout != nil {
			opts = append(opts, asynq.Timeout(t.GetTaskOptions().GetTimeout().AsDuration()))
		}
		if t.GetTaskOptions().Deadline != nil {
			opts = append(opts, asynq.Deadline(t.GetTaskOptions().GetDeadline().AsTime()))
		}
		if t.GetTaskOptions().ProcessIn != nil {
			opts = append(opts, asynq.ProcessIn(t.GetTaskOptions().GetProcessIn().AsDuration()))
		}
		if t.GetTaskOptions().ProcessAt != nil {
			opts = append(opts, asynq.ProcessAt(t.GetTaskOptions().GetProcessAt().AsTime()))
		}
		if t.GetTaskOptions().UniqueTtl != nil {
			opts = append(opts, asynq.Unique(t.GetTaskOptions().GetUniqueTtl().AsDuration()))
		}
		if t.GetTaskOptions().Retention != nil {
			opts = append(opts, asynq.Retention(t.GetTaskOptions().GetRetention().AsDuration()))
		}
		if t.GetTaskOptions().Group != nil {
			opts = append(opts, asynq.Group(t.GetTaskOptions().GetGroup()))
		}
		if t.GetTaskOptions().TaskId != nil {
			opts = append(opts, asynq.TaskID(t.GetTaskOptions().GetTaskId()))
		}
	}

	return
}

// startTask 启动一个任务
func (s *TaskService) startTask(t *taskV1.Task) error {
	if t == nil {
		return errors.New("task is nil")
	}

	if t.GetEnable() == false {
		return errors.New("task is not enable")
	}

	// nil 调度器防护
	if s.taskScheduler == nil {
		return errors.New("task scheduler is not configured")
	}

	var opts []asynq.Option
	var payload any
	var err error

	switch t.GetType() {
	case taskV1.Task_PERIODIC:
		opts, payload = s.convertTaskOption(t)
		if _, err = s.taskScheduler.NewPeriodicTask(t.GetCronSpec(), t.GetTypeName(), payload, opts...); err != nil {
			s.log.Errorf("[%s] 创建定时任务失败[%s]", t.GetTypeName(), err.Error())
			return err
		}

	case taskV1.Task_DELAY:
		opts, payload = s.convertTaskOption(t)
		if err = s.taskScheduler.NewTask(t.GetTypeName(), payload, opts...); err != nil {
			s.log.Errorf("[%s] 创建延迟任务失败[%s]", t.GetTypeName(), err.Error())
			return err
		}

	case taskV1.Task_WAIT_RESULT:
		opts, payload = s.convertTaskOption(t)
		if err = s.taskScheduler.NewWaitResultTask(t.GetTypeName(), payload, opts...); err != nil {
			s.log.Errorf("[%s] 创建等待结果任务失败[%s]", t.GetTypeName(), err.Error())
			return err
		}
	}

	return nil
}

// AsyncBackup 异步备份
func (s *TaskService) AsyncBackup(taskType string, taskData *task.BackupTaskData) error {
	s.log.Infof("AsyncBackup [%s] [%+v] [%s]", taskType, taskData, taskData.Name)
	return nil
}

// EnqueueAiDocReindex 入队一个知识库切片重索引任务（baseID=0 全量）。
//
// best-effort 语义：调度器未注入（asynq server 未配置）时告警并返回 nil，
// 不阻塞 ReindexDocs RPC——重索引是运维性重建，不是用户显式的数据变更。
func (s *TaskService) EnqueueAiDocReindex(baseID int64) error {
	if s.taskScheduler == nil {
		s.log.Warnf("ai doc reindex skipped: task scheduler not available (base=%d)", baseID)
		return nil
	}
	if err := s.taskScheduler.NewTask(task.AiDocReindexTaskType, &task.AiDocReindexPayload{BaseID: baseID}); err != nil {
		s.log.Errorf("enqueue ai doc reindex failed (base=%d): %v", baseID, err)
		return err
	}
	s.log.Infof("enqueued ai doc reindex (base=%d)", baseID)
	return nil
}
