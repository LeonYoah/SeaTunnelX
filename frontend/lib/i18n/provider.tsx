'use client';

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

import {
  createContext,
  useContext,
  useState,
  useEffect,
  useCallback,
  ReactNode,
} from 'react';
import {NextIntlClientProvider} from 'next-intl';
import {Locale, defaultLocale, getCurrentLocale, saveLocale} from './config';

import zhMessages from './locales/zh.json';
import enMessages from './locales/en.json';

type Messages = typeof zhMessages;

const messages: Record<Locale, Messages> = {
  zh: zhMessages,
  en: enMessages as Messages,
};

interface LocaleContextType {
  locale: Locale;
  setLocale: (locale: Locale) => void;
}

const LocaleContext = createContext<LocaleContextType | undefined>(undefined);

interface I18nProviderProps {
  children: ReactNode;
}

/**
 * 国际化 Provider 组件
 * 提供语言切换和翻译功能
 */
export function I18nProvider({children}: I18nProviderProps) {
  const [locale, setLocaleState] = useState<Locale>(defaultLocale);
  const [isHydrated, setIsHydrated] = useState(false);

  // 客户端初始化时检测语言
  useEffect(() => {
    const detectedLocale = getCurrentLocale();
    setLocaleState(detectedLocale);
    setIsHydrated(true);
  }, []);

  const setLocale = useCallback((newLocale: Locale) => {
    setLocaleState(newLocale);
    saveLocale(newLocale);
  }, []);

  // 在 hydration 完成前使用默认语言，避免 hydration mismatch
  const currentLocale = isHydrated ? locale : defaultLocale;
  const currentMessages = messages[currentLocale];

  return (
    <LocaleContext.Provider value={{locale: currentLocale, setLocale}}>
      <NextIntlClientProvider
        locale={currentLocale}
        messages={currentMessages}
        timeZone='Asia/Shanghai'
      >
        {children}
      </NextIntlClientProvider>
    </LocaleContext.Provider>
  );
}

/**
 * 获取当前语言和切换语言的 hook
 */
export function useLocale(): LocaleContextType {
  const context = useContext(LocaleContext);
  if (context === undefined) {
    throw new Error('useLocale must be used within an I18nProvider');
  }
  return context;
}
