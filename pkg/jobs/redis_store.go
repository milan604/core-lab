package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	redis "github.com/redis/go-redis/v9"

	"github.com/milan604/core-lab/pkg/apperr"
)

const redisListBatchSize = int64(250)

type RedisStoreConfig struct {
	Address      string
	Password     string
	DB           int
	Namespace    string
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

// RedisStore persists jobs in Redis so multiple services can share one job namespace.
type RedisStore struct {
	client     redis.UniversalClient
	namespace  string
	ownsClient bool
}

var claimReadyScript = redis.NewScript(`
local ids = redis.call('ZRANGEBYSCORE', KEYS[1], '-inf', ARGV[1], 'LIMIT', 0, ARGV[2])
local claimed = {}
local queueFilter = {}
local typeFilter = {}
local queueFilterEnabled = tonumber(ARGV[4]) == 1
local typeFilterEnabled = tonumber(ARGV[5]) == 1
local index = 6

if queueFilterEnabled then
  local queueCount = tonumber(ARGV[index]) or 0
  index = index + 1
  for i = 1, queueCount do
    queueFilter[ARGV[index]] = true
    index = index + 1
  end
end

if typeFilterEnabled then
  local typeCount = tonumber(ARGV[index]) or 0
  index = index + 1
  for i = 1, typeCount do
    typeFilter[ARGV[index]] = true
    index = index + 1
  end
end

for _, id in ipairs(ids) do
  local raw = redis.call('GET', KEYS[2] .. id)
  if raw then
    local job = cjson.decode(raw)
    if job["status"] == "queued" or job["status"] == "scheduled" then
      if queueFilterEnabled and not queueFilter[job["queue"]] then
        goto continue
      end
      if typeFilterEnabled and not typeFilter[job["type"]] then
        goto continue
      end
      job["status"] = "running"
      job["attempt"] = (job["attempt"] or 0) + 1
      job["started_at"] = ARGV[3]
      job["updated_at"] = ARGV[3]
      local updated = cjson.encode(job)
      redis.call('SET', KEYS[2] .. id, updated)
      redis.call('ZREM', KEYS[1], id)
      table.insert(claimed, updated)
    else
      redis.call('ZREM', KEYS[1], id)
    end
  else
    redis.call('ZREM', KEYS[1], id)
  end
  ::continue::
end

return claimed
`)

var pruneTerminalScript = redis.NewScript(`
local ids = redis.call('ZRANGEBYSCORE', KEYS[1], '-inf', ARGV[1])
local removed = 0

for _, id in ipairs(ids) do
  redis.call('DEL', KEYS[2] .. id)
  redis.call('ZREM', KEYS[3], id)
  redis.call('ZREM', KEYS[4], id)
  redis.call('ZREM', KEYS[1], id)
  removed = removed + 1
end

return removed
`)

func NewRedisStore(client redis.UniversalClient, namespace string) (*RedisStore, error) {
	if client == nil {
		return nil, apperr.New(apperr.ErrorCodeInvalidInput).
			WithMessage("redis client is required")
	}

	return &RedisStore{
		client:    client,
		namespace: normalizeRedisNamespace(namespace),
	}, nil
}

func NewRedisStoreFromConfig(ctx context.Context, cfg RedisStoreConfig) (*RedisStore, error) {
	address := strings.TrimSpace(cfg.Address)
	if address == "" {
		return nil, apperr.New(apperr.ErrorCodeInvalidInput).
			WithMessage("redis address is required")
	}

	client := redis.NewClient(&redis.Options{
		Addr:         address,
		Password:     cfg.Password,
		DB:           cfg.DB,
		DialTimeout:  normalizeRedisTimeout(cfg.DialTimeout, 3*time.Second),
		ReadTimeout:  normalizeRedisTimeout(cfg.ReadTimeout, 3*time.Second),
		WriteTimeout: normalizeRedisTimeout(cfg.WriteTimeout, 3*time.Second),
	})

	pingCtx := ctx
	if pingCtx == nil {
		pingCtx = context.Background()
	}
	pingCtx, cancel := context.WithTimeout(pingCtx, normalizeRedisTimeout(cfg.DialTimeout, 3*time.Second))
	defer cancel()

	if err := client.Ping(pingCtx).Err(); err != nil {
		_ = client.Close()
		return nil, err
	}

	return &RedisStore{
		client:     client,
		namespace:  normalizeRedisNamespace(cfg.Namespace),
		ownsClient: true,
	}, nil
}

func (s *RedisStore) Close() error {
	if s == nil || !s.ownsClient || s.client == nil {
		return nil
	}
	return s.client.Close()
}

func (s *RedisStore) Create(ctx context.Context, job Job) (Job, error) {
	key := s.jobKey(job.ID)
	job.CreatedAt = job.CreatedAt.UTC()
	job.UpdatedAt = job.UpdatedAt.UTC()
	job.AvailableAt = job.AvailableAt.UTC()
	if job.StartedAt != nil {
		startedAt := job.StartedAt.UTC()
		job.StartedAt = &startedAt
	}
	if job.CompletedAt != nil {
		completedAt := job.CompletedAt.UTC()
		job.CompletedAt = &completedAt
	}

	payload, err := json.Marshal(job)
	if err != nil {
		return Job{}, err
	}

	for attempt := 0; attempt < 5; attempt++ {
		err = s.client.Watch(ctx, func(tx *redis.Tx) error {
			exists, err := tx.Exists(ctx, key).Result()
			if err != nil {
				return err
			}
			if exists > 0 {
				return apperr.New(apperr.ErrorCodeInvalidInput).
					WithMessage("job id already exists").
					AddSuggestion("id", "provide a unique job id")
			}

			_, err = tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
				pipe.Set(ctx, key, payload, 0)
				pipe.ZAdd(ctx, s.createdIndexKey(), redis.Z{
					Score:  timeScore(job.CreatedAt),
					Member: job.ID,
				})
				s.syncIndexes(ctx, pipe, job)
				return nil
			})
			return err
		}, key)
		if err == redis.TxFailedErr {
			continue
		}
		if err != nil {
			return Job{}, err
		}
		return job.clone(), nil
	}

	return Job{}, apperr.New(apperr.ErrorCodeInternal).
		WithMessage("failed to create job due to concurrent updates")
}

