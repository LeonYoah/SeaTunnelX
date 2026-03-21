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

import {getCurrentLocale, type Locale} from './config';

const HAS_CHINESE_PATTERN = /[\u4e00-\u9fff]/;
const HAS_LATIN_PATTERN = /[A-Za-z]/;
const LOCALIZED_SEPARATOR = ' / ';
const PATH_CAN_BE_CREATED_RE =
  /^Path (.+) can be created \(parent (.+) is writable\)$/;
const PATH_PARENT_NOT_WRITABLE_RE =
  /^Path (.+) does not exist and parent (.+) is not writable$/;
const PATH_NOT_DIRECTORY_RE = /^Path (.+) is not a directory$/;
const PATH_EXISTS_RE = /^Path (.+) exists$/;
const PATH_MISSING_RE = /^Path (.+) does not exist$/;
const DIRECTORY_EXISTS_WRITABLE_RE = /^Directory (.+) exists and is writable$/;
const DIRECTORY_NOT_EXIST_RE = /^Directory (.+) does not exist$/;
const DIRECTORY_NOT_WRITABLE_RE = /^Directory (.+) is not writable: (.+)$/;
const DIRECTORY_NOTHING_TO_CLEAN_RE =
  /^Directory (.+) does not exist, nothing to clean$/;
const TCP_REACHABLE_RE = /^TCP endpoint (.+) is reachable$/;
const TCP_UNREACHABLE_RE = /^TCP endpoint (.+) is not reachable: (.+)$/;
const INVALID_YAML_RE = /^Invalid YAML in (.+): (.+)$/;
const INVALID_YAML_LINE_RE = /^Invalid YAML in (.+): yaml: line (\d+): (.+)$/;
const INVALID_ROOT_RE = /^Invalid (.+): expected top-level key '(.+)'$/;

function localizeStructuredText(value: string, locale: Locale): string | null {
  const structuredPatterns: Array<{
    pattern: RegExp;
    render: (match: RegExpExecArray) => {zh: string; en: string};
  }> = [
    {
      pattern: PATH_CAN_BE_CREATED_RE,
      render: (match) => ({
        zh: `路径 ${match[1]} 可创建（父目录 ${match[2]} 可写）`,
        en: `Path ${match[1]} can be created (parent ${match[2]} is writable)`,
      }),
    },
    {
      pattern: PATH_PARENT_NOT_WRITABLE_RE,
      render: (match) => ({
        zh: `路径 ${match[1]} 不存在，且父目录 ${match[2]} 不可写`,
        en: `Path ${match[1]} does not exist and parent ${match[2]} is not writable`,
      }),
    },
    {
      pattern: PATH_NOT_DIRECTORY_RE,
      render: (match) => ({
        zh: `路径 ${match[1]} 不是目录`,
        en: `Path ${match[1]} is not a directory`,
      }),
    },
    {
      pattern: PATH_EXISTS_RE,
      render: (match) => ({
        zh: `路径 ${match[1]} 已存在`,
        en: `Path ${match[1]} exists`,
      }),
    },
    {
      pattern: PATH_MISSING_RE,
      render: (match) => ({
        zh: `路径 ${match[1]} 不存在`,
        en: `Path ${match[1]} does not exist`,
      }),
    },
    {
      pattern: DIRECTORY_EXISTS_WRITABLE_RE,
      render: (match) => ({
        zh: `目录 ${match[1]} 存在且可写`,
        en: `Directory ${match[1]} exists and is writable`,
      }),
    },
    {
      pattern: DIRECTORY_NOT_EXIST_RE,
      render: (match) => ({
        zh: `目录 ${match[1]} 不存在`,
        en: `Directory ${match[1]} does not exist`,
      }),
    },
    {
      pattern: DIRECTORY_NOT_WRITABLE_RE,
      render: (match) => ({
        zh: `目录 ${match[1]} 不可写：${match[2]}`,
        en: `Directory ${match[1]} is not writable: ${match[2]}`,
      }),
    },
    {
      pattern: DIRECTORY_NOTHING_TO_CLEAN_RE,
      render: (match) => ({
        zh: `目录 ${match[1]} 不存在，无需清理`,
        en: `Directory ${match[1]} does not exist, nothing to clean`,
      }),
    },
    {
      pattern: TCP_REACHABLE_RE,
      render: (match) => ({
        zh: `TCP 端点 ${match[1]} 可达`,
        en: `TCP endpoint ${match[1]} is reachable`,
      }),
    },
    {
      pattern: TCP_UNREACHABLE_RE,
      render: (match) => ({
        zh: `TCP 端点 ${match[1]} 不可达：${match[2]}`,
        en: `TCP endpoint ${match[1]} is not reachable: ${match[2]}`,
      }),
    },
    {
      pattern: INVALID_YAML_LINE_RE,
      render: (match) => ({
        zh: `${match[1]} YAML 格式无效：第 ${match[2]} 行附近存在缩进或结构错误（${match[3]}）。智能修复只会规范可解析的 YAML，请先手动修正后再试。`,
        en: `Invalid YAML in ${match[1]}: there is an indentation or structure error near line ${match[2]} (${match[3]}). Smart repair only normalizes parseable YAML, so please fix it manually first.`,
      }),
    },
    {
      pattern: INVALID_YAML_RE,
      render: (match) => ({
        zh: `${match[1]} YAML 格式无效：${match[2]}`,
        en: `Invalid YAML in ${match[1]}: ${match[2]}`,
      }),
    },
    {
      pattern: INVALID_ROOT_RE,
      render: (match) => ({
        zh: `${match[1]} 配置无效：缺少顶层键 ${match[2]}`,
        en: `Invalid ${match[1]}: expected top-level key '${match[2]}'`,
      }),
    },
  ];

  for (const item of structuredPatterns) {
    const match = item.pattern.exec(value);
    if (!match) {
      continue;
    }
    const localized = item.render(match);
    return locale === 'en' ? localized.en : localized.zh;
  }

  return null;
}

