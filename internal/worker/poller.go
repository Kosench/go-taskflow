package worker

import (
	"context"
	"sync"
	"time"

	"github.com/Kosench/go-taskflow/internal/domain"
	"github.com/Kosench/go-taskflow/internal/pkg/logger"
)

type PendingTaskSource interface {
	ListPending(ctx context.Context, limit int) ([]*domain.Task, error)
}

// Poller is the database fallback for missed Kafka publications and the
// scheduler for tasks whose scheduled_at has become due.
type Poller struct {
	source      PendingTaskSource
	processor   *Processor
	interval    time.Duration
	batchSize   int
	concurrency int
	logger      *logger.Logger
}

func NewPoller(source PendingTaskSource, processor *Processor, interval time.Duration, batchSize, concurrency int, log *logger.Logger) *Poller {
	if interval <= 0 {
		interval = time.Second
	}
	if batchSize <= 0 {
		batchSize = 10
	}
	if concurrency <= 0 {
		concurrency = 1
	}

	return &Poller{
		source:      source,
		processor:   processor,
		interval:    interval,
		batchSize:   batchSize,
		concurrency: concurrency,
		logger:      log,
	}
}

func (p *Poller) Run(ctx context.Context) {
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	p.poll(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.poll(ctx)
		}
	}
}

func (p *Poller) poll(ctx context.Context) {
	tasks, err := p.source.ListPending(ctx, p.batchSize)
	if err != nil {
		p.logger.Error().Err(err).Msg("failed to poll pending tasks")
		return
	}

	semaphore := make(chan struct{}, p.concurrency)
	var wg sync.WaitGroup
	for _, task := range tasks {
		if ctx.Err() != nil {
			break
		}

		semaphore <- struct{}{}
		wg.Add(1)
		go func(task *domain.Task) {
			defer wg.Done()
			defer func() { <-semaphore }()
			if err := p.processor.HandleTask(ctx, task); err != nil {
				p.logger.Error().Err(err).Str("task_id", task.ID).Msg("failed to process polled task")
			}
		}(task)
	}
	wg.Wait()
}
