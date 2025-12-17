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

package audit

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// setupTestDB creates an in-memory SQLite database for testing
// setupTestDB 创建用于测试的内存 SQLite 数据库
func setupTestDB(t *testing.T) (*gorm.DB, func()) {
	// Create a temporary directory for the test database
	// 为测试数据库创建临时目录
	tempDir, err := os.MkdirTemp("", "audit_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	dbPath := filepath.Join(tempDir, "test.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("Failed to open database: %v", err)
	}

	// Auto-migrate the models
	// 自动迁移模型
	if err := db.AutoMigrate(&CommandLog{}, &AuditLog{}); err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("Failed to migrate: %v", err)
	}

	cleanup := func() {
		sqlDB, _ := db.DB()
		if sqlDB != nil {
			sqlDB.Close()
		}
		os.RemoveAll(tempDir)
	}

	return db, cleanup
}

// genValidUsername generates valid usernames (alphanumeric, 1-50 chars)
// genValidUsername 生成有效的用户名（字母数字，1-50 字符）
func genValidUsername() gopter.Gen {
	return gen.RegexMatch("[a-zA-Z][a-zA-Z0-9_]{0,49}").SuchThat(func(s string) bool {
		return len(s) > 0 && len(s) <= 50
	})
}

// genValidAction generates valid action types
// genValidAction 生成有效的操作类型
func genValidAction() gopter.Gen {
	return gen.OneConstOf(
		"create",
		"update",
		"delete",
		"start",
		"stop",
		"restart",
		"deploy",
		"login",
		"logout",
	)
}

// genValidResourceType generates valid resource types
// genValidResourceType 生成有效的资源类型
func genValidResourceType() gopter.Gen {
	return gen.OneConstOf(
		"host",
		"cluster",
		"node",
		"command",
		"user",
		"project",
	)
}

// genValidResourceID generates valid resource IDs
// genValidResourceID 生成有效的资源 ID
func genValidResourceID() gopter.Gen {
	return gen.RegexMatch("[a-zA-Z0-9_-]{1,50}")
}

// genValidIPAddress generates valid IP addresses for testing
// genValidIPAddress 生成用于测试的有效 IP 地址
func genValidIPAddress() gopter.Gen {
	return gen.OneConstOf(
		"192.168.1.1",
		"10.0.0.1",
		"172.16.0.1",
		"127.0.0.1",
		"::1",
	)
}

// AuditLogTestData represents test data for audit log property tests
// AuditLogTestData 表示审计日志属性测试的测试数据
type AuditLogTestData struct {
	UserID       uint
	Username     string
	Action       string
	ResourceType string
	ResourceID   string
	IPAddress    string
}

// genAuditLogTestData generates valid audit log test data
// genAuditLogTestData 生成有效的审计日志测试数据
func genAuditLogTestData() gopter.Gen {
	return gopter.CombineGens(
		gen.UIntRange(1, 1000),
		genValidUsername(),
		genValidAction(),
		genValidResourceType(),
		genValidResourceID(),
		genValidIPAddress(),
	).Map(func(vals []interface{}) AuditLogTestData {
		return AuditLogTestData{
			UserID:       vals[0].(uint),
			Username:     vals[1].(string),
			Action:       vals[2].(string),
			ResourceType: vals[3].(string),
			ResourceID:   vals[4].(string),
			IPAddress:    vals[5].(string),
		}
	})
}

// **Feature: seatunnel-agent, Property 20: Audit Log Filtering**
// **Validates: Requirements 10.4**
// For any audit log query with filters (time range, action type, user, host),
// the returned results SHALL only contain entries matching all specified filter criteria.

