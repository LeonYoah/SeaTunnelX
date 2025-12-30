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

package config

import (
	"context"
	"errors"
	"time"
)

var (
	ErrTemplateNotFound      = errors.New("cluster template config not found")
	ErrCannotPromoteTemplate = errors.New("cannot promote template config")
	ErrCannotSyncTemplate    = errors.New("cannot sync template config from itself")
)

// HostProvider 主机信息提供者接口
type HostProvider interface {
	GetHostByID(ctx context.Context, id uint) (*HostInfo, error)
}

// NodeInfoProvider 节点信息提供者接口
type NodeInfoProvider interface {
	GetNodeInstallDir(ctx context.Context, clusterID uint, hostID uint) (string, error)
}

// HostInfo 主机信息
type HostInfo struct {
	ID        uint
	Name      string
	IPAddress string
}

// AgentClient Agent 客户端接口
type AgentClient interface {
	PullConfig(ctx context.Context, hostID uint, installDir string, configType ConfigType) (string, error)
	PushConfig(ctx context.Context, hostID uint, installDir string, configType ConfigType, content string) error
}

// Service 配置管理服务
type Service struct {
	repo             *Repository
	hostProvider     HostProvider
	nodeInfoProvider NodeInfoProvider
	agentClient      AgentClient
}

// NewService 创建配置服务实例
func NewService(repo *Repository, hostProvider HostProvider, nodeInfoProvider NodeInfoProvider, agentClient AgentClient) *Service {
	return &Service{
		repo:             repo,
		hostProvider:     hostProvider,
		nodeInfoProvider: nodeInfoProvider,
		agentClient:      agentClient,
	}
}

// Get 获取配置详情
func (s *Service) Get(ctx context.Context, id uint) (*ConfigInfo, error) {
	config, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return s.toConfigInfo(ctx, config)
}

// GetByCluster 获取集群所有配置
func (s *Service) GetByCluster(ctx context.Context, clusterID uint) ([]*ConfigInfo, error) {
	configs, err := s.repo.ListByCluster(ctx, clusterID)
	if err != nil {
		return nil, err
	}

	// 获取模板配置用于比较
	templates := make(map[ConfigType]*Config)
	for _, c := range configs {
		if c.IsTemplate() {
			templates[c.ConfigType] = c
		}
	}

	result := make([]*ConfigInfo, 0, len(configs))
	for _, c := range configs {
		info, err := s.toConfigInfo(ctx, c)
		if err != nil {
			return nil, err
		}
		// 检查是否与模板一致
		if !c.IsTemplate() {
			if tpl, ok := templates[c.ConfigType]; ok {
				info.MatchTemplate = c.Content == tpl.Content
			}
		}
		result = append(result, info)
	}
	return result, nil
}

// Create 创建配置
func (s *Service) Create(ctx context.Context, req *CreateConfigRequest, userID uint) (*ConfigInfo, error) {
	config := &Config{
		ClusterID:  req.ClusterID,
		HostID:     req.HostID,
		ConfigType: req.ConfigType,
		FilePath:   GetConfigFilePath(req.ConfigType),
		Content:    req.Content,
		Version:    1,
		UpdatedBy:  userID,
	}

	err := s.repo.Transaction(ctx, func(tx *Repository) error {
		if err := tx.Create(ctx, config); err != nil {
			return err
		}
		// 创建初始版本
		version := &ConfigVersion{
			ConfigID:  config.ID,
			Version:   1,
			Content:   req.Content,
			Comment:   req.Comment,
			CreatedBy: userID,
		}
		return tx.CreateVersion(ctx, version)
	})
	if err != nil {
		return nil, err
	}

	return s.toConfigInfo(ctx, config)
}

