package cache

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/tidwall/buntdb"
)

// CacheEntry represents a cached item.
type CacheEntry struct {
	Key       string        `json:"key"`
	Value     string        `json:"value"`
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`
	HitCount  int64         `json:"hit_count"`
	Size      int64         `json:"size"`
	TTL       time.Duration `json:"ttl"`
}

// Manager handles all cache operations.
type Manager struct {
	db     *buntdb.DB
	logger zerolog.Logger
	stats  *Statistics
}

// Statistics tracks cache performance.
type Statistics struct {
	mu        sync.RWMutex
	Hits      int64
	Misses    int64
	Sets      int64
	Deletes   int64
	TotalSize int64
	ItemCount int64
}

// NewManager creates a new cache manager.
func NewManager(cacheDir string, logger zerolog.Logger) (*Manager, error) {
	dbPath := fmt.Sprintf("%s/cache.db", cacheDir)

	db, err := buntdb.Open(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open cache database: %w", err)
	}

	// Create indexes
	if err := db.CreateIndex("ttl", "*", buntdb.IndexJSON("updated_at")); err != nil && err != buntdb.ErrIndexExists {
		return nil, fmt.Errorf("failed to create ttl index: %w", err)
	}

	if err := db.CreateIndex("size", "*", buntdb.IndexJSON("size")); err != nil && err != buntdb.ErrIndexExists {
		return nil, fmt.Errorf("failed to create size index: %w", err)
	}

	m := &Manager{
		db:     db,
		logger: logger,
		stats:  &Statistics{},
	}

	// Start cleanup routine
	go m.cleanupRoutine()

	logger.Info().Str("path", dbPath).Msg("Cache manager initialized")
	return m, nil
}

// Get retrieves a value from the cache.
func (m *Manager) Get(key string) (string, error) {
	var value string
	var entry CacheEntry

	err := m.db.View(func(tx *buntdb.Tx) error {
		val, err := tx.Get(key)
		if err != nil {
			return err
		}

		if err := json.Unmarshal([]byte(val), &entry); err != nil {
			return fmt.Errorf("failed to unmarshal cache entry: %w", err)
		}

		// Check if entry is expired
		if entry.TTL > 0 && time.Since(entry.UpdatedAt) > entry.TTL {
			return buntdb.ErrNotFound
		}

		value = entry.Value
		return nil
	})

	if err != nil {
		if err == buntdb.ErrNotFound {
			m.recordMiss()
			return "", fmt.Errorf("key not found: %s", key)
		}
		return "", fmt.Errorf("failed to get key: %w", err)
	}

	// Update hit count
	go func() {
		if err := m.incrementHitCount(key); err != nil {
			m.logger.Error().Err(err).Str("key", key).Msg("Failed to increment hit count")
		}
	}()

	m.recordHit()
	return value, nil
}

// Set stores a value in the cache.
func (m *Manager) Set(key, value string, ttl time.Duration) error {
	entry := CacheEntry{
		Key:       key,
		Value:     value,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		HitCount:  0,
		Size:      int64(len(value)),
		TTL:       ttl,
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal cache entry: %w", err)
	}

	err = m.db.Update(func(tx *buntdb.Tx) error {
		opts := &buntdb.SetOptions{}
		if ttl > 0 {
			opts.Expires = true
			opts.TTL = ttl
		}

		_, _, err := tx.Set(key, string(data), opts)
		return err
	})

	if err != nil {
		return fmt.Errorf("failed to set key: %w", err)
	}

	m.recordSet(entry.Size)
	m.logger.Debug().
		Str("key", key).
		Int64("size", entry.Size).
		Dur("ttl", ttl).
		Msg("Cache entry set")

	return nil
}

// Delete removes a value from the cache.
func (m *Manager) Delete(key string) error {
	err := m.db.Update(func(tx *buntdb.Tx) error {
		_, err := tx.Delete(key)
		return err
	})

	if err != nil && err != buntdb.ErrNotFound {
		return fmt.Errorf("failed to delete key: %w", err)
	}

	m.recordDelete()
	return nil
}

// GetStats returns current cache statistics.
func (m *Manager) GetStats() Statistics {
	m.stats.mu.RLock()
	defer m.stats.mu.RUnlock()
	return Statistics{
		Hits:      m.stats.Hits,
		Misses:    m.stats.Misses,
		Sets:      m.stats.Sets,
		Deletes:   m.stats.Deletes,
		TotalSize: m.stats.TotalSize,
		ItemCount: m.stats.ItemCount,
	}
}

// Close closes the cache database.
func (m *Manager) Close() error {
	if err := m.db.Close(); err != nil {
		return fmt.Errorf("failed to close cache database: %w", err)
	}
	return nil
}

// Helper methods

func (m *Manager) incrementHitCount(key string) error {
	return m.db.Update(func(tx *buntdb.Tx) error {
		val, err := tx.Get(key)
		if err != nil {
			return err
		}

		var entry CacheEntry
		if err := json.Unmarshal([]byte(val), &entry); err != nil {
			return err
		}

		entry.HitCount++
		entry.UpdatedAt = time.Now()

		data, err := json.Marshal(entry)
		if err != nil {
			return err
		}

		_, _, err = tx.Set(key, string(data), nil)
		return err
	})
}

func (m *Manager) cleanupRoutine() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		if err := m.cleanup(); err != nil {
			m.logger.Error().Err(err).Msg("Failed to run cache cleanup")
		}
	}
}

func (m *Manager) cleanup() error {
	count := 0
	err := m.db.Update(func(tx *buntdb.Tx) error {
		now := time.Now()
		var keysToDelete []string

		err := tx.Ascend("ttl", func(key, value string) bool {
			var entry CacheEntry
			if err := json.Unmarshal([]byte(value), &entry); err != nil {
				return true // Continue iteration
			}

			if entry.TTL > 0 && now.Sub(entry.UpdatedAt) > entry.TTL {
				keysToDelete = append(keysToDelete, key)
			}
			return true
		})

		if err != nil {
			return err
		}

		for _, key := range keysToDelete {
			if _, err := tx.Delete(key); err != nil {
				m.logger.Error().Err(err).Str("key", key).Msg("Failed to delete expired key")
			} else {
				count++
			}
		}

		return nil
	})

	if err != nil {
		return err
	}

	if count > 0 {
		m.logger.Info().Int("count", count).Msg("Cleaned up expired cache entries")
	}

	return nil
}

// Statistics helpers

func (m *Manager) recordHit() {
	m.stats.mu.Lock()
	m.stats.Hits++
	m.stats.mu.Unlock()
}

func (m *Manager) recordMiss() {
	m.stats.mu.Lock()
	m.stats.Misses++
	m.stats.mu.Unlock()
}

func (m *Manager) recordSet(size int64) {
	m.stats.mu.Lock()
	m.stats.Sets++
	m.stats.TotalSize += size
	m.stats.ItemCount++
	m.stats.mu.Unlock()
}

func (m *Manager) recordDelete() {
	m.stats.mu.Lock()
	m.stats.Deletes++
	m.stats.ItemCount--
	m.stats.mu.Unlock()
}
