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

package stupgrade

import "testing"

func TestNormalizeUserVisibleText_prefersChineseText(t *testing.T) {
	input := "package checksum is missing / 安装包缺少 checksum"
	got := normalizeUserVisibleText(input)
	if got != "安装包缺少 checksum" {
		t.Fatalf("expected chinese text, got %q", got)
	}
}

func TestNormalizeUserVisibleText_handlesNestedLocalizedPair(t *testing.T) {
	input := "upgrade failed at SYNC_PLUGINS: unsupported managed upgrade sub_command: sync_plugins_manifest / 不支持的受管升级子命令: sync_plugins_manifest; rollback succeeded / 升级在 SYNC_PLUGINS 失败：不支持的受管升级子命令: sync_plugins_manifest；回滚成功"
	got := normalizeUserVisibleText(input)
	want := "升级在 SYNC_PLUGINS 失败：不支持的受管升级子命令: sync_plugins_manifest；回滚成功"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}
