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

package sync

import (
	"fmt"
	"sync"
	"time"
)

// JobIDGenerator generates platform-owned numeric job ids.
// JobIDGenerator 生成平台自管的纯数字作业 ID。
type JobIDGenerator struct {
	mu            sync.Mutex
	lastUnixMilli int64
	sequence      int
}

// NewJobIDGenerator creates a numeric job id generator.
// NewJobIDGenerator 创建纯数字作业 ID 生成器。
func NewJobIDGenerator() *JobIDGenerator {
	return &JobIDGenerator{}
}

// NextJobID returns a pure numeric job id string.
// NextJobID 返回纯数字字符串作业 ID。
func (g *JobIDGenerator) NextJobID() string {
	if g == nil {
		g = NewJobIDGenerator()
	}
	g.mu.Lock()
	defer g.mu.Unlock()

	nowMillis := time.Now().UnixMilli()
	if nowMillis == g.lastUnixMilli {
		g.sequence++
	} else {
		g.lastUnixMilli = nowMillis
		g.sequence = 0
	}
	return fmt.Sprintf("%d%05d", nowMillis, g.sequence)
}