func TestProperty_AuditLogFiltering(t *testing.T) {
	// **Feature: seatunnel-agent, Property 20: Audit Log Filtering**
	// **Validates: Requirements 10.4**

	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	parameters.Rng.Seed(42)

	properties := gopter.NewProperties(parameters)

	// Property: Filtering by action returns only matching entries
	// 属性：按操作过滤仅返回匹配的条目
	properties.Property("filtering by action returns only matching entries", prop.ForAll(
		func(testData1 AuditLogTestData, testData2 AuditLogTestData) bool {
			// Ensure different actions for meaningful test
			// 确保不同的操作以进行有意义的测试
			if testData1.Action == testData2.Action {
				return true // Skip if same action
			}

			db, cleanup := setupTestDB(t)
			defer cleanup()

			repo := NewRepository(db)
			ctx := context.Background()

			// Create audit logs with different actions
			// 创建具有不同操作的审计日志
			userID1 := testData1.UserID
			log1 := &AuditLog{
				UserID:       &userID1,
				Username:     testData1.Username,
				Action:       testData1.Action,
				ResourceType: testData1.ResourceType,
				ResourceID:   testData1.ResourceID,
				IPAddress:    testData1.IPAddress,
			}
			if err := repo.CreateAuditLog(ctx, log1); err != nil {
				t.Logf("Failed to create audit log 1: %v", err)
				return false
			}

			userID2 := testData2.UserID
			log2 := &AuditLog{
				UserID:       &userID2,
				Username:     testData2.Username,
				Action:       testData2.Action,
				ResourceType: testData2.ResourceType,
				ResourceID:   testData2.ResourceID,
				IPAddress:    testData2.IPAddress,
			}
			if err := repo.CreateAuditLog(ctx, log2); err != nil {
				t.Logf("Failed to create audit log 2: %v", err)
				return false
			}

			// Filter by action1
			// 按 action1 过滤
			filter := &AuditLogFilter{
				Action: testData1.Action,
			}
			results, _, err := repo.ListAuditLogs(ctx, filter)
			if err != nil {
				t.Logf("Failed to list audit logs: %v", err)
				return false
			}

			// All results should have the filtered action
			// 所有结果应具有过滤的操作
			for _, result := range results {
				if result.Action != testData1.Action {
					t.Logf("Found log with action %s, expected %s", result.Action, testData1.Action)
					return false
				}
			}

			return true
		},
		genAuditLogTestData(),
		genAuditLogTestData(),
	))

	// Property: Filtering by user ID returns only matching entries
	// 属性：按用户 ID 过滤仅返回匹配的条目
	properties.Property("filtering by user ID returns only matching entries", prop.ForAll(
		func(testData1 AuditLogTestData, testData2 AuditLogTestData) bool {
			// Ensure different user IDs for meaningful test
			// 确保不同的用户 ID 以进行有意义的测试
			if testData1.UserID == testData2.UserID {
				return true // Skip if same user ID
			}

			db, cleanup := setupTestDB(t)
			defer cleanup()

			repo := NewRepository(db)
			ctx := context.Background()

			// Create audit logs with different user IDs
			// 创建具有不同用户 ID 的审计日志
			userID1 := testData1.UserID
			log1 := &AuditLog{
				UserID:       &userID1,
				Username:     testData1.Username,
				Action:       testData1.Action,
				ResourceType: testData1.ResourceType,
				ResourceID:   testData1.ResourceID,
			}
			if err := repo.CreateAuditLog(ctx, log1); err != nil {
				t.Logf("Failed to create audit log 1: %v", err)
				return false
			}

			userID2 := testData2.UserID
			log2 := &AuditLog{
				UserID:       &userID2,
				Username:     testData2.Username,
				Action:       testData2.Action,
				ResourceType: testData2.ResourceType,
				ResourceID:   testData2.ResourceID,
			}
			if err := repo.CreateAuditLog(ctx, log2); err != nil {
				t.Logf("Failed to create audit log 2: %v", err)
				return false
			}

			// Filter by user ID 1
			// 按用户 ID 1 过滤
			filter := &AuditLogFilter{
				UserID: &userID1,
			}
			results, _, err := repo.ListAuditLogs(ctx, filter)
			if err != nil {
				t.Logf("Failed to list audit logs: %v", err)
				return false
			}

			// All results should have the filtered user ID
			// 所有结果应具有过滤的用户 ID
			for _, result := range results {
				if result.UserID == nil || *result.UserID != userID1 {
					t.Logf("Found log with user ID %v, expected %d", result.UserID, userID1)
					return false
				}
			}

			return true
		},
		genAuditLogTestData(),
		genAuditLogTestData(),
	))

	// Property: Filtering by resource type returns only matching entries
	// 属性：按资源类型过滤仅返回匹配的条目
	properties.Property("filtering by resource type returns only matching entries", prop.ForAll(
		func(testData1 AuditLogTestData, testData2 AuditLogTestData) bool {
			// Ensure different resource types for meaningful test
			// 确保不同的资源类型以进行有意义的测试
			if testData1.ResourceType == testData2.ResourceType {
				return true // Skip if same resource type
			}

			db, cleanup := setupTestDB(t)
			defer cleanup()

			repo := NewRepository(db)
			ctx := context.Background()

			// Create audit logs with different resource types
			// 创建具有不同资源类型的审计日志
			userID1 := testData1.UserID
			log1 := &AuditLog{
				UserID:       &userID1,
				Username:     testData1.Username,
				Action:       testData1.Action,
				ResourceType: testData1.ResourceType,
				ResourceID:   testData1.ResourceID,
			}
			if err := repo.CreateAuditLog(ctx, log1); err != nil {
				t.Logf("Failed to create audit log 1: %v", err)
				return false
			}

			userID2 := testData2.UserID
			log2 := &AuditLog{
				UserID:       &userID2,
				Username:     testData2.Username,
				Action:       testData2.Action,
				ResourceType: testData2.ResourceType,
				ResourceID:   testData2.ResourceID,
			}
			if err := repo.CreateAuditLog(ctx, log2); err != nil {
				t.Logf("Failed to create audit log 2: %v", err)
				return false
			}

			// Filter by resource type 1
			// 按资源类型 1 过滤
			filter := &AuditLogFilter{
				ResourceType: testData1.ResourceType,
			}
			results, _, err := repo.ListAuditLogs(ctx, filter)
			if err != nil {
				t.Logf("Failed to list audit logs: %v", err)
				return false
			}

			// All results should have the filtered resource type
			// 所有结果应具有过滤的资源类型
			for _, result := range results {
				if result.ResourceType != testData1.ResourceType {
					t.Logf("Found log with resource type %s, expected %s", result.ResourceType, testData1.ResourceType)
					return false
				}
			}

			return true
		},
		genAuditLogTestData(),
		genAuditLogTestData(),
	))

	properties.TestingRun(t)
}

