/*
 * Licensed to the Apache Software Foundation (ASF) under one or more
 * contributor license agreements.  See the NOTICE file distributed with
 * this work for additional information regarding copyright ownership.
 * The ASF licenses this file to You under the Apache License, Version 2.0
 * (the "License"); you may not use this file except in compliance with
 * the License.  You may obtain a copy of the License at
 *
 *    http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

// Package session 提供会话存储抽象层，支持内存存储和 Redis 存储
package session

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// 错误定义
var (
	ErrKeyNotFound = errors.New("session: key not found")
	ErrExpired     = errors.New("session: key expired")
)

// SessionStore 会话存储接口
// 定义了会话存储的基本操作：获取、设置、删除
type SessionStore interface {
	// Get 获取指定 key 的值
	// 如果 key 不存在，返回 ErrKeyNotFound
	Get(ctx context.Context, key string) (any, error)

	// Set 设置指定 key 的值，并指定过期时间
	// expiration 为 0 表示永不过期
	Set(ctx context.Context, key string, value any, expiration time.Duration) error

	// Delete 删除指定 key
	Delete(ctx context.Context, key string) error

	// Exists 检查 key 是否存在
	Exists(ctx context.Context, key string) (bool, error)
}

// memoryItem 内存存储项，包含值和过期时间
type memoryItem struct {
	value      any
	expiration int64 // Unix 纳秒时间戳，0 表示永不过期
}

// isExpired 检查存储项是否已过期
func (item *memoryItem) isExpired() bool {
	if item.expiration == 0 {
		return false
	}
	return time.Now().UnixNano() > item.expiration
}

// MemoryStore 内存会话存储实现
// 使用 sync.Map 实现线程安全的内存存储
type MemoryStore struct {
	data sync.Map
}

// NewMemoryStore 创建新的内存存储实例
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{}
}

// Get 从内存中获取指定 key 的值
func (m *MemoryStore) Get(ctx context.Context, key string) (any, error) {
	value, ok := m.data.Load(key)
	if !ok {
		return nil, ErrKeyNotFound
	}

	item, ok := value.(*memoryItem)
	if !ok {
		return nil, ErrKeyNotFound
	}

	// 检查是否过期
	if item.isExpired() {
		m.data.Delete(key)
		return nil, ErrExpired
	}

	return item.value, nil
}

// Set 将值存储到内存中
func (m *MemoryStore) Set(ctx context.Context, key string, value any, expiration time.Duration) error {
	var exp int64
	if expiration > 0 {
		exp = time.Now().Add(expiration).UnixNano()
	}

	item := &memoryItem{
		value:      value,
		expiration: exp,
	}
	m.data.Store(key, item)
	return nil
}

// Delete 从内存中删除指定 key
func (m *MemoryStore) Delete(ctx context.Context, key string) error {
	m.data.Delete(key)
	return nil
}

// Exists 检查 key 是否存在于内存中
func (m *MemoryStore) Exists(ctx context.Context, key string) (bool, error) {
	value, ok := m.data.Load(key)
	if !ok {
		return false, nil
	}

	item, ok := value.(*memoryItem)
	if !ok {
		return false, nil
	}

	// 检查是否过期
	if item.isExpired() {
		m.data.Delete(key)
		return false, nil
	}

	return true, nil
}

// RedisStore Redis 会话存储实现
type RedisStore struct {
	client *redis.Client
	prefix string // key 前缀，用于区分不同应用的会话
}

// NewRedisStore 创建新的 Redis 存储实例
func NewRedisStore(client *redis.Client, prefix string) *RedisStore {
	if prefix == "" {
		prefix = "session:"
	}
	return &RedisStore{
		client: client,
		prefix: prefix,
	}
}

// buildKey 构建带前缀的 key
func (r *RedisStore) buildKey(key string) string {
	return r.prefix + key
}

// Get 从 Redis 中获取指定 key 的值
func (r *RedisStore) Get(ctx context.Context, key string) (any, error) {
	fullKey := r.buildKey(key)
	result, err := r.client.Get(ctx, fullKey).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, ErrKeyNotFound
		}
		return nil, err
	}

	// 尝试解析 JSON
	var value any
	if err := json.Unmarshal([]byte(result), &value); err != nil {
		// 如果不是 JSON，直接返回字符串
		return result, nil
	}
	return value, nil
}

// Set 将值存储到 Redis 中
func (r *RedisStore) Set(ctx context.Context, key string, value any, expiration time.Duration) error {
	fullKey := r.buildKey(key)

	// 将值序列化为 JSON
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}

	return r.client.Set(ctx, fullKey, data, expiration).Err()
}

// Delete 从 Redis 中删除指定 key
func (r *RedisStore) Delete(ctx context.Context, key string) error {
	fullKey := r.buildKey(key)
	return r.client.Del(ctx, fullKey).Err()
}

// Exists 检查 key 是否存在于 Redis 中
func (r *RedisStore) Exists(ctx context.Context, key string) (bool, error) {
	fullKey := r.buildKey(key)
	result, err := r.client.Exists(ctx, fullKey).Result()
	if err != nil {
		return false, err
	}
	return result > 0, nil
}