// Update 更新配置
func (s *Service) Update(ctx context.Context, id uint, req *UpdateConfigRequest, userID uint) (*ConfigInfo, error) {
	config, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// 内容没变化则不更新
	if config.Content == req.Content {
		return s.toConfigInfo(ctx, config)
	}

	oldVersion := config.Version
	config.Content = req.Content
	config.Version = oldVersion + 1
	config.UpdatedBy = userID
	config.UpdatedAt = time.Now()

	err = s.repo.Transaction(ctx, func(tx *Repository) error {
		if err := tx.Update(ctx, config); err != nil {
			return err
		}
		// 创建新版本
		version := &ConfigVersion{
			ConfigID:  config.ID,
			Version:   config.Version,
			Content:   req.Content,
			Comment:   req.Comment,
			CreatedBy: userID,
		}
		return tx.CreateVersion(ctx, version)
	})
	if err != nil {
		return nil, err
	}

	info, err := s.toConfigInfo(ctx, config)
	if err != nil {
		return nil, err
	}

	// 如果是节点配置（非模板），推送到节点
	if config.HostID != nil && s.nodeInfoProvider != nil && s.agentClient != nil {
		installDir, dirErr := s.nodeInfoProvider.GetNodeInstallDir(ctx, config.ClusterID, *config.HostID)
		if dirErr != nil {
			info.PushError = "获取节点安装目录失败: " + dirErr.Error()
		} else if installDir != "" {
			pushErr := s.agentClient.PushConfig(ctx, *config.HostID, installDir, config.ConfigType, config.Content)
			if pushErr != nil {
				info.PushError = "推送配置到节点失败: " + pushErr.Error()
			}
		}
	}

	return info, nil
}

// GetVersions 获取版本历史
func (s *Service) GetVersions(ctx context.Context, configID uint) ([]*ConfigVersionInfo, error) {
	versions, err := s.repo.ListVersions(ctx, configID)
	if err != nil {
		return nil, err
	}

	result := make([]*ConfigVersionInfo, len(versions))
	for i, v := range versions {
		result[i] = &ConfigVersionInfo{
			ID:        v.ID,
			ConfigID:  v.ConfigID,
			Version:   v.Version,
			Content:   v.Content,
			Comment:   v.Comment,
			CreatedBy: v.CreatedBy,
			CreatedAt: v.CreatedAt,
		}
	}
	return result, nil
}

// Rollback 回滚到指定版本
func (s *Service) Rollback(ctx context.Context, id uint, req *RollbackConfigRequest, userID uint) (*ConfigInfo, error) {
	config, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// 获取目标版本
	targetVersion, err := s.repo.GetVersion(ctx, id, req.Version)
	if err != nil {
		return nil, err
	}

	// 更新配置
	config.Content = targetVersion.Content
	config.Version = config.Version + 1
	config.UpdatedBy = userID
	config.UpdatedAt = time.Now()

	comment := req.Comment
	if comment == "" {
		comment = "Rollback to version " + string(rune(req.Version))
	}

	err = s.repo.Transaction(ctx, func(tx *Repository) error {
		if err := tx.Update(ctx, config); err != nil {
			return err
		}
		version := &ConfigVersion{
			ConfigID:  config.ID,
			Version:   config.Version,
			Content:   targetVersion.Content,
			Comment:   comment,
			CreatedBy: userID,
		}
		return tx.CreateVersion(ctx, version)
	})
	if err != nil {
		return nil, err
	}

	info, err := s.toConfigInfo(ctx, config)
	if err != nil {
		return nil, err
	}

	// 如果是节点配置（非模板），推送到节点
	if config.HostID != nil && s.nodeInfoProvider != nil && s.agentClient != nil {
		installDir, dirErr := s.nodeInfoProvider.GetNodeInstallDir(ctx, config.ClusterID, *config.HostID)
		if dirErr != nil {
			info.PushError = "获取节点安装目录失败: " + dirErr.Error()
		} else if installDir != "" {
			pushErr := s.agentClient.PushConfig(ctx, *config.HostID, installDir, config.ConfigType, config.Content)
			if pushErr != nil {
				info.PushError = "推送配置到节点失败: " + pushErr.Error()
			}
		}
	}

	return info, nil
}