// TestProperty_AuditLogTimeRangeFiltering tests time range filtering
// TestProperty_AuditLogTimeRangeFiltering 测试时间范围过滤
func TestProperty_AuditLogTimeRangeFiltering(t *testing.T) {
	// **Feature: seatunnel-agent, Property 20: Audit Log Filtering**
	// **Validates: Requirements 10.4**

	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	parameters.Rng.Seed(42)

	properties := gopter.NewProperties(parameters)

	// Property: Filtering by time range returns only entries within the range
	// 属性：按时间范围过滤仅返回范围内的条目
	properties.Property("filtering by time range returns only entries within range", prop.ForAll(
		func(testData AuditLogTestData) bool {
			db, cleanup := setupTestDB(t)
			defer cleanup()

			repo := NewRepository(db)
			ctx := context.Background()

			// Create an audit log
			// 创建审计日志
			userID := testData.UserID
			log := &AuditLog{
				UserID:       &userID,
				Username:     testData.Username,
				Action:       testData.Action,
				ResourceType: testData.ResourceType,
				ResourceID:   testData.ResourceID,
			}
			if err := repo.CreateAuditLog(ctx, log); err != nil {
				t.Logf("Failed to create audit log: %v", err)
				return false
			}

			// Get the created log to know its timestamp
			// 获取创建的日志以了解其时间戳
			createdLog, err := repo.GetAuditLogByID(ctx, log.ID)
			if err != nil {
				t.Logf("Failed to get audit log: %v", err)
				return false
			}

			// Filter with time range that includes the log
			// 使用包含日志的时间范围进行过滤
			startTime := createdLog.CreatedAt.Add(-1 * time.Hour)
			endTime := createdLog.CreatedAt.Add(1 * time.Hour)
			filter := &AuditLogFilter{
				StartTime: &startTime,
				EndTime:   &endTime,
			}
			results, _, err := repo.ListAuditLogs(ctx, filter)
			if err != nil {
				t.Logf("Failed to list audit logs: %v", err)
				return false
			}

			// Should find the log
			// 应该找到日志
			if len(results) == 0 {
				t.Logf("Expected to find log within time range")
				return false
			}

			// All results should be within the time range
			// 所有结果应在时间范围内
			for _, result := range results {
				if result.CreatedAt.Before(startTime) || result.CreatedAt.After(endTime) {
					t.Logf("Found log outside time range: %v", result.CreatedAt)
					return false
				}
			}

			// Filter with time range that excludes the log (future)
			// 使用排除日志的时间范围进行过滤（未来）
			futureStart := createdLog.CreatedAt.Add(1 * time.Hour)
			futureEnd := createdLog.CreatedAt.Add(2 * time.Hour)
			filterFuture := &AuditLogFilter{
				StartTime: &futureStart,
				EndTime:   &futureEnd,
			}
			resultsFuture, _, err := repo.ListAuditLogs(ctx, filterFuture)
			if err != nil {
				t.Logf("Failed to list audit logs: %v", err)
				return false
			}

			// Should not find the log
			// 不应该找到日志
			if len(resultsFuture) != 0 {
				t.Logf("Expected no logs in future time range, found %d", len(resultsFuture))
				return false
			}

			return true
		},
		genAuditLogTestData(),
	))

	properties.TestingRun(t)
}

