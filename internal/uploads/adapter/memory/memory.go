package memory

import (
	"context"
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

func (o *OutboxRepo) ListUnpublished(_ context.Context, limit int) ([]port.OutboxMessage, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	var out []port.OutboxMessage
	for _, m := range o.msgs {
		if o.published[m.ID] {
			continue
		}
		out = append(out, m)
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

func (o *OutboxRepo) MarkPublished(_ context.Context, id int64, _ time.Time) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.published[id] = true
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

func (s *Storage) Stat(_ context.Context, key string) (int64, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, ok := s.objs[key]
	if !ok {
		return 0, "", domain.ErrObjectMissing
	}
	return int64(len(b)), s.ct[key], nil
}

func (s *Storage) Put(key, contentType string, body []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.objs[key] = append([]byte(nil), body...)
	s.ct[key] = contentType
}

type AccessOK struct{}

func (AccessOK) EnsureMember(context.Context, uuid.UUID, uuid.UUID) error { return nil }

type AccessDeny struct{}

func (AccessDeny) EnsureMember(context.Context, uuid.UUID, uuid.UUID) error {
	return domain.ErrForbidden
}
