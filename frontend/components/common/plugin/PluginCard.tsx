/**
 * Plugin Card Component
 * 插件卡片组件
 */

'use client';

import { useTranslations } from 'next-intl';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Database, Upload, Shuffle, ExternalLink, Download, CheckCircle } from 'lucide-react';
import type { Plugin, PluginCategory } from '@/lib/services/plugin';

interface PluginCardProps {
  plugin: Plugin;
  onClick: () => void;
  showInstallButton?: boolean;
  isBuiltIn?: boolean;
  isInstalled?: boolean;
  onInstall?: () => void;
}

/**
 * Get category icon
 * 获取分类图标
 */
function getCategoryIcon(category: PluginCategory) {
  switch (category) {
    case 'source':
      return <Database className="h-5 w-5" />;
    case 'sink':
      return <Upload className="h-5 w-5" />;
    case 'transform':
      return <Shuffle className="h-5 w-5" />;
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
    case 'source':
      return 'bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-300';
    case 'sink':
      return 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-300';
    case 'transform':
      return 'bg-purple-100 text-purple-800 dark:bg-purple-900 dark:text-purple-300';
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
  isBuiltIn = false,
  isInstalled = false,
  onInstall,
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
            {isBuiltIn && (
              <Badge variant="secondary" className="bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-300">
                {t('plugin.builtIn')}
              </Badge>
            )}
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
        
        {/* Install/Status button / 安装/状态按钮 */}
        {showInstallButton && !isBuiltIn && (
          <div className="mt-3 pt-3 border-t">
            {isInstalled ? (
              <Button variant="outline" size="sm" className="w-full" disabled>
                <CheckCircle className="h-4 w-4 mr-2 text-green-600" />
                {t('plugin.installed')}
              </Button>
            ) : (
              <Button 
                variant="outline" 
                size="sm" 
                className="w-full"
                onClick={handleInstallClick}
              >
                <Download className="h-4 w-4 mr-2" />
                {t('plugin.install')}
              </Button>
            )}
          </div>
        )}
        
        {/* Built-in indicator / 内置指示器 */}
        {isBuiltIn && (
          <div className="mt-3 pt-3 border-t">
            <div className="flex items-center justify-center gap-2 text-sm text-green-600">
              <CheckCircle className="h-4 w-4" />
              {t('plugin.alwaysEnabled')}
            </div>
          </div>
        )}
      </CardContent>
    </Card>
  );
}