// TestProperty_AuditLogCombinedFiltering tests combined filter criteria
// TestProperty_AuditLogCombinedFiltering 测试组合过滤条件
func TestProperty_AuditLogCombinedFiltering(t *testing.T) {
	// **Feature: seatunnel-agent, Property 20: Audit Log Filtering**
	// **Validates: Requirements 10.4**

	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	parameters.Rng.Seed(42)

	properties := gopter.NewProperties(parameters)

	// Property: Combined filters return only entries matching ALL criteria
	// 属性：组合过滤器仅返回匹配所有条件的条目
	properties.Property("combined filters return only entries matching all criteria", prop.ForAll(
		func(testData1 AuditLogTestData, testData2 AuditLogTestData) bool {
			// Ensure different data for meaningful test
			// 确保不同的数据以进行有意义的测试
			if testData1.Action == testData2.Action && testData1.ResourceType == testData2.ResourceType {
				return true // Skip if same action and resource type
			}

			db, cleanup := setupTestDB(t)
			defer cleanup()

			repo := NewRepository(db)
			ctx := context.Background()

			// Create audit logs with different combinations
			// 创建具有不同组合的审计日志
			userID1 := testData1.UserID
			log1 := &AuditLog{
				UserID:       &userID1,
				Username:     testData1.Username,
				Action:       testData1.Action,
				ResourceType: testData1.ResourceType,
				ResourceID:   testData1.ResourceID,
			}
			if err := repo.CreateAuditLog(ctx, log1); err != nil {
				t.Logf("Failed to create audit log 1: %v", err)
				return false
			}

			userID2 := testData2.UserID
			log2 := &AuditLog{
				UserID:       &userID2,
				Username:     testData2.Username,
				Action:       testData2.Action,
				ResourceType: testData2.ResourceType,
				ResourceID:   testData2.ResourceID,
			}
			if err := repo.CreateAuditLog(ctx, log2); err != nil {
				t.Logf("Failed to create audit log 2: %v", err)
				return false
			}

			// Filter by both action AND resource type from log1
			// 按 log1 的操作和资源类型进行过滤
			filter := &AuditLogFilter{
				Action:       testData1.Action,
				ResourceType: testData1.ResourceType,
			}
			results, _, err := repo.ListAuditLogs(ctx, filter)
			if err != nil {
				t.Logf("Failed to list audit logs: %v", err)
				return false
			}

			// All results should match BOTH criteria
			// 所有结果应匹配两个条件
			for _, result := range results {
				if result.Action != testData1.Action {
					t.Logf("Found log with action %s, expected %s", result.Action, testData1.Action)
					return false
				}
				if result.ResourceType != testData1.ResourceType {
					t.Logf("Found log with resource type %s, expected %s", result.ResourceType, testData1.ResourceType)
					return false
				}
			}

			return true
		},
		genAuditLogTestData(),
		genAuditLogTestData(),
	))

	properties.TestingRun(t)
}
