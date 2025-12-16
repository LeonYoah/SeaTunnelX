/**
 * Copyright 2024 Apache Software Foundation
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import {getRequestConfig} from 'next-intl/server';

export type Locale = 'zh' | 'en';

export const locales: Locale[] = ['zh', 'en'];
export const defaultLocale: Locale = 'zh';

export const localeNames: Record<Locale, string> = {
  zh: '中文',
  en: 'English',
};

/**
 * 获取浏览器语言偏好
 */
export function getBrowserLocale(): Locale {
  if (typeof window === 'undefined') {
    return defaultLocale;
  }

  const browserLang = navigator.language.toLowerCase();

  if (browserLang.startsWith('zh')) {
    return 'zh';
  }

  if (browserLang.startsWith('en')) {
    return 'en';
  }

  return defaultLocale;
}

/**
 * 从 localStorage 获取保存的语言设置
 */
export function getSavedLocale(): Locale | null {
  if (typeof window === 'undefined') {
    return null;
  }

  const saved = localStorage.getItem('locale');
  if (saved && locales.includes(saved as Locale)) {
    return saved as Locale;
  }

  return null;
}

/**
 * 保存语言设置到 localStorage
 */
export function saveLocale(locale: Locale): void {
  if (typeof window !== 'undefined') {
    localStorage.setItem('locale', locale);
  }
}

/**
 * 获取当前应使用的语言
 */
export function getCurrentLocale(): Locale {
  const saved = getSavedLocale();
  if (saved) {
    return saved;
  }

  return getBrowserLocale();
}

export default getRequestConfig(async () => {
  const locale = defaultLocale;

  return {
    locale,
    messages: (await import(`./locales/${locale}.json`)).default,
  };
});
