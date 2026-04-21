package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/chenqilscy/ripple/backend-go/internal/domain"
	"github.com/chenqilscy/ripple/backend-go/internal/store"
	"github.com/rs/zerolog"
)

// OutboxDispatcher 周期轮询 outbox，把事件投递到目标存储。
//
// 当前支持事件：LakeCreated → lakes.Create(Neo4j)。
//
// 投递语义：至少一次。处理幂等（Neo4j 约束冲突视为已投递）。
type OutboxDispatcher struct {
	outbox store.OutboxRepository
	lakes  store.LakeRepository
	log    zerolog.Logger

	// 可调参数
	Interval time.Duration
	Batch    int
}

// NewOutboxDispatcher 装配。
func NewOutboxDispatcher(
	outbox store.OutboxRepository,
	lakes store.LakeRepository,
	log zerolog.Logger,
) *OutboxDispatcher {
	return &OutboxDispatcher{
		outbox:   outbox,
		lakes:    lakes,
		log:      log,
		Interval: 2 * time.Second,
		Batch:    10,
	}
}

// Run 阻塞直到 ctx 取消。由调用方开 goroutine。
func (d *OutboxDispatcher) Run(ctx context.Context) {
	t := time.NewTicker(d.Interval)
	defer t.Stop()
	d.log.Info().Dur("interval", d.Interval).Msg("outbox dispatcher started")
	for {
		select {
		case <-ctx.Done():
			d.log.Info().Msg("outbox dispatcher stopped")
			return
		case <-t.C:
			d.tick(ctx)
		}
	}
}

func (d *OutboxDispatcher) tick(ctx context.Context) {
	evs, err := d.outbox.Dequeue(ctx, d.Batch)
	if err != nil {
		d.log.Error().Err(err).Msg("outbox dequeue failed")
		return
	}
	for _, ev := range evs {
		if err := d.handle(ctx, ev); err != nil {
			d.log.Error().Int64("id", ev.ID).Str("type", ev.EventType).
				Err(err).Msg("outbox handle failed")
			_ = d.outbox.MarkFailed(ctx, ev.ID, err.Error())
			continue
		}
		if err := d.outbox.MarkDone(ctx, ev.ID); err != nil {
			d.log.Error().Int64("id", ev.ID).Err(err).Msg("outbox mark done failed")
		}
	}
}

func (d *OutboxDispatcher) handle(ctx context.Context, ev store.OutboxEvent) error {
	switch ev.EventType {
	case OutboxEventLakeCreated:
		var l domain.Lake
		if err := json.Unmarshal(ev.Payload, &l); err != nil {
			return fmt.Errorf("unmarshal: %w", err)
		}
		return d.lakes.Create(ctx, &l)
	default:
		return fmt.Errorf("unknown event type: %s", ev.EventType)
	}
}
