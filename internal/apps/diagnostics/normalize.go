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
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
)

var (
	exceptionClassPattern   = regexp.MustCompile(`([A-Za-z0-9_$.]+(?:Exception|Error))`)
	javaThreadPrefixPattern = regexp.MustCompile(`^Exception in thread "[^"]+"\s+`)
	seatunnelLogHeaderPattern = regexp.MustCompile(`^(?:\[[^\]]*\]\s+)?\d{4}-\d{2}-\d{2}[ T]\d{2}:\d{2}:\d{2}(?:[\.,]\d{3})?\s+(?:TRACE|DEBUG|INFO|WARN|ERROR|FATAL)\s+(?:\[[^\]]*\]\s+){0,2}-\s*`)
	timestampPattern        = regexp.MustCompile(`\d{4}-\d{2}-\d{2}[ T]\d{2}:\d{2}:\d{2}(?:[\.,]\d{3})?`)
	uuidPattern             = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)
	ipPattern               = regexp.MustCompile(`\b\d{1,3}(?:\.\d{1,3}){3}\b`)
	hexPattern              = regexp.MustCompile(`0x[0-9a-fA-F]+`)
	durationPattern         = regexp.MustCompile(`\b\d+(?:\.\d+)?\s*(?:ns|us|µs|ms|s|m|h|sec|secs|second|seconds|minute|minutes|hour|hours)\b`)
	numberPattern           = regexp.MustCompile(`\b\d+\b`)
	statusSuffixPattern     = regexp.MustCompile(`\b([A-Z][A-Z0-9_]{2,}:\s+.+)$`)
	separatorOnlyPattern    = regexp.MustCompile(`^[=\-_*#~\s]+$`)
	whitespacePattern       = regexp.MustCompile(`\s+`)
)

var diagnosticNoiseExactLines = map[string]struct{}{
	"fatal error": {},
	"please submit bug report in https://github.com/apache/seatunnel/issues": {},
	"reason:seatunnel job executed failed":                                   {},
}

// BuildErrorFingerprint normalizes a Seatunnel error sample into a stable fingerprint.
// BuildErrorFingerprint 将 Seatunnel 错误样本归一化为稳定指纹。
func BuildErrorFingerprint(message, evidence string) (fingerprint, normalized, exceptionClass, title string) {
	raw := strings.TrimSpace(evidence)
	if raw == "" {
		raw = strings.TrimSpace(message)
	}
	rootLine := extractRootCauseLine(message, raw)
	exceptionClass = ExtractExceptionClass(rootLine)
	title = buildErrorTitle(rootLine, message, raw, exceptionClass)
	normalized = normalizeFingerprintText(strings.Join(composeFingerprintLines(title, rootLine, raw, exceptionClass), "\n"))
	if normalized == "" {
		normalized = normalizeFingerprintText(title)
	}
	if normalized == "" {
		return "", "", "", ""
	}
	hash := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(hash[:]), normalized, exceptionClass, title
}

// ExtractExceptionClass extracts the first exception/error class from text.
// ExtractExceptionClass 从文本中提取第一个异常/错误类名。
func ExtractExceptionClass(value string) string {
	matches := exceptionClassPattern.FindStringSubmatch(value)
	if len(matches) != 2 {
		return ""
	}
	return strings.TrimSpace(matches[1])
}

func buildErrorTitle(rootLine, message, evidence, exceptionClass string) string {
	candidates := []string{
		strings.TrimSpace(rootLine),
		strings.TrimSpace(message),
		firstMeaningfulLine(evidence),
	}
	for _, item := range candidates {
		item = canonicalizeFingerprintLine(item)
		if item == "" || isDiagnosticNoiseLine(item) {
			continue
		}
		if exceptionClass != "" && !strings.Contains(item, exceptionClass) {
			return exceptionClass + ": " + item
		}
		return item
	}
	return exceptionClass
}

func composeFingerprintLines(title, rootLine, evidence, exceptionClass string) []string {
	result := make([]string, 0, 6)
	seen := make(map[string]struct{}, 8)
	appendUnique := func(value string) {
		value = canonicalizeFingerprintLine(value)
		if isDiagnosticNoiseLine(value) {
			return
		}
		if value == "" {
			return
		}
		if _, exists := seen[value]; exists {
			return
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}

	appendUnique(exceptionClass)
	appendUnique(rootLine)
	if len(result) > 0 {
		return result
	}

	appendUnique(title)

	for _, line := range strings.Split(evidence, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "at ") {
			continue
		}
		appendUnique(trimmed)
		if len(result) >= 6 {
			break
		}
	}
	return result
}

func normalizeFingerprintText(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	normalized = timestampPattern.ReplaceAllString(normalized, "<ts>")
	normalized = uuidPattern.ReplaceAllString(normalized, "<uuid>")
	normalized = ipPattern.ReplaceAllString(normalized, "<ip>")
	normalized = hexPattern.ReplaceAllString(normalized, "<hex>")
	normalized = durationPattern.ReplaceAllString(normalized, "<duration>")
	normalized = numberPattern.ReplaceAllString(normalized, "<num>")
	normalized = whitespacePattern.ReplaceAllString(normalized, " ")
	return strings.TrimSpace(normalized)
}

func extractRootCauseLine(message, evidence string) string {
	if causedBy := lastCausedByLine(evidence); causedBy != "" {
		return canonicalizeFingerprintLine(causedBy)
	}
	for _, candidate := range []string{
		strings.TrimSpace(message),
		firstMeaningfulLine(evidence),
	} {
		if line := canonicalizeFingerprintLine(candidate); line != "" && !isDiagnosticNoiseLine(line) {
			return line
		}
	}
	return ""
}

func lastCausedByLine(value string) string {
	result := ""
	for _, line := range strings.Split(value, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "Caused by:") {
			candidate := strings.TrimSpace(strings.TrimPrefix(trimmed, "Caused by:"))
			if !isDiagnosticNoiseLine(candidate) {
				result = candidate
			}
		}
	}
	return result
}

func canonicalizeFingerprintLine(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	if strings.HasPrefix(trimmed, "Caused by:") {
		trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, "Caused by:"))
	}
	if strings.HasPrefix(trimmed, "Exception StackTrace:") {
		trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, "Exception StackTrace:"))
	}
	trimmed = strings.TrimSpace(seatunnelLogHeaderPattern.ReplaceAllString(trimmed, ""))
	trimmed = strings.TrimSpace(javaThreadPrefixPattern.ReplaceAllString(trimmed, ""))
	if matches := statusSuffixPattern.FindStringSubmatch(trimmed); len(matches) == 2 {
		trimmed = strings.TrimSpace(matches[1])
	}
	trimmed = whitespacePattern.ReplaceAllString(trimmed, " ")
	return strings.TrimSpace(trimmed)
}

func firstMeaningfulLine(value string) string {
	for _, line := range strings.Split(value, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "at ") {
			continue
		}
		candidate := canonicalizeFingerprintLine(trimmed)
		if isDiagnosticNoiseLine(candidate) {
			continue
		}
		return candidate
	}
	return ""
}

func isDiagnosticNoiseLine(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return true
	}
	if separatorOnlyPattern.MatchString(trimmed) {
		return true
	}
	normalized := strings.ToLower(strings.Trim(trimmed, " \t\r\n,.;:-"))
	if normalized == "" {
		return true
	}
	_, ok := diagnosticNoiseExactLines[normalized]
	return ok
}
