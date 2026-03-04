package services

import (
	"gpt-load/internal/models"
	app_errors "gpt-load/internal/errors"

	"github.com/sirupsen/logrus"
)

const (
	deleteChunkSize = 1000
)

// KeyDeleteResult 存储删除任务的结果。
type KeyDeleteResult struct {
	DeletedCount int `json:"deleted_count"`
	IgnoredCount int `json:"ignored_count"`
}

// KeyDeleteService 处理大量密钥的异步删除。
type KeyDeleteService struct {
	TaskService *TaskService
	KeyService  *KeyService
}

// NewKeyDeleteService 创建一个新的 KeyDeleteService。
func NewKeyDeleteService(taskService *TaskService, keyService *KeyService) *KeyDeleteService {
	return &KeyDeleteService{
		TaskService: taskService,
		KeyService:  keyService,
	}
}

// StartDeleteTask 启动一个新的异步密钥删除任务。
func (s *KeyDeleteService) StartDeleteTask(group *models.Group, keysText string) (*TaskStatus, error) {
	keys := s.KeyService.ParseKeysFromText(keysText)
	if len(keys) == 0 {
		return nil, app_errors.NewServiceError(app_errors.ErrNoValidKeysFound, "no valid keys found in the input text")
	}

	initialStatus, err := s.TaskService.StartTask(TaskTypeKeyDelete, group.Name, len(keys))
	if err != nil {
		return nil, err
	}

	go s.runDelete(group, keys)

	return initialStatus, nil
}

func (s *KeyDeleteService) runDelete(group *models.Group, keys []string) {
	progressCallback := func(processed int) {
		if err := s.TaskService.UpdateProgress(processed); err != nil {
			logrus.Warnf("Failed to update task progress for group %d: %v", group.ID, err)
		}
	}

	deletedCount, ignoredCount, err := s.processAndDeleteKeys(group.ID, keys, progressCallback)
	if err != nil {
		if endErr := s.TaskService.EndTask(nil, err); endErr != nil {
			logrus.Errorf("Failed to end task with error for group %d: %v (original error: %v)", group.ID, endErr, err)
		}
		return
	}

	result := KeyDeleteResult{
		DeletedCount: deletedCount,
		IgnoredCount: ignoredCount,
	}

	if endErr := s.TaskService.EndTask(result, nil); endErr != nil {
		logrus.Errorf("Failed to end task with success result for group %d: %v", group.ID, endErr)
	}
}

// processAndDeleteKeys 是带进度跟踪的删除密钥的核心函数。
func (s *KeyDeleteService) processAndDeleteKeys(
	groupID uint,
	keys []string,
	progressCallback func(processed int),
) (deletedCount int, ignoredCount int, err error) {
	var totalDeletedCount int64

	for i := 0; i < len(keys); i += deleteChunkSize {
		end := i + deleteChunkSize
		if end > len(keys) {
			end = len(keys)
		}
		chunk := keys[i:end]

		deletedChunkCount, err := s.KeyService.KeyProvider.RemoveKeys(groupID, chunk)
		if err != nil {
			return int(totalDeletedCount), len(keys) - int(totalDeletedCount), err
		}

		totalDeletedCount += deletedChunkCount

		if progressCallback != nil {
			progressCallback(i + len(chunk))
		}
	}

	return int(totalDeletedCount), len(keys) - int(totalDeletedCount), nil
}