function countPatternMatches(value: string, pattern: RegExp): number {
  return value.match(pattern)?.length || 0;
}

function splitLocalizedPair(
  value: string,
): {zh: string; en: string} | null {
  let bestCandidate:
    | {
        zh: string;
        en: string;
        score: number;
      }
    | null = null;

  let startIndex = 0;
  while (startIndex < value.length) {
    const separatorIndex = value.indexOf(LOCALIZED_SEPARATOR, startIndex);
    if (separatorIndex < 0) {
      break;
    }

    const left = value.slice(0, separatorIndex).trim();
    const right = value
      .slice(separatorIndex + LOCALIZED_SEPARATOR.length)
      .trim();
    startIndex = separatorIndex + LOCALIZED_SEPARATOR.length;

    if (!left || !right) {
      continue;
    }

    const candidates = [
      {zh: left, en: right},
      {zh: right, en: left},
    ];

    for (const candidate of candidates) {
      const zhChineseCount = countPatternMatches(
        candidate.zh,
        /[\u4e00-\u9fff]/g,
      );
      const zhLatinCount = countPatternMatches(candidate.zh, /[A-Za-z]/g);
      const enChineseCount = countPatternMatches(
        candidate.en,
        /[\u4e00-\u9fff]/g,
      );
      const enLatinCount = countPatternMatches(candidate.en, /[A-Za-z]/g);

      if (zhChineseCount === 0 || enLatinCount === 0) {
        continue;
      }

      const score =
        zhChineseCount * 2 -
        zhLatinCount +
        enLatinCount * 2 -
        enChineseCount;

      if (!bestCandidate || score > bestCandidate.score) {
        bestCandidate = {...candidate, score};
      }
    }
  }

  if (!bestCandidate || bestCandidate.score <= 0) {
    return null;
  }

  return {
    zh: bestCandidate.zh,
    en: bestCandidate.en,
  };
}

export function localizeBackendText(
  value?: string | null,
  locale: Locale = getCurrentLocale(),
): string {
  const text = value?.trim() || '';
  if (!text) {
    return '';
  }

  const structured = localizeStructuredText(text, locale);
  if (structured) {
    return structured;
  }

  const localizedPair = splitLocalizedPair(text);
  if (localizedPair) {
    return locale === 'en' ? localizedPair.en : localizedPair.zh;
  }

  if (locale === 'zh' && HAS_CHINESE_PATTERN.test(text)) {
    return text;
  }
  if (locale === 'en' && HAS_LATIN_PATTERN.test(text)) {
    return text;
  }
  return text;
}
