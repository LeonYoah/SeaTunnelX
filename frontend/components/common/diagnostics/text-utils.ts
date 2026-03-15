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

import {getCurrentLocale, type Locale} from '@/lib/i18n/config';

const HAS_CHINESE_PATTERN = /[\u4e00-\u9fff]/;
const HAS_LATIN_PATTERN = /[A-Za-z]/;
const LOCALIZED_SEPARATOR = ' / ';

const EXACT_TEXT_MAP: Record<
  string,
  {
    zh: string;
    en: string;
  }
> = {
  '汇总错误上下文': {
    zh: '汇总错误上下文',
    en: 'Collect Error Context',
  },
  '收集进程事件': {
    zh: '收集进程事件',
    en: 'Collect Process Events',
  },
  '收集告警快照': {
    zh: '收集告警快照',
    en: 'Collect Alert Snapshot',
  },
  '收集配置快照': {
    zh: '收集配置快照',
    en: 'Collect Config Snapshot',
  },
  '收集日志样本': {
    zh: '收集日志样本',
    en: 'Collect Log Sample',
  },
  '收集线程栈': {
    zh: '收集线程栈',
    en: 'Collect Thread Dump',
  },
  '收集 JVM Dump': {
    zh: '收集 JVM Dump',
    en: 'Collect JVM Dump',
  },
  '生成 Manifest': {
    zh: '生成 Manifest',
    en: 'Assemble Manifest',
  },
  '生成诊断报告': {
    zh: '生成诊断报告',
    en: 'Render Diagnostic Report',
  },
  '完成': {
    zh: '完成',
    en: 'Complete',
  },
};

const REPLACERS: Array<{
  pattern: RegExp;
  replace: (...args: string[]) => {zh: string; en: string};
}> = [
  {
    pattern: /^Diagnostic bundle created from error group #(\d+)$/,
    replace: (id) => ({
      zh: `错误组 #${id} 触发的诊断包`,
      en: `Diagnostic bundle created from error group #${id}`,
    }),
  },
  {
    pattern: /^Diagnostic bundle created from inspection finding #(\d+)$/,
    replace: (id) => ({
      zh: `巡检发现 #${id} 触发的诊断包`,
      en: `Diagnostic bundle created from inspection finding #${id}`,
    }),
  },
  {
    pattern: /^Diagnostic bundle created from alert (.+)$/,
    replace: (id) => ({
      zh: `告警 ${id} 触发的诊断包`,
      en: `Diagnostic bundle created from alert ${id}`,
    }),
  },
  {
    pattern: /^Manual diagnostic bundle task$/,
    replace: () => ({
      zh: '手动创建的诊断包任务',
      en: 'Manual diagnostic bundle task',
    }),
  },
  {
    pattern: /^Step completed\.$/,
    replace: () => ({
      zh: '步骤执行完成。',
      en: 'Step completed.',
    }),
  },
  {
    pattern: /^Diagnostic task completed\.$/,
    replace: () => ({
      zh: '诊断任务执行完成。',
      en: 'Diagnostic task completed.',
    }),
  },
  {
    pattern: /^Thread dump is disabled by task options\.$/,
    replace: () => ({
      zh: '任务配置未开启线程栈采集。',
      en: 'Thread dump is disabled by task options.',
    }),
  },
  {
    pattern: /^JVM dump is disabled by task options\.$/,
    replace: () => ({
      zh: '任务配置未开启 JVM Dump 采集。',
      en: 'JVM dump is disabled by task options.',
    }),
  },
  {
    pattern: /^Thread dump collected\.$/,
    replace: () => ({
      zh: '线程栈采集完成。',
      en: 'Thread dump collected.',
    }),
  },
  {
    pattern: /^Log sample collected\.$/,
    replace: () => ({
      zh: '日志样本采集完成。',
      en: 'Log sample collected.',
    }),
  },
  {
    pattern: /^No log sample collected\.$/,
    replace: () => ({
      zh: '未采集到日志样本。',
      en: 'No log sample collected.',
    }),
  },
  {
    pattern: /^Thread dump collected to (.+)$/,
    replace: (path) => ({
      zh: `线程栈已保存到 ${path}`,
      en: `Thread dump collected to ${path}`,
    }),
  },
  {
    pattern: /^Collected log sample from (.+)$/,
    replace: (path) => ({
      zh: `已从 ${path} 采集日志样本`,
      en: `Collected log sample from ${path}`,
    }),
  },
  {
    pattern: /^Failed to collect log sample from (.+): (.+)$/,
    replace: (path, detail) => ({
      zh: `从 ${path} 采集日志样本失败：${detail}`,
      en: `Failed to collect log sample from ${path}: ${detail}`,
    }),
  },
  {
    pattern: /^Thread dump failed: (.+)$/,
    replace: (detail) => ({
      zh: `线程栈采集失败：${detail}`,
      en: `Thread dump failed: ${detail}`,
    }),
  },
  {
    pattern: /^thread dump failed for all nodes: (.+)$/i,
    replace: (detail) => ({
      zh: `全部节点线程栈采集失败：${detail}`,
      en: `Thread dump failed on all nodes: ${detail}`,
    }),
  },
  {
    pattern: /^JVM dump failed: (.+)$/,
    replace: (detail) => ({
      zh: `JVM Dump 采集失败：${detail}`,
      en: `JVM dump failed: ${detail}`,
    }),
  },
  {
    pattern: /^jvm dump failed for all nodes: (.+)$/i,
    replace: (detail) => ({
      zh: `全部节点 JVM Dump 采集失败：${detail}`,
      en: `JVM dump failed on all nodes: ${detail}`,
    }),
  },
  {
    pattern: /^no log samples collected: (.+)$/i,
    replace: (detail) => ({
      zh: `全部节点都未采集到日志样本：${detail}`,
      en: `No log samples collected: ${detail}`,
    }),
  },
  {
    pattern: /^Created (.+)$/,
    replace: (name) => ({
      zh: `已生成 ${name}`,
      en: `Created ${name}`,
    }),
  },
  {
    pattern: /^(.+) inspection generated (\d+) findings \((\d+) critical \/ (\d+) warning \/ (\d+) info\)$/,
    replace: (cluster, total, critical, warning, info) => ({
      zh: `${cluster} 巡检生成 ${total} 条发现（严重 ${critical} / 告警 ${warning} / 信息 ${info}）`,
      en: `${cluster} inspection generated ${total} findings (${critical} critical / ${warning} warning / ${info} info)`,
    }),
  },
];

