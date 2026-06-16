package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Job struct {
	Schedule    string `json:"schedule"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Task        string `json:"task,omitempty"`
}

type CronParts struct {
	Minute []int
	Hour   []int
	DOM    []int
	Month  []int
	DOW    []int
}

type LLMClient interface {
	Chat(ctx context.Context, task string, onMessage func(string)) (string, error)
}

type MessageSender func(chatId, text string)

type Scheduler struct {
	dir         string
	client      LLMClient
	send        MessageSender
	chatId      string
	lastRuns    map[string]time.Time
	mu          sync.Mutex
	currentTask string // name of the currently active task, empty if none
	taskMu      sync.RWMutex
}

func NewScheduler(dir string, client LLMClient, send MessageSender, chatId string) *Scheduler {
	return &Scheduler{
		dir:      dir,
		client:   client,
		send:     send,
		chatId:   chatId,
		lastRuns: make(map[string]time.Time),
	}
}

func (s *Scheduler) Start() {
	os.MkdirAll(s.dir, 0755)
	log.Printf("[scheduler] Watching directory: %s", s.dir)
	go s.loop()
}

func (s *Scheduler) loop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	s.checkAndRun()

	for range ticker.C {
		s.checkAndRun()
	}
}

func (s *Scheduler) GetCurrentTask() string {
	s.taskMu.RLock()
	defer s.taskMu.RUnlock()
	return s.currentTask
}

func (s *Scheduler) EndTask() {
	s.taskMu.Lock()
	defer s.taskMu.Unlock()
	if s.currentTask != "" {
		log.Printf("[scheduler] Ending task %q", s.currentTask)
		s.currentTask = ""
	}
}

func (s *Scheduler) setTask(name string) {
	s.taskMu.Lock()
	defer s.taskMu.Unlock()
	s.currentTask = name
}

func (s *Scheduler) checkAndRun() {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return
	}

	now := time.Now()

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		path := filepath.Join(s.dir, entry.Name())
		job, err := loadJob(path)
		if err != nil {
			log.Printf("[scheduler] %s: %v", entry.Name(), err)
			continue
		}

		cron, err := parseCron(job.Schedule)
		if err != nil {
			log.Printf("[scheduler] %s: invalid schedule %q: %v", entry.Name(), job.Schedule, err)
			continue
		}

		if !matchesCron(cron, now) {
			continue
		}

		jobName := strings.TrimSuffix(entry.Name(), ".json")

		s.mu.Lock()
		lastRun, exists := s.lastRuns[jobName]
		s.mu.Unlock()

		if exists && now.Sub(lastRun) < time.Minute {
			continue
		}

		log.Printf("[scheduler] Running task %q (%s)", jobName, job.Schedule)

		if job.Task != "" && s.client != nil {
			// Don't start a new task if one is already running
			if s.GetCurrentTask() != "" {
				log.Printf("[scheduler] Task %q already active, skipping job %q", s.GetCurrentTask(), jobName)
				continue
			}

			displayName := jobName
			if job.Name != "" {
				displayName = job.Name
			}
			s.setTask(displayName)

			go func(name, prompt string) {
				defer s.EndTask()

				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
				defer cancel()

				log.Printf("[scheduler] Sending LLM request for task %q", name)

				// Send each message from the LLM as it comes
				msgCount := 0
				result, err := s.client.Chat(ctx, prompt, func(msg string) {
					msgCount++
					var full string
					if msgCount == 1 {
						full = fmt.Sprintf("📋 Task %q:\n%s", name, msg)
					} else {
						full = fmt.Sprintf("📋 Task %q (continued):\n%s", name, msg)
					}
					if s.send != nil {
						s.send(s.chatId, full)
					}
				})
				if err != nil {
					errMsg := fmt.Sprintf("⚠️ Task %q failed: %v", name, err)
					log.Printf("[scheduler] %s", errMsg)
					if s.send != nil {
						s.send(s.chatId, errMsg)
					}
				} else {
					log.Printf("[scheduler] Task %q completed", name)
					if s.send != nil {
						s.send(s.chatId, fmt.Sprintf("✅ Task %q completed.", name))
					}
					_ = result
				}
			}(displayName, job.Task)
		}

		s.mu.Lock()
		s.lastRuns[jobName] = now
		s.mu.Unlock()
	}
}

func loadJob(path string) (Job, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Job{}, err
	}

	var job Job
	if err := json.Unmarshal(data, &job); err != nil {
		return Job{}, fmt.Errorf("invalid JSON: %w", err)
	}

	if job.Schedule == "" {
		return Job{}, fmt.Errorf("missing 'schedule' key")
	}

	return job, nil
}

func parseCron(schedule string) (CronParts, error) {
	parts := strings.Fields(schedule)
	if len(parts) != 5 {
		return CronParts{}, fmt.Errorf("expected 5 fields, got %d", len(parts))
	}

	minute, err := parseField(parts[0], 0, 59)
	if err != nil {
		return CronParts{}, fmt.Errorf("minute: %w", err)
	}
	hour, err := parseField(parts[1], 0, 23)
	if err != nil {
		return CronParts{}, fmt.Errorf("hour: %w", err)
	}
	dom, err := parseField(parts[2], 1, 31)
	if err != nil {
		return CronParts{}, fmt.Errorf("day of month: %w", err)
	}
	month, err := parseField(parts[3], 1, 12)
	if err != nil {
		return CronParts{}, fmt.Errorf("month: %w", err)
	}
	dow, err := parseField(parts[4], 0, 7)
	if err != nil {
		return CronParts{}, fmt.Errorf("day of week: %w", err)
	}

	return CronParts{Minute: minute, Hour: hour, DOM: dom, Month: month, DOW: dow}, nil
}

func parseField(field string, min, max int) ([]int, error) {
	if field == "*" {
		result := make([]int, max-min+1)
		for i := min; i <= max; i++ {
			result[i-min] = i
		}
		return result, nil
	}

	if strings.HasPrefix(field, "*/") {
		step, err := strconv.Atoi(field[2:])
		if err != nil || step <= 0 {
			return nil, fmt.Errorf("invalid step: %q", field)
		}
		var result []int
		for i := min; i <= max; i += step {
			result = append(result, i)
		}
		return result, nil
	}

	val, err := strconv.Atoi(field)
	if err != nil {
		return nil, fmt.Errorf("invalid value: %q", field)
	}
	if val < min || val > max {
		return nil, fmt.Errorf("value %d out of range [%d,%d]", val, min, max)
	}
	return []int{val}, nil
}

func matchesCron(c CronParts, t time.Time) bool {
	if !intInSlice(int(t.Month()), c.Month) {
		return false
	}
	if !intInSlice(t.Day(), c.DOM) {
		return false
	}
	if !intInSlice(t.Hour(), c.Hour) {
		return false
	}
	if !intInSlice(t.Minute(), c.Minute) {
		return false
	}
	dow := int(t.Weekday())
	if !intInSlice(dow, c.DOW) && !intInSlice(7, c.DOW) {
		return false
	}
	return true
}

func intInSlice(n int, slice []int) bool {
	for _, v := range slice {
		if v == n {
			return true
		}
	}
	return false
}