// Promote 推广配置到集群（节点配置 → 集群模板 → 所有节点）
func (s *Service) Promote(ctx context.Context, id uint, req *PromoteConfigRequest, userID uint) error {
	config, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	// 不能推广模板配置
	if config.IsTemplate() {
		return ErrCannotPromoteTemplate
	}

	return s.repo.Transaction(ctx, func(tx *Repository) error {
		// 1. 更新或创建集群模板
		template, err := tx.GetTemplate(ctx, config.ClusterID, config.ConfigType)
		if errors.Is(err, ErrConfigNotFound) {
			// 创建模板
			template = &Config{
				ClusterID:  config.ClusterID,
				HostID:     nil,
				ConfigType: config.ConfigType,
				FilePath:   config.FilePath,
				Content:    config.Content,
				Version:    1,
				UpdatedBy:  userID,
			}
			if err := tx.Create(ctx, template); err != nil {
				return err
			}
		} else if err != nil {
			return err
		} else {
			// 更新模板
			template.Content = config.Content
			template.Version = template.Version + 1
			template.UpdatedBy = userID
			template.UpdatedAt = time.Now()
			if err := tx.Update(ctx, template); err != nil {
				return err
			}
		}

		// 创建模板版本
		templateVersion := &ConfigVersion{
			ConfigID:  template.ID,
			Version:   template.Version,
			Content:   config.Content,
			Comment:   req.Comment,
			CreatedBy: userID,
		}
		if err := tx.CreateVersion(ctx, templateVersion); err != nil {
			return err
		}

		// 2. 同步到所有节点配置
		nodeConfigs, err := tx.ListNodeConfigs(ctx, config.ClusterID, config.ConfigType)
		if err != nil {
			return err
		}

		for _, nc := range nodeConfigs {
			if nc.Content == config.Content {
				continue // 内容相同跳过
			}
			nc.Content = config.Content
			nc.Version = nc.Version + 1
			nc.UpdatedBy = userID
			nc.UpdatedAt = time.Now()
			if err := tx.Update(ctx, nc); err != nil {
				return err
			}
			// 创建节点版本
			nodeVersion := &ConfigVersion{
				ConfigID:  nc.ID,
				Version:   nc.Version,
				Content:   config.Content,
				Comment:   "Synced from cluster template",
				CreatedBy: userID,
			}
			if err := tx.CreateVersion(ctx, nodeVersion); err != nil {
				return err
			}
		}

		return nil
	})
}

// SyncFromTemplate 从集群模板同步到节点
func (s *Service) SyncFromTemplate(ctx context.Context, id uint, req *SyncConfigRequest, userID uint) (*ConfigInfo, error) {
	config, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// 不能同步模板配置
	if config.IsTemplate() {
		return nil, ErrCannotSyncTemplate
	}

	// 获取模板
	template, err := s.repo.GetTemplate(ctx, config.ClusterID, config.ConfigType)
	if err != nil {
		return nil, ErrTemplateNotFound
	}

	// 内容相同则不更新
	if config.Content == template.Content {
		return s.toConfigInfo(ctx, config)
	}

	config.Content = template.Content
	config.Version = config.Version + 1
	config.UpdatedBy = userID
	config.UpdatedAt = time.Now()

	comment := req.Comment
	if comment == "" {
		comment = "Synced from cluster template"
	}

	err = s.repo.Transaction(ctx, func(tx *Repository) error {
		if err := tx.Update(ctx, config); err != nil {
			return err
		}
		version := &ConfigVersion{
			ConfigID:  config.ID,
			Version:   config.Version,
			Content:   template.Content,
			Comment:   comment,
			CreatedBy: userID,
		}
		return tx.CreateVersion(ctx, version)
	})
	if err != nil {
		return nil, err
	}

	info, err := s.toConfigInfo(ctx, config)
	if err != nil {
		return nil, err
	}

	// 推送配置到节点
	if config.HostID != nil && s.nodeInfoProvider != nil && s.agentClient != nil {
		installDir, dirErr := s.nodeInfoProvider.GetNodeInstallDir(ctx, config.ClusterID, *config.HostID)
		if dirErr != nil {
			info.PushError = "获取节点安装目录失败: " + dirErr.Error()
		} else if installDir != "" {
			pushErr := s.agentClient.PushConfig(ctx, *config.HostID, installDir, config.ConfigType, config.Content)
			if pushErr != nil {
				info.PushError = "推送配置到节点失败: " + pushErr.Error()
			}
		}
	}

	return info, nil
}