function pickLocalizedText(
  localized: {zh: string; en: string},
  locale: Locale,
): string {
  return locale === 'en' ? localized.en : localized.zh;
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

    const zh = value.slice(0, separatorIndex).trim();
    const en = value
      .slice(separatorIndex + LOCALIZED_SEPARATOR.length)
      .trim();
    startIndex = separatorIndex + LOCALIZED_SEPARATOR.length;

    if (!zh || !en) {
      continue;
    }

    const zhChineseCount = countPatternMatches(zh, /[\u4e00-\u9fff]/g);
    const zhLatinCount = countPatternMatches(zh, /[A-Za-z]/g);
    const enChineseCount = countPatternMatches(en, /[\u4e00-\u9fff]/g);
    const enLatinCount = countPatternMatches(en, /[A-Za-z]/g);

    if (zhChineseCount === 0 || enLatinCount === 0) {
      continue;
    }

    const score =
      zhChineseCount * 2 -
      zhLatinCount +
      enLatinCount * 2 -
      enChineseCount;

    if (!bestCandidate || score > bestCandidate.score) {
      bestCandidate = {zh, en, score};
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

export function localizeDiagnosticsText(
  value?: string | null,
  locale: Locale = getCurrentLocale(),
): string {
  const text = value?.trim() || '';
  if (!text) {
    return '';
  }

  const localizedPair = splitLocalizedPair(text);
  if (localizedPair) {
    return locale === 'en' ? localizedPair.en : localizedPair.zh;
  }

  const exact = EXACT_TEXT_MAP[text];
  if (exact) {
    return pickLocalizedText(exact, locale);
  }

  for (const item of REPLACERS) {
    const match = text.match(item.pattern);
    if (!match) {
      continue;
    }
    return pickLocalizedText(item.replace(...match.slice(1)), locale);
  }

  if (locale === 'zh' && HAS_CHINESE_PATTERN.test(text)) {
    return text;
  }
  if (locale === 'en' && HAS_LATIN_PATTERN.test(text)) {
    return text;
  }
  return text;
}
