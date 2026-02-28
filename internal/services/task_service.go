package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"gpt-load/internal/store"
	"time"
)

const (
	globalTaskKey = "global_task"
	ResultTTL     = 60 * time.Minute
)

const (
	TaskTypeKeyValidation = "KEY_VALIDATION"
	TaskTypeKeyImport     = "KEY_IMPORT"
	TaskTypeKeyDelete     = "KEY_DELETE"
)

// TaskStatus 表示长时间运行任务的完整生命周期
type TaskStatus struct {
	TaskType        string     `json:"task_type"`
	IsRunning       bool       `json:"is_running"`
	GroupName       string     `json:"group_name,omitempty"`
	Processed       int        `json:"processed"`
	Total           int        `json:"total"`
	Result          any        `json:"result,omitempty"`
	Error           string     `json:"error,omitempty"`
	StartedAt       time.Time  `json:"started_at"`
	FinishedAt      *time.Time `json:"finished_at,omitempty"`
	DurationSeconds float64    `json:"duration_seconds,omitempty"`
}

// TaskService 使用存储接口管理单个全局长时间运行任务的状态
type TaskService struct {
	store store.Store
}

// NewTaskService 创建新的 TaskService
func NewTaskService(store store.Store) *TaskService {
	return &TaskService{
		store: store,
	}
}

// StartTask 尝试启动新任务。如果任务已在运行则返回错误
func (s *TaskService) StartTask(taskType, groupName string, total int) (*TaskStatus, error) {
	currentStatus, err := s.GetTaskStatus()
	if err != nil {
		return nil, fmt.Errorf("failed to check current task status before starting a new one: %w", err)
	}

	if currentStatus.IsRunning {
		return nil, errors.New("a task is already running, please wait")
	}

	status := &TaskStatus{
		TaskType:  taskType,
		IsRunning: true,
		GroupName: groupName,
		Total:     total,
		Processed: 0,
		StartedAt: time.Now(),
	}
	statusBytes, err := json.Marshal(status)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize new task status: %w", err)
	}

	if err := s.store.Set(globalTaskKey, statusBytes, ResultTTL); err != nil {
		return nil, fmt.Errorf("failed to set initial task status: %w", err)
	}

	return status, nil
}

// GetTaskStatus 返回当前任务状态
func (s *TaskService) GetTaskStatus() (*TaskStatus, error) {
	statusBytes, err := s.store.Get(globalTaskKey)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return &TaskStatus{IsRunning: false}, nil
		}
		return nil, fmt.Errorf("failed to get task status: %w", err)
	}

	var status TaskStatus
	if err := json.Unmarshal(statusBytes, &status); err != nil {
		return nil, fmt.Errorf("failed to deserialize task status: %w", err)
	}

	if !status.IsRunning && status.FinishedAt != nil {
		status.DurationSeconds = status.FinishedAt.Sub(status.StartedAt).Seconds()
	}

	return &status, nil
}

// UpdateProgress 更新当前任务的进度
func (s *TaskService) UpdateProgress(processed int) error {
	status, err := s.GetTaskStatus()
	if err != nil {
		return err
	}
	if !status.IsRunning {
		return nil
	}

	status.Processed = processed
	statusBytes, err := json.Marshal(status)
	if err != nil {
		return fmt.Errorf("failed to serialize updated status: %w", err)
	}

	return s.store.Set(globalTaskKey, statusBytes, ResultTTL)
}

// EndTask 标记当前任务为已完成并存储最终结果
func (s *TaskService) EndTask(resultData any, taskErr error) error {
	status, err := s.GetTaskStatus()
	if err != nil {
		return fmt.Errorf("failed to get task object to end task: %w", err)
	}
	if !status.IsRunning {
		return nil
	}

	now := time.Now()
	status.IsRunning = false
	status.FinishedAt = &now
	status.DurationSeconds = now.Sub(status.StartedAt).Seconds()
	if taskErr != nil {
		status.Error = taskErr.Error()
	} else {
		status.Result = resultData
	}

	updatedTaskBytes, err := json.Marshal(status)
	if err != nil {
		return fmt.Errorf("failed to serialize final task status: %w", err)
	}

	return s.store.Set(globalTaskKey, updatedTaskBytes, ResultTTL)
}