// InitClusterConfigs 初始化集群配置（安装完成后调用）
func (s *Service) InitClusterConfigs(ctx context.Context, clusterID uint, hostID uint, installDir string, userID uint) error {
	for _, configType := range SupportedConfigTypes {
		// 从节点拉取配置
		content, err := s.agentClient.PullConfig(ctx, hostID, installDir, configType)
		if err != nil {
			continue // 某些配置文件可能不存在，跳过
		}

		if content == "" {
			continue // 空内容跳过
		}

		// 检查模板是否已存在
		existingTemplate, err := s.repo.GetTemplate(ctx, clusterID, configType)
		if err == nil && existingTemplate != nil {
			// 模板已存在，更新内容
			if existingTemplate.Content != content {
				existingTemplate.Content = content
				existingTemplate.Version = existingTemplate.Version + 1
				existingTemplate.UpdatedBy = userID
				existingTemplate.UpdatedAt = time.Now()
				if err := s.repo.Update(ctx, existingTemplate); err != nil {
					return err
				}
				// 创建新版本
				templateVersion := &ConfigVersion{
					ConfigID:  existingTemplate.ID,
					Version:   existingTemplate.Version,
					Content:   content,
					Comment:   "Updated from node sync",
					CreatedBy: userID,
				}
				if err := s.repo.CreateVersion(ctx, templateVersion); err != nil {
					return err
				}
			}
		} else {
			// 创建集群模板
			template := &Config{
				ClusterID:  clusterID,
				HostID:     nil,
				ConfigType: configType,
				FilePath:   GetConfigFilePath(configType),
				Content:    content,
				Version:    1,
				UpdatedBy:  userID,
			}
			if err := s.repo.Create(ctx, template); err != nil {
				return err
			}

			// 创建模板版本
			templateVersion := &ConfigVersion{
				ConfigID:  template.ID,
				Version:   1,
				Content:   content,
				Comment:   "Initial config from installation",
				CreatedBy: userID,
			}
			if err := s.repo.CreateVersion(ctx, templateVersion); err != nil {
				return err
			}
		}

		// 检查节点配置是否已存在
		existingNodeConfig, err := s.repo.GetNodeConfig(ctx, clusterID, hostID, configType)
		if err == nil && existingNodeConfig != nil {
			// 节点配置已存在，更新内容
			if existingNodeConfig.Content != content {
				existingNodeConfig.Content = content
				existingNodeConfig.Version = existingNodeConfig.Version + 1
				existingNodeConfig.UpdatedBy = userID
				existingNodeConfig.UpdatedAt = time.Now()
				if err := s.repo.Update(ctx, existingNodeConfig); err != nil {
					return err
				}
				// 创建新版本
				nodeVersion := &ConfigVersion{
					ConfigID:  existingNodeConfig.ID,
					Version:   existingNodeConfig.Version,
					Content:   content,
					Comment:   "Updated from node sync",
					CreatedBy: userID,
				}
				if err := s.repo.CreateVersion(ctx, nodeVersion); err != nil {
					return err
				}
			}
		} else {
			// 创建节点配置
			nodeConfig := &Config{
				ClusterID:  clusterID,
				HostID:     &hostID,
				ConfigType: configType,
				FilePath:   GetConfigFilePath(configType),
				Content:    content,
				Version:    1,
				UpdatedBy:  userID,
			}
			if err := s.repo.Create(ctx, nodeConfig); err != nil {
				return err
			}

			// 创建节点版本
			nodeVersion := &ConfigVersion{
				ConfigID:  nodeConfig.ID,
				Version:   1,
				Content:   content,
				Comment:   "Initial config from installation",
				CreatedBy: userID,
			}
			if err := s.repo.CreateVersion(ctx, nodeVersion); err != nil {
				return err
			}
		}
	}
	return nil
}

