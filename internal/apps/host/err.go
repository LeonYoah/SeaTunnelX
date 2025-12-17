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

package host

import "errors"

// Error definitions for host management operations.
var (
	// ErrHostNotFound indicates the requested host does not exist.
	ErrHostNotFound = errors.New("host: host not found")
	// ErrHostNameDuplicate indicates a host with the same name already exists.
	ErrHostNameDuplicate = errors.New("host: host name already exists")
	// ErrHostIPInvalid indicates the IP address format is invalid.
	ErrHostIPInvalid = errors.New("host: invalid IP address format")
	// ErrHostHasCluster indicates the host is associated with one or more clusters.
	ErrHostHasCluster = errors.New("host: host is associated with clusters and cannot be deleted")
	// ErrHostNameEmpty indicates the host name is empty.
	ErrHostNameEmpty = errors.New("host: host name cannot be empty")
)

// Error codes for host management operations.
const (
	ErrCodeHostNotFound      = 2001
	ErrCodeHostNameDuplicate = 2002
	ErrCodeHostIPInvalid     = 2003
	ErrCodeHostHasCluster    = 2004
)