func (s *RedisStore) Get(ctx context.Context, id string) (Job, bool, error) {
	raw, err := s.client.Get(ctx, s.jobKey(id)).Bytes()
	if err == redis.Nil {
		return Job{}, false, nil
	}
	if err != nil {
		return Job{}, false, err
	}

	job, err := decodeRedisJob(raw)
	if err != nil {
		return Job{}, false, err
	}
	return job, true, nil
}

func (s *RedisStore) List(ctx context.Context, filter JobFilter) ([]Job, error) {
	result := make([]Job, 0)
	skipped := 0
	limit := filter.Limit
	if limit <= 0 {
		limit = 100
	}

	statuses := make(map[Status]struct{}, len(filter.Statuses))
	for _, status := range filter.Statuses {
		statuses[status] = struct{}{}
	}

	for start := int64(0); ; start += redisListBatchSize {
		ids, err := s.client.ZRevRange(ctx, s.createdIndexKey(), start, start+redisListBatchSize-1).Result()
		if err != nil {
			return nil, err
		}
		if len(ids) == 0 {
			break
		}

		jobs, err := s.loadJobsByIDs(ctx, ids)
		if err != nil {
			return nil, err
		}

		for _, job := range jobs {
			if filter.Queue != "" && job.Queue != filter.Queue {
				continue
			}
			if filter.Type != "" && job.Type != filter.Type {
				continue
			}
			if len(statuses) > 0 {
				if _, ok := statuses[job.Status]; !ok {
					continue
				}
			}

			if skipped < filter.Offset {
				skipped++
				continue
			}

			result = append(result, job)
			if len(result) >= limit {
				return result, nil
			}
		}

		if len(ids) < int(redisListBatchSize) {
			break
		}
	}

	return result, nil
}

