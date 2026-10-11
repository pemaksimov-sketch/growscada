package outbox

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/kipitix/growscada/internal/server/domain/event"
)

type policy struct {
	name      string
	eventType event.EventType
	handle    func(context.Context, event.Event) error
	wake      chan struct{}
	mu        sync.Mutex
}

// RegisterPolicy registers a reliable subscriber. Call only during startup,
// before Run or DrainOnce. Names are stable durable subscription identifiers:
// renaming one requires migrating its queued rows. Each subscriber is ordered,
// but a failed or slow subscriber cannot block SSE or another subscriber.
func (d *Dispatcher) RegisterPolicy(name string, eventType event.EventType, handle func(context.Context, event.Event) error) {
	if name == "" || !eventType.IsValid() || handle == nil {
		panic("outbox: invalid policy registration")
	}
	for _, p := range d.policies {
		if p.name == name {
			panic("outbox: duplicate policy " + name)
		}
	}
	d.policies = append(d.policies, &policy{name: name, eventType: eventType, handle: handle, wake: make(chan struct{}, 1)})
}

// enqueuePolicies runs before publishing a source event. Persisting these
// copies closes the crash window between removing a source row and processing
// its subscribers. Partial insertion is safe: the unique key deduplicates it.
func (d *Dispatcher) enqueuePolicies(ctx context.Context, row outboxRow, e event.Event) error {
	for _, p := range d.policies {
		if p.eventType != e.Type() {
			continue
		}
		_, err := d.db.ExecContext(ctx, `INSERT INTO policy_outbox (source_seq,subscriber,type,payload,project_id)
		 VALUES ($1,$2,$3,$4,$5) ON CONFLICT (source_seq,subscriber) DO NOTHING`, row.seq, p.name, row.eventType, string(row.payload), row.projectID)
		if err != nil {
			return fmt.Errorf("enqueue policy %s: %w", p.name, err)
		}
		select {
		case p.wake <- struct{}{}:
		default:
		}
	}
	return nil
}

func (d *Dispatcher) runPolicy(ctx context.Context, p *policy) {
	ticker := time.NewTicker(d.pollInterval)
	defer ticker.Stop()
	for {
		_, err := d.drainPolicy(ctx, p)
		if err != nil && ctx.Err() == nil {
			slog.Error("outbox policy failed; will retry", "policy", p.name, "error", err)
		}
		// On failure consume notifications but do not retry before the polling
		// interval: hot tag traffic must not turn a failing policy into a loop.
		if err != nil {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
			continue
		}
		select {
		case <-ctx.Done():
			return
		case <-p.wake:
		case <-ticker.C:
		}
	}
}

func (d *Dispatcher) drainPolicy(ctx context.Context, p *policy) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	done := 0
	// Keep acknowledgements until the source has disappeared. A crash after
	// handle but before acknowledgement can repeat handle, hence idempotence.
	_, err := d.db.ExecContext(ctx, `DELETE FROM policy_outbox p WHERE subscriber=$1 AND completed
	 AND NOT EXISTS (SELECT 1 FROM outbox o WHERE o.seq=p.source_seq)`, p.name)
	if err != nil {
		return done, err
	}
	for {
		rows, err := d.db.QueryContext(ctx, `SELECT seq,type,payload,project_id FROM policy_outbox
		 WHERE subscriber=$1 AND NOT completed ORDER BY seq LIMIT $2`, p.name, batchSize)
		if err != nil {
			return done, err
		}
		batch := []outboxRow{}
		for rows.Next() {
			var row outboxRow
			if err = rows.Scan(&row.seq, &row.eventType, &row.payload, &row.projectID); err != nil {
				break
			}
			batch = append(batch, row)
		}
		if err == nil {
			err = rows.Err()
		}
		rows.Close()
		if err != nil {
			return done, err
		}
		for _, row := range batch {
			e, err := row.decode()
			if err != nil {
				return done, fmt.Errorf("decode policy job %d: %w", row.seq, err)
			}
			if err = p.handle(ctx, e); err != nil {
				return done, err
			}
			if _, err = d.db.ExecContext(ctx, `UPDATE policy_outbox SET completed=true WHERE seq=$1`, row.seq); err != nil {
				return done, err
			}
			done++
		}
		if len(batch) < batchSize {
			return done, nil
		}
	}
}
