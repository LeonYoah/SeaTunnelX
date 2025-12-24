/**
 * Plugin Card Component
 * 插件卡片组件
 */

'use client';

import { useTranslations } from 'next-intl';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Database, ExternalLink, Download, CheckCircle } from 'lucide-react';
import type { Plugin, PluginCategory } from '@/lib/services/plugin';

interface PluginCardProps {
  plugin: Plugin;
  onClick: () => void;
  showInstallButton?: boolean;
  isInstalled?: boolean;
  isDownloaded?: boolean;
  isDownloading?: boolean;
  downloadProgress?: number;
  onInstall?: () => void;
  onDownload?: () => void;
}

/**
 * Get category icon
 * 获取分类图标
 */
function getCategoryIcon(category: PluginCategory) {
  switch (category) {
    case 'connector':
      return <Database className="h-5 w-5" />;
    default:
      return <Database className="h-5 w-5" />;
  }
}

/**
 * Get category color
 * 获取分类颜色
 */
function getCategoryColor(category: PluginCategory): string {
  switch (category) {
    case 'connector':
      return 'bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-300';
    default:
      return 'bg-gray-100 text-gray-800 dark:bg-gray-900 dark:text-gray-300';
  }
}

/**
 * Plugin Card Component
 * 插件卡片组件 - 展示单个插件的基本信息
 */
export function PluginCard({ 
  plugin, 
  onClick,
  showInstallButton = false,
  isInstalled = false,
  isDownloaded = false,
  isDownloading = false,
  downloadProgress = 0,
  onInstall,
  onDownload,
}: PluginCardProps) {
  const t = useTranslations();

  /**
   * Handle install button click
   * 处理安装按钮点击
   */
  const handleInstallClick = (e: React.MouseEvent) => {
    e.stopPropagation();
    onInstall?.();
  };

  /**
   * Handle download button click
   * 处理下载按钮点击
   */
  const handleDownloadClick = (e: React.MouseEvent) => {
    e.stopPropagation();
    onDownload?.();
  };

  return (
    <Card
      className="cursor-pointer hover:shadow-md transition-shadow duration-200 hover:border-primary/50"
      onClick={onClick}
    >
      <CardHeader className="pb-3">
        <div className="flex items-start justify-between">
          <div className="flex items-center gap-3">
            <div className={`p-2 rounded-lg ${getCategoryColor(plugin.category)}`}>
              {getCategoryIcon(plugin.category)}
            </div>
            <div>
              <CardTitle className="text-base font-semibold line-clamp-1">
                {plugin.display_name || plugin.name}
              </CardTitle>
              <CardDescription className="text-xs mt-0.5">
                {plugin.name}
              </CardDescription>
            </div>
          </div>
          {plugin.doc_url && (
            <a
              href={plugin.doc_url}
              target="_blank"
              rel="noopener noreferrer"
              onClick={(e) => e.stopPropagation()}
              className="text-muted-foreground hover:text-primary"
            >
              <ExternalLink className="h-4 w-4" />
            </a>
          )}
        </div>
      </CardHeader>
      <CardContent className="pt-0">
        <p className="text-sm text-muted-foreground line-clamp-2 mb-3 min-h-[2.5rem]">
          {plugin.description || t('plugin.noDescription')}
        </p>
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <Badge variant="outline" className={getCategoryColor(plugin.category)}>
              {t(`plugin.category.${plugin.category}`)}
            </Badge>
          </div>
          <span className="text-xs text-muted-foreground">
            v{plugin.version}
          </span>
        </div>
        {plugin.dependencies && plugin.dependencies.length > 0 && (
          <div className="mt-2 text-xs text-muted-foreground">
            {t('plugin.dependencies')}: {plugin.dependencies.length}
          </div>
        )}
        
        {/* Install/Download/Status button / 安装/下载/状态按钮 */}
        {showInstallButton && (
          <div className="mt-3 pt-3 border-t">
            {isInstalled ? (
              <Button variant="outline" size="sm" className="w-full" disabled>
                <CheckCircle className="h-4 w-4 mr-2 text-green-600" />
                {t('plugin.installed')}
              </Button>
            ) : isDownloading ? (
              <div className="space-y-2">
                <div className="flex items-center justify-between text-xs text-muted-foreground">
                  <span>{t('plugin.downloading')}</span>
                  <span>{downloadProgress}%</span>
                </div>
                <div className="w-full bg-gray-200 rounded-full h-1.5 dark:bg-gray-700">
                  <div 
                    className="bg-blue-600 h-1.5 rounded-full transition-all duration-300" 
                    style={{ width: `${downloadProgress}%` }}
                  />
                </div>
              </div>
            ) : isDownloaded ? (
              <Button 
                variant="outline" 
                size="sm" 
                className="w-full"
                onClick={handleInstallClick}
              >
                <Download className="h-4 w-4 mr-2" />
                {t('plugin.install')}
              </Button>
            ) : (
              <Button 
                variant="outline" 
                size="sm" 
                className="w-full"
                onClick={handleDownloadClick}
              >
                <Download className="h-4 w-4 mr-2" />
                {t('plugin.download')}
              </Button>
            )}
          </div>
        )}
      </CardContent>
    </Card>
  );
}