func (s *RedisStore) ClaimReady(ctx context.Context, now time.Time, limit int, filter ClaimFilter) ([]Job, error) {
	if limit <= 0 {
		return nil, nil
	}

	args := []any{
		timeScore(now.UTC()),
		limit,
		now.UTC().Format(time.RFC3339Nano),
		0,
		0,
	}

	if len(filter.Queues) > 0 {
		args[3] = 1
		args = append(args, len(filter.Queues))
		for _, queue := range filter.Queues {
			args = append(args, queue)
		}
	}

	if len(filter.Types) > 0 {
		args[4] = 1
		args = append(args, len(filter.Types))
		for _, jobType := range filter.Types {
			args = append(args, jobType)
		}
	}

	claimedRaw, err := claimReadyScript.Run(ctx, s.client, []string{
		s.availableIndexKey(),
		s.jobPrefix(),
	}, args...).Result()
	if err != nil {
		return nil, err
	}

	entries, ok := claimedRaw.([]any)
	if !ok {
		return nil, fmt.Errorf("unexpected redis claim response type %T", claimedRaw)
	}

	claimed := make([]Job, 0, len(entries))
	for _, entry := range entries {
		raw, ok := entry.(string)
		if !ok {
			continue
		}
		job, err := decodeRedisJob([]byte(raw))
		if err != nil {
			return nil, err
		}
		claimed = append(claimed, job)
	}

	return claimed, nil
}

func (s *RedisStore) MarkSucceeded(ctx context.Context, id string, result jsonRawResult, now time.Time) (Job, error) {
	return s.updateJob(ctx, id, func(job *Job) error {
		completedAt := now.UTC()
		job.Status = StatusSucceeded
		job.Result = append(jsonRawResult(nil), result...)
		job.LastError = ""
		job.CompletedAt = &completedAt
		job.UpdatedAt = completedAt
		return nil
	})
}

func (s *RedisStore) MarkRetry(ctx context.Context, id, reason string, runAt, now time.Time) (Job, error) {
	return s.updateJob(ctx, id, func(job *Job) error {
		job.Status = StatusScheduled
		job.LastError = reason
		job.AvailableAt = runAt.UTC()
		job.CompletedAt = nil
		job.UpdatedAt = now.UTC()
		return nil
	})
}

func (s *RedisStore) MarkFailed(ctx context.Context, id, reason string, now time.Time) (Job, error) {
	return s.updateJob(ctx, id, func(job *Job) error {
		completedAt := now.UTC()
		job.Status = StatusFailed
		job.LastError = reason
		job.CompletedAt = &completedAt
		job.UpdatedAt = completedAt
		return nil
	})
}

func (s *RedisStore) Cancel(ctx context.Context, id string, now time.Time) (Job, error) {
	return s.updateJob(ctx, id, func(job *Job) error {
		if job.Status == StatusRunning {
			return apperr.New(apperr.ErrorCodeInvalidInput).
				WithMessage("running jobs cannot be canceled")
		}
		if job.Status.IsTerminal() {
			return apperr.New(apperr.ErrorCodeInvalidInput).
				WithMessage("job is already in a terminal state")
		}

		completedAt := now.UTC()
		job.Status = StatusCanceled
		job.CompletedAt = &completedAt
		job.UpdatedAt = completedAt
		return nil
	})
}

func (s *RedisStore) Retry(ctx context.Context, id string, now time.Time) (Job, error) {
	return s.updateJob(ctx, id, func(job *Job) error {
		if !(job.Status == StatusFailed || job.Status == StatusCanceled) {
			return apperr.New(apperr.ErrorCodeInvalidInput).
				WithMessage("only failed or canceled jobs can be retried")
		}

		job.Status = StatusQueued
		job.Attempt = 0
		job.LastError = ""
		job.Result = nil
		job.AvailableAt = now.UTC()
		job.StartedAt = nil
		job.CompletedAt = nil
		job.UpdatedAt = now.UTC()
		return nil
	})
}

func (s *RedisStore) PruneTerminalBefore(ctx context.Context, cutoff time.Time) (int, error) {
	removed, err := pruneTerminalScript.Run(ctx, s.client, []string{
		s.terminalIndexKey(),
		s.jobPrefix(),
		s.createdIndexKey(),
		s.availableIndexKey(),
	}, timeScore(cutoff.UTC())).Int()
	if err == redis.Nil {
		return 0, nil
	}
	return removed, err
}

