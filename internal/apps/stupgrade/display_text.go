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

import "strings"

const localizedSeparator = " / "

type localizedPair struct {
	zh string
	en string
}

// normalizeUserVisibleText 收敛历史遗留的“英文 / 中文”双语拼接文案，默认保留中文展示。
// normalizeUserVisibleText collapses legacy "English / Chinese" strings and keeps the Chinese text for display.
func normalizeUserVisibleText(text string) string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return ""
	}

	if pair, ok := splitLocalizedPair(trimmed); ok {
		return strings.TrimSpace(pair.zh)
	}
	return trimmed
}

func splitLocalizedPair(value string) (localizedPair, bool) {
	best := localizedPair{}
	bestScore := 0
	start := 0

	for start < len(value) {
		separatorIndex := strings.Index(value[start:], localizedSeparator)
		if separatorIndex < 0 {
			break
		}
		separatorIndex += start

		left := strings.TrimSpace(value[:separatorIndex])
		right := strings.TrimSpace(value[separatorIndex+len(localizedSeparator):])
		start = separatorIndex + len(localizedSeparator)

		if left == "" || right == "" {
			continue
		}

		candidates := []localizedPair{
			{zh: left, en: right},
			{zh: right, en: left},
		}
		for _, candidate := range candidates {
			score := scoreLocalizedCandidate(candidate)
			if score > bestScore {
				best = candidate
				bestScore = score
			}
		}
	}

	if bestScore <= 0 {
		return localizedPair{}, false
	}
	return best, true
}

func scoreLocalizedCandidate(candidate localizedPair) int {
	zhChineseCount := countChinese(candidate.zh)
	zhLatinCount := countLatin(candidate.zh)
	enChineseCount := countChinese(candidate.en)
	enLatinCount := countLatin(candidate.en)

	if zhChineseCount == 0 || enLatinCount == 0 {
		return 0
	}

	return zhChineseCount*2 - zhLatinCount + enLatinCount*2 - enChineseCount
}

func countChinese(value string) int {
	count := 0
	for _, r := range value {
		if r >= 0x4e00 && r <= 0x9fff {
			count++
		}
	}
	return count
}

func countLatin(value string) int {
	count := 0
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			count++
		}
	}
	return count
}
