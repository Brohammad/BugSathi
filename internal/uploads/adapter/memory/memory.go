package memory

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Brohammad/BugSathi/internal/uploads/domain"
	"github.com/Brohammad/BugSathi/internal/uploads/port"
	"github.com/google/uuid"
)

type OutboxRepo struct {
	mu        sync.Mutex
	msgs      []port.OutboxMessage
	published map[int64]bool
}

func NewOutboxRepo() *OutboxRepo {
	return &OutboxRepo{published: map[int64]bool{}}
}

func (o *OutboxRepo) add(m port.OutboxMessage) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.msgs = append(o.msgs, m)
}

func (o *OutboxRepo) WithClaimed(ctx context.Context, limit int, fn func(context.Context, []port.OutboxMessage) error) error {
	if limit <= 0 {
		return nil
	}
	// Hold the mutex for the whole claim+fn+mark cycle so concurrent Flush
	// callers (simulating API+worker relays) cannot double-claim a row.
	o.mu.Lock()
	defer o.mu.Unlock()

	var claimed []port.OutboxMessage
	for _, m := range o.msgs {
		if o.published[m.ID] {
			continue
		}
		claimed = append(claimed, m)
		if len(claimed) >= limit {
			break
		}
	}
	if len(claimed) == 0 {
		return nil
	}

	if err := fn(ctx, claimed); err != nil {
		return err
	}
	for _, m := range claimed {
		o.published[m.ID] = true
	}
	return nil
}

func (o *OutboxRepo) Messages() []port.OutboxMessage {
	o.mu.Lock()
	defer o.mu.Unlock()
	return append([]port.OutboxMessage(nil), o.msgs...)
}

type RecordingRepo struct {
	mu         sync.Mutex
	byID       map[uuid.UUID]domain.Recording
	nextOutbox int64
	outbox     *OutboxRepo
}

func NewRecordingRepo(outbox *OutboxRepo) *RecordingRepo {
	return &RecordingRepo{byID: make(map[uuid.UUID]domain.Recording), outbox: outbox}
}

func (r *RecordingRepo) Create(_ context.Context, rec domain.Recording) (domain.Recording, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.byID[rec.ID] = rec
	return rec, nil
}

func (r *RecordingRepo) Get(_ context.Context, id uuid.UUID) (domain.Recording, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	rec, ok := r.byID[id]
	if !ok {
		return domain.Recording{}, domain.ErrNotFound
	}
	return rec, nil
}

func (r *RecordingRepo) Update(_ context.Context, rec domain.Recording) (domain.Recording, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.byID[rec.ID]; !ok {
		return domain.Recording{}, domain.ErrNotFound
	}
	r.byID[rec.ID] = rec
	return rec, nil
}

func (r *RecordingRepo) CompleteWithOutbox(
	_ context.Context,
	rec domain.Recording,
	eventTopic, partitionKey string,
	payload []byte,
	correlationID string,
) (domain.Recording, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	cur, ok := r.byID[rec.ID]
	if !ok {
		return domain.Recording{}, domain.ErrNotFound
	}
	if cur.Status != domain.StatusUploading {
		return cur, nil
	}
	r.byID[rec.ID] = rec
	id := atomic.AddInt64(&r.nextOutbox, 1)
	r.outbox.add(port.OutboxMessage{
		ID: id, Topic: eventTopic, PartitionKey: partitionKey,
		Payload: payload, CorrelationID: correlationID, CreatedAt: time.Now(),
	})
	return rec, nil
}

func (r *RecordingRepo) InsertOutbox(
	_ context.Context,
	eventTopic, partitionKey string,
	payload []byte,
	correlationID string,
) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	id := atomic.AddInt64(&r.nextOutbox, 1)
	r.outbox.add(port.OutboxMessage{
		ID: id, Topic: eventTopic, PartitionKey: partitionKey,
		Payload: payload, CorrelationID: correlationID, CreatedAt: time.Now(),
	})
	return nil
}

type Storage struct {
	mu   sync.Mutex
	objs map[string][]byte
	ct   map[string]string
}

func NewStorage() *Storage {
	return &Storage{objs: map[string][]byte{}, ct: map[string]string{}}
}

func (s *Storage) PresignPut(_ context.Context, key, contentType string, _ time.Duration) (string, error) {
	return "memory://upload/" + key + "?ct=" + contentType, nil
}

func (s *Storage) Stat(_ context.Context, key string) (port.ObjectMeta, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, ok := s.objs[key]
	if !ok {
		return port.ObjectMeta{}, domain.ErrObjectMissing
	}
	return port.ObjectMeta{
		Size:        int64(len(b)),
		ContentType: s.ct[key],
		ETag:        fmt.Sprintf("%x", len(b)),
	}, nil
}

func (s *Storage) Put(key, contentType string, body []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.objs[key] = append([]byte(nil), body...)
	s.ct[key] = contentType
}

func (s *Storage) Has(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.objs[key]
	return ok
}

func (s *Storage) Keys() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, 0, len(s.objs))
	for k := range s.objs {
		out = append(out, k)
	}
	return out
}

func (s *Storage) Delete(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.objs, key)
	delete(s.ct, key)
	return nil
}

func (s *Storage) DeletePrefix(_ context.Context, prefix string) error {
	if prefix == "" {
		return fmt.Errorf("refusing to delete with empty prefix")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for k := range s.objs {
		if strings.HasPrefix(k, prefix) {
			delete(s.objs, k)
			delete(s.ct, k)
		}
	}
	return nil
}

type AccessOK struct{}

func (AccessOK) EnsureMember(context.Context, uuid.UUID, uuid.UUID) error { return nil }
func (AccessOK) EnsureOwner(context.Context, uuid.UUID, uuid.UUID) error  { return nil }

type AccessDeny struct{}

func (AccessDeny) EnsureMember(context.Context, uuid.UUID, uuid.UUID) error {
	return domain.ErrForbidden
}
func (AccessDeny) EnsureOwner(context.Context, uuid.UUID, uuid.UUID) error {
	return domain.ErrForbidden
}

type AccessMemberOnly struct{}

func (AccessMemberOnly) EnsureMember(context.Context, uuid.UUID, uuid.UUID) error { return nil }
func (AccessMemberOnly) EnsureOwner(context.Context, uuid.UUID, uuid.UUID) error {
	return domain.ErrForbidden
}
