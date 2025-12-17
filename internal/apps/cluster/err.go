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

package cluster

import "errors"

// Error definitions for cluster management operations.
var (
	// ErrClusterNotFound indicates the requested cluster does not exist.
	ErrClusterNotFound = errors.New("cluster: cluster not found")
	// ErrClusterNameDuplicate indicates a cluster with the same name already exists.
	ErrClusterNameDuplicate = errors.New("cluster: cluster name already exists")
	// ErrClusterNameEmpty indicates the cluster name is empty.
	ErrClusterNameEmpty = errors.New("cluster: cluster name cannot be empty")
	// ErrClusterHasRunningTask indicates the cluster has running tasks and cannot be deleted.
	ErrClusterHasRunningTask = errors.New("cluster: cluster has running tasks and cannot be deleted")
	// ErrNodeNotFound indicates the requested cluster node does not exist.
	ErrNodeNotFound = errors.New("cluster: node not found")
	// ErrNodeAlreadyExists indicates the host is already a node in the cluster.
	ErrNodeAlreadyExists = errors.New("cluster: host is already a node in this cluster")
	// ErrNodeAgentNotInstalled indicates the host's agent is not installed.
	ErrNodeAgentNotInstalled = errors.New("cluster: host agent is not installed")
	// ErrInvalidDeploymentMode indicates an invalid deployment mode was specified.
	ErrInvalidDeploymentMode = errors.New("cluster: invalid deployment mode")
	// ErrInvalidNodeRole indicates an invalid node role was specified.
	ErrInvalidNodeRole = errors.New("cluster: invalid node role")
)

// Error codes for cluster management operations.
const (
	ErrCodeClusterNotFound       = 3001
	ErrCodeClusterNameDuplicate  = 3002
	ErrCodeClusterHasRunningTask = 3003
	ErrCodeNodeAgentNotInstalled = 3004
	ErrCodeNodeNotFound          = 3005
	ErrCodeNodeAlreadyExists     = 3006
)
