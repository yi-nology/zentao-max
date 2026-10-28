package initialization

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"github.com/yi-nology/zentao-max/backend/internal/models"
)

const maxLogs = 100

type ConfigStore struct {
	filePath string
	mu       sync.RWMutex
}

func NewConfigStore(baseDir string) *ConfigStore {
	if baseDir == "" {
		homeDir, err := os.UserHomeDir()
		if err == nil {
			baseDir = filepath.Join(homeDir, ".zentao-max")
		} else {
			baseDir = "."
		}
	} else {
		baseDir = filepath.Dir(baseDir)
	}
	filePath := filepath.Join(baseDir, "cron.db")
	return &ConfigStore{filePath: filePath}
}

func (s *ConfigStore) load() (*models.CronDB, error) {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return &models.CronDB{Tasks: []models.SchedulerTask{}, Logs: []models.TaskExecutionLog{}}, nil
		}
		return nil, err
	}
	var db models.CronDB
	if err := json.Unmarshal(data, &db); err != nil {
		return nil, err
	}
	if db.Tasks == nil {
		db.Tasks = []models.SchedulerTask{}
	}
	if db.Logs == nil {
		db.Logs = []models.TaskExecutionLog{}
	}
	return &db, nil
}

func (s *ConfigStore) save(db *models.CronDB) error {
	dir := filepath.Dir(s.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(db, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.filePath, data, 0644)
}

func (s *ConfigStore) ListTasks() ([]models.SchedulerTask, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	db, err := s.load()
	if err != nil {
		return nil, err
	}
	return db.Tasks, nil
}

func (s *ConfigStore) GetTask(id string) (*models.SchedulerTask, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	db, err := s.load()
	if err != nil {
		return nil, err
	}
	for i := range db.Tasks {
		if db.Tasks[i].ID == id {
			return &db.Tasks[i], nil
		}
	}
	return nil, nil
}

func (s *ConfigStore) CreateTask(task *models.SchedulerTask) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	db, err := s.load()
	if err != nil {
		return err
	}
	db.Tasks = append(db.Tasks, *task)
	return s.save(db)
}

func (s *ConfigStore) UpdateTask(task *models.SchedulerTask) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	db, err := s.load()
	if err != nil {
		return err
	}
	for i := range db.Tasks {
		if db.Tasks[i].ID == task.ID {
			db.Tasks[i] = *task
			return s.save(db)
		}
	}
	return os.ErrNotExist
}

func (s *ConfigStore) DeleteTask(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	db, err := s.load()
	if err != nil {
		return err
	}
	for i := range db.Tasks {
		if db.Tasks[i].ID == id {
			db.Tasks = append(db.Tasks[:i], db.Tasks[i+1:]...)
			break
		}
	}
	var filtered []models.TaskExecutionLog
	for _, l := range db.Logs {
		if l.TaskID != id {
			filtered = append(filtered, l)
		}
	}
	db.Logs = filtered
	return s.save(db)
}

func (s *ConfigStore) SaveLog(log *models.TaskExecutionLog) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	db, err := s.load()
	if err != nil {
		return err
	}
	db.Logs = append(db.Logs, *log)
	if len(db.Logs) > maxLogs {
		db.Logs = db.Logs[len(db.Logs)-maxLogs:]
	}
	for i := range db.Tasks {
		if db.Tasks[i].ID == log.TaskID {
			db.Tasks[i].LastRunAt = &log.StartedAt
			db.Tasks[i].LastRunStatus = log.Status
			break
		}
	}
	return s.save(db)
}

func (s *ConfigStore) ListLogs(taskID string, limit int) ([]models.TaskExecutionLog, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	db, err := s.load()
	if err != nil {
		return nil, err
	}
	var result []models.TaskExecutionLog
	for i := len(db.Logs) - 1; i >= 0; i-- {
		if taskID == "" || db.Logs[i].TaskID == taskID {
			result = append(result, db.Logs[i])
			if limit > 0 && len(result) >= limit {
				break
			}
		}
	}
	if result == nil {
		result = []models.TaskExecutionLog{}
	}
	return result, nil
}
