'use client';

import {useState, useCallback} from 'react';
import {useTranslations} from 'next-intl';
import {motion, AnimatePresence} from 'motion/react';
import {
  MessageCircleQuestion,
  X,
  ExternalLink,
  Shield,
  AlertTriangle,
  Copy,
  Check,
  BookOpen,
  Search,
  HelpCircle,
} from 'lucide-react';
import {Button} from '@/components/ui/button';
import {Textarea} from '@/components/ui/textarea';
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip';

// 敏感信息过滤正则表达式
// Sensitive information filter regex patterns
const SENSITIVE_PATTERNS = [
  // IP 地址 / IP addresses
  {pattern: /\b(?:\d{1,3}\.){3}\d{1,3}\b/g, replacement: '[IP_ADDRESS]'},
  // 邮箱 / Email addresses
  {pattern: /\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b/g, replacement: '[EMAIL]'},
  // 密码字段 / Password fields
  {pattern: /password\s*[=:]\s*['"]?[^'"\s]+['"]?/gi, replacement: 'password=[REDACTED]'},
  // 密钥/Token / API keys and tokens
  {pattern: /(?:api[_-]?key|secret[_-]?key|access[_-]?token|auth[_-]?token)\s*[=:]\s*['"]?[A-Za-z0-9_\-]+['"]?/gi, replacement: '[API_KEY_REDACTED]'},
  // AWS 密钥 / AWS credentials
  {pattern: /(?:AKIA|ABIA|ACCA|ASIA)[A-Z0-9]{16}/g, replacement: '[AWS_KEY_REDACTED]'},
  // 私钥 / Private keys
  {pattern: /-----BEGIN\s+(?:RSA\s+)?PRIVATE\s+KEY-----[\s\S]*?-----END\s+(?:RSA\s+)?PRIVATE\s+KEY-----/g, replacement: '[PRIVATE_KEY_REDACTED]'},
  // 数据库连接字符串 / Database connection strings
  {pattern: /(?:jdbc|mysql|postgresql|mongodb):\/\/[^\s]+/gi, replacement: '[DB_CONNECTION_REDACTED]'},
  // 主机名 / Hostnames
  {pattern: /\b(?:host|hostname)\s*[=:]\s*['"]?[a-zA-Z0-9.-]+['"]?/gi, replacement: 'host=[HOSTNAME_REDACTED]'},
  // 用户名 / Usernames in connection strings
  {pattern: /(?:user|username)\s*[=:]\s*['"]?[^'"\s,;]+['"]?/gi, replacement: 'user=[USER_REDACTED]'},
  // 手机号 / Phone numbers
  {pattern: /\b1[3-9]\d{9}\b/g, replacement: '[PHONE]'},
];

/**
 * 过滤敏感信息
 * Filter sensitive information from text
 */
function filterSensitiveInfo(text: string): {filtered: string; hasFiltered: boolean} {
  let filtered = text;
  let hasFiltered = false;

  for (const {pattern, replacement} of SENSITIVE_PATTERNS) {
    const newText = filtered.replace(pattern, replacement);
    if (newText !== filtered) {
      hasFiltered = true;
      filtered = newText;
    }
  }

  return {filtered, hasFiltered};
}

export function AIAssistant() {
  const t = useTranslations('assistant');
  const [isOpen, setIsOpen] = useState(false);
  const [inputText, setInputText] = useState('');
  const [filteredText, setFilteredText] = useState('');
  const [hasFiltered, setHasFiltered] = useState(false);
  const [copied, setCopied] = useState(false);
  const [showCopyTip, setShowCopyTip] = useState(false);

  // 处理输入变化，实时过滤敏感信息
  // Handle input change with real-time sensitive info filtering
  const handleInputChange = useCallback((value: string) => {
    setInputText(value);
    const result = filterSensitiveInfo(value);
    setFilteredText(result.filtered);
    setHasFiltered(result.hasFiltered);
  }, []);

  // 复制过滤后的文本
  // Copy filtered text to clipboard
  const handleCopy = useCallback(async () => {
    try {
      await navigator.clipboard.writeText(filteredText);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      console.error('Failed to copy');
    }
  }, [filteredText]);

  // 在 DeepWiki 中打开并复制查询内容到剪贴板
  // Open DeepWiki and copy query to clipboard for pasting
  const searchInDeepWiki = useCallback(async () => {
    // 先复制过滤后的文本到剪贴板，方便用户粘贴到 DeepWiki 搜索框
    // Copy filtered text to clipboard first, so user can paste it to DeepWiki search box
    try {
      await navigator.clipboard.writeText(filteredText);
      // 显示提示
      // Show tip
      setShowCopyTip(true);
      setTimeout(() => setShowCopyTip(false), 3000);
    } catch {
      console.error('Failed to copy to clipboard');
    }
    // 打开 DeepWiki 页面
    // Open DeepWiki page
    window.open('https://deepwiki.com/apache/seatunnel', '_blank');
  }, [filteredText]);

  // 打开官方文档
  // Open official documentation
  const openOfficialDocs = useCallback(() => {
    window.open('https://seatunnel.apache.org/zh-CN/docs/about', '_blank');
  }, []);

  // 快捷链接 - 指向官方文档
  // Quick links - pointing to official documentation
  const quickLinks = [
    {
      icon: BookOpen,
      label: t('quickLinks.docs'),
      url: 'https://seatunnel.apache.org/zh-CN/docs/about',
    },
    {
      icon: HelpCircle,
      label: t('quickLinks.faq'),
      url: 'https://seatunnel.apache.org/zh-CN/docs/faq',
    },
    {
      icon: Search,
      label: t('quickLinks.connectors'),
      url: 'https://seatunnel.apache.org/zh-CN/docs/connector-v2/source',
    },
  ];

  return (
    <>
      {/* 悬浮按钮 / Floating button */}
      <TooltipProvider>
        <Tooltip>
          <TooltipTrigger asChild>
            <motion.button
              className='fixed bottom-6 right-6 z-50 w-14 h-14 rounded-full bg-gradient-to-r from-cyan-500 to-blue-500 text-white shadow-lg hover:shadow-xl flex items-center justify-center'
              whileHover={{scale: 1.1}}
              whileTap={{scale: 0.95}}
              onClick={() => setIsOpen(true)}
              aria-label={t('title')}
            >
              <MessageCircleQuestion className='w-6 h-6' />
            </motion.button>
          </TooltipTrigger>
          <TooltipContent side='left'>
            <p>{t('title')}</p>
          </TooltipContent>
        </Tooltip>
      </TooltipProvider>

      {/* 助手面板 / Assistant panel */}
      <AnimatePresence>
        {isOpen && (
          <motion.div
            initial={{opacity: 0, y: 100, scale: 0.9}}
            animate={{opacity: 1, y: 0, scale: 1}}
            exit={{opacity: 0, y: 100, scale: 0.9}}
            transition={{type: 'spring', damping: 25, stiffness: 300}}
            className='fixed z-50 bottom-24 right-6 w-[420px] max-h-[85vh] bg-background border rounded-xl shadow-2xl flex flex-col overflow-hidden'
          >
            {/* 头部 / Header */}
            <div className='flex items-center justify-between p-4 border-b bg-gradient-to-r from-cyan-500/10 to-blue-500/10'>
              <div className='flex items-center gap-2'>
                <MessageCircleQuestion className='w-5 h-5 text-cyan-500' />
                <span className='font-semibold'>{t('title')}</span>
              </div>
              <Button
                variant='ghost'
                size='icon'
                className='h-8 w-8'
                onClick={() => setIsOpen(false)}
              >
                <X className='w-4 h-4' />
              </Button>
            </div>

            {/* 内容区 / Content area */}
            <div className='flex-1 flex flex-col overflow-auto'>
              {/* 说明区域 / Description area */}
              <div className='p-4 space-y-3'>
                <p className='text-sm text-muted-foreground'>{t('description')}</p>
                <div className='flex items-start gap-2 p-3 bg-amber-500/10 border border-amber-500/20 rounded-lg'>
                  <Shield className='w-4 h-4 text-amber-500 mt-0.5 flex-shrink-0' />
                  <p className='text-xs text-amber-600 dark:text-amber-400'>
                    {t('privacyNotice')}
                  </p>
                </div>
              </div>

              {/* 快捷链接 / Quick links */}
              <div className='px-4 pb-4'>
                <label className='text-sm font-medium mb-2 block'>{t('quickLinksTitle')}</label>
                <div className='grid grid-cols-3 gap-2'>
                  {quickLinks.map((link) => (
                    <Button
                      key={link.label}
                      variant='outline'
                      size='sm'
                      className='h-auto py-3 flex flex-col gap-1'
                      onClick={() => window.open(link.url, '_blank')}
                    >
                      <link.icon className='w-4 h-4 text-cyan-500' />
                      <span className='text-xs'>{link.label}</span>
                    </Button>
                  ))}
                </div>
              </div>

              {/* 分隔线 / Divider */}
              <div className='px-4'>
                <div className='border-t' />
              </div>

              {/* 输入区域 / Input area */}
              <div className='p-4 space-y-3'>
                <label className='text-sm font-medium'>{t('inputLabel')}</label>
                <Textarea
                  placeholder={t('inputPlaceholder')}
                  value={inputText}
                  onChange={(e) => handleInputChange(e.target.value)}
                  className='min-h-[100px] resize-none text-sm'
                />
              </div>

              {/* 过滤预览 / Filtered preview */}
              {inputText && (
                <div className='px-4 pb-4 space-y-2'>
                  <div className='flex items-center justify-between'>
                    <label className='text-sm font-medium flex items-center gap-2'>
                      {t('filteredPreview')}
                      {hasFiltered && (
                        <span className='flex items-center gap-1 text-xs text-amber-500'>
                          <AlertTriangle className='w-3 h-3' />
                          {t('sensitiveFiltered')}
                        </span>
                      )}
                    </label>
                    <Button variant='ghost' size='sm' className='h-7 text-xs' onClick={handleCopy}>
                      {copied ? (
                        <Check className='w-3 h-3 mr-1' />
                      ) : (
                        <Copy className='w-3 h-3 mr-1' />
                      )}
                      {copied ? t('copied') : t('copy')}
                    </Button>
                  </div>
                  <div className='p-3 bg-muted rounded-lg text-xs font-mono whitespace-pre-wrap break-all max-h-[150px] overflow-auto'>
                    {filteredText}
                  </div>
                </div>
              )}
            </div>

            {/* 复制提示 / Copy tip */}
            <AnimatePresence>
              {showCopyTip && (
                <motion.div
                  initial={{opacity: 0, y: 10}}
                  animate={{opacity: 1, y: 0}}
                  exit={{opacity: 0, y: 10}}
                  className='mx-4 mb-2 p-2 bg-green-500/10 border border-green-500/20 rounded-lg'
                >
                  <p className='text-xs text-green-600 dark:text-green-400 flex items-center gap-1'>
                    <Check className='w-3 h-3' />
                    {t('openDeepWikiTip')}
                  </p>
                </motion.div>
              )}
            </AnimatePresence>

            {/* 底部操作 / Footer actions */}
            <div className='p-4 border-t bg-muted/30 flex items-center justify-between gap-2'>
              <Button variant='outline' size='sm' onClick={openOfficialDocs}>
                <BookOpen className='w-4 h-4 mr-2' />
                {t('viewDocs')}
              </Button>
              <Button
                size='sm'
                onClick={searchInDeepWiki}
                disabled={!filteredText}
                className='bg-gradient-to-r from-cyan-500 to-blue-500 hover:from-cyan-600 hover:to-blue-600'
              >
                <ExternalLink className='w-4 h-4 mr-2' />
                {t('openDeepWiki')}
              </Button>
            </div>
          </motion.div>
        )}
      </AnimatePresence>
    </>
  );
}
