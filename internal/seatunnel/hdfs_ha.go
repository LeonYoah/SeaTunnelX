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

package seatunnel

import (
	"fmt"
	"strings"
)

// HDFSHARPCEndpoint describes one configured HA namenode RPC endpoint.
// HDFSHARPCEndpoint 描述一个 HDFS HA Namenode RPC 地址。
type HDFSHARPCEndpoint struct {
	Name    string
	Address string
}

// ParseHDFSHANamenodes parses and validates HDFS HA namenode aliases.
// ParseHDFSHANamenodes 解析并校验 HDFS HA 的 namenode 别名列表。
func ParseHDFSHANamenodes(raw string) ([]string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, fmt.Errorf("hdfs_ha_namenodes is required for HDFS HA storage")
	}

	parts := strings.Split(trimmed, ",")
	namenodes := make([]string, 0, len(parts))
	for _, part := range parts {
		name := strings.TrimSpace(part)
		if name != "" {
			namenodes = append(namenodes, name)
		}
	}

	switch len(namenodes) {
	case 0:
		return nil, fmt.Errorf("hdfs_ha_namenodes is required for HDFS HA storage")
	case 2:
		return namenodes, nil
	default:
		if len(namenodes) < 2 {
			return nil, fmt.Errorf("hdfs_ha_namenodes must include exactly two namenodes for HDFS HA storage")
		}
		return nil, fmt.Errorf("only two HDFS HA namenodes are supported")
	}
}

// ResolveHDFSHARPCAddresses validates and pairs namenode aliases with RPC addresses.
// ResolveHDFSHARPCAddresses 校验并组装 namenode 别名与 RPC 地址。
func ResolveHDFSHARPCAddresses(rawNamenodes string, addr1 string, addr2 string) ([]HDFSHARPCEndpoint, error) {
	namenodes, err := ParseHDFSHANamenodes(rawNamenodes)
	if err != nil {
		return nil, err
	}

	addresses := []string{
		strings.TrimSpace(addr1),
		strings.TrimSpace(addr2),
	}

	endpoints := make([]HDFSHARPCEndpoint, 0, len(namenodes))
	for idx, name := range namenodes {
		if addresses[idx] == "" {
			return nil, fmt.Errorf("hdfs_namenode_rpc_address_%d is required for HDFS HA storage", idx+1)
		}
		endpoints = append(endpoints, HDFSHARPCEndpoint{
			Name:    name,
			Address: addresses[idx],
		})
	}
	return endpoints, nil
}
