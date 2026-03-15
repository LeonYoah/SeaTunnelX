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

package diagnostics

import (
	"fmt"
	"html/template"
	"strings"
	"unicode"
)

func bilingualText(zh, en string) string {
	zh = strings.TrimSpace(zh)
	en = strings.TrimSpace(en)
	switch {
	case zh == "":
		return en
	case en == "":
		return zh
	default:
		return zh + " / " + en
	}
}

func renderDiagnosticLocalizedPair(zh, en string) template.HTML {
	zh = strings.TrimSpace(zh)
	en = strings.TrimSpace(en)
	switch {
	case zh == "" && en == "":
		return template.HTML(template.HTMLEscapeString("-"))
	case zh == "" || en == "" || zh == en:
		return template.HTML(template.HTMLEscapeString(firstNonEmptyString(zh, en)))
	default:
		return template.HTML(
			`<span class="i18n-zh">` + template.HTMLEscapeString(zh) + `</span>` +
				`<span class="i18n-en">` + template.HTMLEscapeString(en) + `</span>`,
		)
	}
}

func renderDiagnosticLocalizedPairByLanguage(zh, en string, lang DiagnosticLanguage) template.HTML {
	value := chooseDiagnosticLocalizedText(diagnosticLocalizedText{ZH: zh, EN: en}, lang)
	value = strings.TrimSpace(value)
	if value == "" {
		value = "-"
	}
	return template.HTML(template.HTMLEscapeString(value))
}

func renderDiagnosticLocalizedText(value string) template.HTML {
	if zh, en, ok := splitDiagnosticLocalizedText(value); ok {
		return renderDiagnosticLocalizedPair(zh, en)
	}
	return template.HTML(template.HTMLEscapeString(strings.TrimSpace(value)))
}

func renderDiagnosticLocalizedTextByLanguage(value string, lang DiagnosticLanguage) template.HTML {
	value = strings.TrimSpace(localizeDiagnosticText(value, lang))
	if value == "" {
		value = "-"
	}
	return template.HTML(template.HTMLEscapeString(value))
}

func splitDiagnosticLocalizedText(value string) (string, string, bool) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", "", false
	}
	if zh, en, ok := diagnosticLocalizedLookup(trimmed); ok {
		return zh, en, true
	}
	left, right, found := findDiagnosticLocalizedPair(trimmed)
	if !found {
		return trimmed, trimmed, false
	}
	return left, right, true
}

func containsHanRune(value string) bool {
	for _, r := range value {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}

func containsLatinRune(value string) bool {
	for _, r := range value {
		if unicode.IsLetter(r) && r <= unicode.MaxASCII {
			return true
		}
	}
	return false
}

func countHanRunes(value string) int {
	count := 0
	for _, r := range value {
		if unicode.Is(unicode.Han, r) {
			count++
		}
	}
	return count
}

func countLatinRunes(value string) int {
	count := 0
	for _, r := range value {
		if unicode.IsLetter(r) && r <= unicode.MaxASCII {
			count++
		}
	}
	return count
}

func findDiagnosticLocalizedPair(value string) (string, string, bool) {
	const separator = " / "
	bestScore := 0
	bestLeft := ""
	bestRight := ""
	startIndex := 0
	for startIndex < len(value) {
		index := strings.Index(value[startIndex:], separator)
		if index < 0 {
			break
		}
		index += startIndex

		left := strings.TrimSpace(value[:index])
		right := strings.TrimSpace(value[index+len(separator):])
		startIndex = index + len(separator)

		if left == "" || right == "" {
			continue
		}

		leftHan := countHanRunes(left)
		leftLatin := countLatinRunes(left)
		rightHan := countHanRunes(right)
		rightLatin := countLatinRunes(right)

		if leftHan == 0 || rightLatin == 0 {
			continue
		}

		score := leftHan*2 - leftLatin + rightLatin*2 - rightHan
		if score > bestScore {
			bestScore = score
			bestLeft = left
			bestRight = right
		}
	}

	if bestScore <= 0 || bestLeft == "" || bestRight == "" {
		return "", "", false
	}
	return bestLeft, bestRight, true
}

func diagnosticLocalizedLookup(value string) (string, string, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "healthy":
		return "健康", "Healthy", true
	case "warning":
		return "警告", "Warning", true
	case "critical":
		return "严重", "Critical", true
	case "info":
		return "信息", "Info", true
	case "succeeded":
		return "成功", "Succeeded", true
	case "failed":
		return "失败", "Failed", true
	case "running":
		return "进行中", "Running", true
	case "pending":
		return "等待中", "Pending", true
	case "skipped":
		return "已跳过", "Skipped", true
	case "firing":
		return "告警中", "Firing", true
	case "resolved":
		return "已恢复", "Resolved", true
	case "inspection_finding":
		return "巡检发现触发", "Inspection Finding", true
	case "error_group":
		return "错误组触发", "Error Group", true
	case "manual":
		return "手动创建", "Manual", true
	case "alert":
		return "告警触发", "Alert", true
	case "restart_failed":
		return "重启失败", "Restart Failed", true
	case "crashed":
		return "进程崩溃", "Crashed", true
	case "node_offline":
		return "节点离线", "Node Offline", true
	case "node_recovered":
		return "节点恢复", "Node Recovered", true
	case "restarted":
		return "已重启", "Restarted", true
	case "started":
		return "已启动", "Started", true
	case "stopped":
		return "已停止", "Stopped", true
	default:
		return "", "", false
	}
}

func resolveDiagnosticCommandFailure(output string, err error, fallbackZH, fallbackEN string) string {
	detail := strings.TrimSpace(output)
	if detail == "" && err != nil {
		detail = strings.TrimSpace(err.Error())
	}
	if detail == "" {
		return bilingualText(fallbackZH, fallbackEN)
	}
	return detail
}

func formatDiagnosticAllNodesFailed(prefixZH, prefixEN string, items []string) error {
	prefix := bilingualText(prefixZH, prefixEN)
	filtered := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item != "" {
			filtered = append(filtered, item)
		}
	}
	if len(filtered) == 0 {
		return fmt.Errorf("%s", prefix)
	}
	return fmt.Errorf("%s: %s", prefix, strings.Join(filtered, "; "))
}