func (s *RedisStore) Stats(ctx context.Context) (storeStats, error) {
	stats := storeStats{
		Totals: make(map[Status]int),
		Queues: make(map[string]QueueStats),
	}

	for start := int64(0); ; start += redisListBatchSize {
		ids, err := s.client.ZRange(ctx, s.createdIndexKey(), start, start+redisListBatchSize-1).Result()
		if err != nil {
			return storeStats{}, err
		}
		if len(ids) == 0 {
			break
		}

		jobs, err := s.loadJobsByIDs(ctx, ids)
		if err != nil {
			return storeStats{}, err
		}

		for _, job := range jobs {
			stats.JobsStored++
			stats.Totals[job.Status]++

			queueStats := stats.Queues[job.Queue]
			queueStats.Name = job.Queue
			if queueStats.Totals == nil {
				queueStats.Totals = make(map[Status]int)
			}
			queueStats.Totals[job.Status]++
			queueStats.JobsTotal++
			stats.Queues[job.Queue] = queueStats
		}

		if len(ids) < int(redisListBatchSize) {
			break
		}
	}

	return stats, nil
}

func (s *RedisStore) updateJob(ctx context.Context, id string, mutate func(*Job) error) (Job, error) {
	key := s.jobKey(id)

	for attempt := 0; attempt < 5; attempt++ {
		var updated Job

		err := s.client.Watch(ctx, func(tx *redis.Tx) error {
			raw, err := tx.Get(ctx, key).Bytes()
			if err == redis.Nil {
				return notFoundJob(id)
			}
			if err != nil {
				return err
			}

			job, err := decodeRedisJob(raw)
			if err != nil {
				return err
			}
			if err := mutate(&job); err != nil {
				return err
			}

			updated = job.clone()
			encoded, err := json.Marshal(job)
			if err != nil {
				return err
			}

			_, err = tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
				pipe.Set(ctx, key, encoded, 0)
				s.syncIndexes(ctx, pipe, job)
				return nil
			})
			return err
		}, key)
		if err == redis.TxFailedErr {
			continue
		}
		if err != nil {
			return Job{}, err
		}

		return updated, nil
	}

	return Job{}, apperr.New(apperr.ErrorCodeInternal).
		WithMessage("failed to update job due to concurrent updates")
}

func (s *RedisStore) loadJobsByIDs(ctx context.Context, ids []string) ([]Job, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	keys := make([]string, 0, len(ids))
	for _, id := range ids {
		keys = append(keys, s.jobKey(id))
	}

	raws, err := s.client.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, err
	}

	jobs := make([]Job, 0, len(raws))
	for _, item := range raws {
		if item == nil {
			continue
		}

		raw, ok := item.(string)
		if !ok {
			continue
		}

		job, err := decodeRedisJob([]byte(raw))
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}

	return jobs, nil
}

func (s *RedisStore) syncIndexes(ctx context.Context, pipe redis.Pipeliner, job Job) {
	pipe.ZRem(ctx, s.availableIndexKey(), job.ID)
	if job.Status == StatusQueued || job.Status == StatusScheduled {
		pipe.ZAdd(ctx, s.availableIndexKey(), redis.Z{
			Score:  timeScore(job.AvailableAt),
			Member: job.ID,
		})
	}

	pipe.ZRem(ctx, s.terminalIndexKey(), job.ID)
	if job.Status.IsTerminal() && job.CompletedAt != nil {
		pipe.ZAdd(ctx, s.terminalIndexKey(), redis.Z{
			Score:  timeScore(*job.CompletedAt),
			Member: job.ID,
		})
	}
}

func (s *RedisStore) jobPrefix() string {
	return s.namespace + ":job:"
}

func (s *RedisStore) jobKey(id string) string {
	return s.jobPrefix() + id
}

func (s *RedisStore) createdIndexKey() string {
	return s.namespace + ":index:created"
}

func (s *RedisStore) availableIndexKey() string {
	return s.namespace + ":index:available"
}

func (s *RedisStore) terminalIndexKey() string {
	return s.namespace + ":index:terminal"
}

func decodeRedisJob(raw []byte) (Job, error) {
	var job Job
	if err := json.Unmarshal(raw, &job); err != nil {
		return Job{}, err
	}
	return job, nil
}

func normalizeRedisNamespace(namespace string) string {
	namespace = strings.TrimSpace(namespace)
	if namespace == "" {
		return "platform-background-jobs"
	}
	return namespace
}

func normalizeRedisTimeout(value, fallback time.Duration) time.Duration {
	if value <= 0 {
		return fallback
	}
	return value
}

func timeScore(t time.Time) float64 {
	return float64(t.UTC().UnixMilli())
}