// SyncTemplateToAllNodes 将集群模板同步到所有节点配置
func (s *Service) SyncTemplateToAllNodes(ctx context.Context, clusterID uint, configType ConfigType, userID uint) (*SyncAllResult, error) {
	// 获取模板
	template, err := s.repo.GetTemplate(ctx, clusterID, configType)
	if err != nil {
		return nil, ErrTemplateNotFound
	}

	// 获取所有节点配置
	nodeConfigs, err := s.repo.ListNodeConfigs(ctx, clusterID, configType)
	if err != nil {
		return nil, err
	}

	result := &SyncAllResult{
		SyncedCount: 0,
		PushErrors:  make([]*PushError, 0),
	}

	err = s.repo.Transaction(ctx, func(tx *Repository) error {
		for _, nc := range nodeConfigs {
			if nc.Content == template.Content {
				continue // 内容相同跳过
			}
			nc.Content = template.Content
			nc.Version = nc.Version + 1
			nc.UpdatedBy = userID
			nc.UpdatedAt = time.Now()
			if err := tx.Update(ctx, nc); err != nil {
				return err
			}
			// 创建节点版本
			nodeVersion := &ConfigVersion{
				ConfigID:  nc.ID,
				Version:   nc.Version,
				Content:   template.Content,
				Comment:   "Synced from cluster template (batch)",
				CreatedBy: userID,
			}
			if err := tx.CreateVersion(ctx, nodeVersion); err != nil {
				return err
			}
			result.SyncedCount++
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// 推送配置到所有节点
	if s.nodeInfoProvider != nil && s.agentClient != nil {
		for _, nc := range nodeConfigs {
			if nc.HostID != nil {
				installDir, dirErr := s.nodeInfoProvider.GetNodeInstallDir(ctx, clusterID, *nc.HostID)
				if dirErr != nil {
					pushErr := &PushError{
						HostID:  *nc.HostID,
						Message: "获取节点安装目录失败: " + dirErr.Error(),
					}
					// 尝试获取主机 IP
					if s.hostProvider != nil {
						if host, err := s.hostProvider.GetHostByID(ctx, *nc.HostID); err == nil {
							pushErr.HostIP = host.IPAddress
						}
					}
					result.PushErrors = append(result.PushErrors, pushErr)
				} else if installDir != "" {
					if pushErr := s.agentClient.PushConfig(ctx, *nc.HostID, installDir, configType, template.Content); pushErr != nil {
						errInfo := &PushError{
							HostID:  *nc.HostID,
							Message: "推送配置失败: " + pushErr.Error(),
						}
						// 尝试获取主机 IP
						if s.hostProvider != nil {
							if host, err := s.hostProvider.GetHostByID(ctx, *nc.HostID); err == nil {
								errInfo.HostIP = host.IPAddress
							}
						}
						result.PushErrors = append(result.PushErrors, errInfo)
					}
				}
			}
		}
	}

	return result, nil
}

// PushConfigToNode 推送配置到节点
func (s *Service) PushConfigToNode(ctx context.Context, id uint, installDir string) error {
	config, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if config.HostID == nil {
		return errors.New("cannot push template config directly")
	}

	return s.agentClient.PushConfig(ctx, *config.HostID, installDir, config.ConfigType, config.Content)
}

// toConfigInfo 转换为 ConfigInfo
func (s *Service) toConfigInfo(ctx context.Context, config *Config) (*ConfigInfo, error) {
	info := &ConfigInfo{
		ID:         config.ID,
		ClusterID:  config.ClusterID,
		HostID:     config.HostID,
		ConfigType: config.ConfigType,
		FilePath:   config.FilePath,
		Content:    config.Content,
		Version:    config.Version,
		IsTemplate: config.IsTemplate(),
		UpdatedAt:  config.UpdatedAt,
		UpdatedBy:  config.UpdatedBy,
	}

	// 获取主机信息
	if config.HostID != nil && s.hostProvider != nil {
		host, err := s.hostProvider.GetHostByID(ctx, *config.HostID)
		if err == nil {
			info.HostName = host.Name
			info.HostIP = host.IPAddress
		}
	}

	return info, nil
}
