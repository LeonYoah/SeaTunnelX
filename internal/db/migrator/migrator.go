/*
 * MIT License
 *
 * Copyright (c) 2025 linux.do
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy
 * of this software and associated documentation files (the "Software"), to deal
 * in the Software without restriction, including without limitation the rights
 * to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 * copies of the Software, and to permit persons to whom the Software is
 * furnished to do so, subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in all
 * copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
 * AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 * LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 * OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
 * SOFTWARE.
 */

package migrator

import (
	"context"
	"errors"
	"log"
	"os"
	"strings"

	"github.com/seatunnel/seatunnelX/internal/apps/audit"
	"github.com/seatunnel/seatunnelX/internal/apps/auth"
	"github.com/seatunnel/seatunnelX/internal/apps/cluster"
	appconfig "github.com/seatunnel/seatunnelX/internal/apps/config"
	"github.com/seatunnel/seatunnelX/internal/apps/host"
	"github.com/seatunnel/seatunnelX/internal/apps/monitor"
	"github.com/seatunnel/seatunnelX/internal/apps/plugin"
	"github.com/seatunnel/seatunnelX/internal/apps/project"
	"github.com/seatunnel/seatunnelX/internal/config"
	"github.com/seatunnel/seatunnelX/internal/db"
	"gorm.io/gorm"
)

func Migrate() {
	// 先初始化数据库连接
	if err := db.InitDatabase(); err != nil {
		log.Fatalf("[Database] 初始化数据库失败: %v\n", err)
	}

	// 检查数据库是否已初始化
	if !db.IsDatabaseInitialized() {
		log.Println("[Database] 数据库未启用，跳过迁移")
		return
	}

	// 执行数据库表迁移，包含用户表
	// 注意：auth.User 是统一的用户表，同时支持密码登录和 OAuth 登录
	// Execute database table migration, including user table
	// Note: auth.User is the unified user table, supporting both password and OAuth login
	if err := db.GetDB(context.Background()).AutoMigrate(
		&auth.User{},           // 统一用户表（支持密码认证和 OAuth 认证）/ Unified user table
		&host.Host{},           // 主机管理表 / Host management table
		&cluster.Cluster{},     // 集群表 / Cluster table
		&cluster.ClusterNode{}, // 集群节点表 / Cluster node table
		&project.Project{},     // 项目表 / Project table
		&project.ProjectItem{}, // 项目条目表 / Project item table
		&project.ProjectTag{},  // 项目标签表 / Project tag table
		&project.ProjectReport{},
		&audit.CommandLog{},              // 命令日志表 / Command log table
		&audit.AuditLog{},                // 审计日志表 / Audit log table
		&plugin.InstalledPlugin{},        // 已安装插件表 / Installed plugin table
		&plugin.PluginDependencyConfig{}, // 插件依赖配置表 / Plugin dependency config table
		&appconfig.Config{},              // 配置文件表 / Config file table
		&appconfig.ConfigVersion{},       // 配置版本表 / Config version table
		&monitor.MonitorConfig{},         // 监控配置表 / Monitor config table (Requirements: 5.2)
		&monitor.ProcessEvent{},          // 进程事件表 / Process event table (Requirements: 6.1)
	); err != nil {
		log.Fatalf("[Database] auto migrate failed: %v\n", err)
	}
	log.Printf("[Database] auto migrate success\n")

	// 初始化默认管理员用户
	if err := initDefaultAdminUser(); err != nil {
		log.Printf("[Database] 初始化默认管理员用户失败: %v\n", err)
	}

	// 创建存储过程（仅 MySQL 支持）
	dbType := db.GetDatabaseType()
	if dbType == "mysql" {
		if err := createStoredProcedures(); err != nil {
			log.Printf("[Database] create stored procedures failed (may not be supported): %v\n", err)
		}
	} else {
		log.Printf("[Database] 跳过存储过程创建（当前数据库类型: %s）\n", dbType)
	}
}

// initDefaultAdminUser 初始化默认管理员用户
// 仅在首次启动时（用户表为空）创建默认 admin 用户
// Requirements: 2.1, 2.2
func initDefaultAdminUser() error {
	database := db.GetDB(context.Background())
	if database == nil {
		return errors.New("数据库连接未初始化")
	}

	// 获取认证配置
	authConfig := config.GetAuthConfig()

	// 检查是否已存在用户
	var count int64
	if err := database.Model(&auth.User{}).Count(&count).Error; err != nil {
		return err
	}

	// 如果已有用户，跳过初始化
	if count > 0 {
		log.Println("[Database] 用户表已有数据，跳过默认管理员初始化")
		return nil
	}

	// 检查是否已存在 admin 用户（双重检查）
	var existingUser auth.User
	err := database.Where("username = ?", authConfig.DefaultAdminUsername).First(&existingUser).Error
	if err == nil {
		log.Printf("[Database] 管理员用户 '%s' 已存在，跳过创建\n", authConfig.DefaultAdminUsername)
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	// 创建默认管理员用户
	adminUser := &auth.User{
		Username: authConfig.DefaultAdminUsername,
		Nickname: "系统管理员",
		IsActive: true,
		IsAdmin:  true,
	}

	// 设置密码（使用 bcrypt 哈希）
	if err := adminUser.SetPassword(authConfig.DefaultAdminPassword, authConfig.BcryptCost); err != nil {
		return err
	}

	// 保存到数据库
	if err := adminUser.Create(database); err != nil {
		return err
	}

	log.Printf("[Database] 成功创建默认管理员用户: %s\n", authConfig.DefaultAdminUsername)
	return nil
}

// 创建存储过程
func createStoredProcedures() error {
	// 读取SQL文件
	sqlFile := "support-files/sql/create_dashboard_proc.sql"
	content, err := os.ReadFile(sqlFile)
	if err != nil {
		return err
	}

	// 处理SQL脚本，替换DELIMITER并分割成单独的语句
	sqlContent := string(content)
	// 移除DELIMITER声明行
	sqlContent = strings.Replace(sqlContent, "DELIMITER $$", "", -1)
	sqlContent = strings.Replace(sqlContent, "DELIMITER ;", "", -1)
	sqlContent = strings.Replace(sqlContent, "$$", ";", -1)

	// 执行SQL语句
	if err := db.GetDB(context.Background()).Exec(sqlContent).Error; err != nil {
		return err
	}

	return nil
}
