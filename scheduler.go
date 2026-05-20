package scheduler

import (
	"log"
	"sync"
	"time"

	"github.com/yourusername/go-backup/internal/backup"
)

// Schedule はスケジュール設定
type Schedule struct {
	Enabled bool   `json:"enabled"`
	Hour    int    `json:"hour"`
	Minute  int    `json:"minute"`
	Src     string `json:"src"`
	Dst     string `json:"dst"`
}

// Scheduler はスケジュール実行を管理する
type Scheduler struct {
	mu       sync.Mutex
	schedule Schedule
	onDone   func(backup.Result)
	stop     chan struct{}
}

// New はSchedulerを作成する
func New(onDone func(backup.Result)) *Scheduler {
	return &Scheduler{
		onDone: onDone,
		stop:   make(chan struct{}),
	}
}

// Set はスケジュールを設定する
func (s *Scheduler) Set(schedule Schedule) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.schedule = schedule
	log.Printf("スケジュール更新: 毎日 %02d:%02d (有効: %v)", schedule.Hour, schedule.Minute, schedule.Enabled)
}

// Get は現在のスケジュールを取得する
func (s *Scheduler) Get() Schedule {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.schedule
}

// Start はスケジューラーを起動する（goroutineで動く）
func (s *Scheduler) Start() {
	go func() {
		ticker := time.NewTicker(30 * time.Second) // 30秒ごとにチェック
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				s.mu.Lock()
				sch := s.schedule
				s.mu.Unlock()

				if !sch.Enabled || sch.Src == "" || sch.Dst == "" {
					continue
				}

				now := time.Now()
				if now.Hour() == sch.Hour && now.Minute() == sch.Minute {
					log.Printf("スケジュールバックアップ開始: %s → %s", sch.Src, sch.Dst)
					result := backup.Run(sch.Src, sch.Dst)
					if s.onDone != nil {
						s.onDone(result)
					}
				}

			case <-s.stop:
				return
			}
		}
	}()
}

// Stop はスケジューラーを停止する
func (s *Scheduler) Stop() {
	close(s.stop)
}
