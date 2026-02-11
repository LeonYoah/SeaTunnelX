/**
 * Plugin Selection Step Component
 * 插件选择步骤组件
 *
 * Allows users to select plugins to install with SeaTunnel.
 * 允许用户选择要与 SeaTunnel 一起安装的插件。
 */

'use client';

import { useState, useMemo, useCallback } from 'react';
import { useTranslations } from 'next-intl';
import { useAvailablePlugins } from '@/hooks/use-plugin';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Checkbox } from '@/components/ui/checkbox';
import { ScrollArea } from '@/components/ui/scroll-area';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { cn } from '@/lib/utils';
import {
  Search,
  Package,
  ArrowDownToLine,
  ArrowUpFromLine,
  Shuffle,
  Info,
  Loader2,
  CheckCircle2,
  ExternalLink,
} from 'lucide-react';
import type { Plugin, PluginCategory, MirrorSource } from '@/lib/services/plugin/types';

interface PluginSelectStepProps {
  /** SeaTunnel version / SeaTunnel 版本 */
  version: string;
  /** Mirror source / 镜像源 */
  mirror: MirrorSource;
  /** Selected plugin names / 已选择的插件名称 */
  selectedPlugins: string[];
  /** Callback when plugins selection changes / 插件选择变化时的回调 */
  onPluginsChange: (plugins: string[]) => void;
}

// Category icons / 分类图标
const categoryIcons: Record<PluginCategory, React.ComponentType<{ className?: string }>> = {
  source: ArrowUpFromLine,
  sink: ArrowDownToLine,
  transform: Shuffle,
  connector: Package,
};

// Category colors / 分类颜色
const categoryColors: Record<PluginCategory, string> = {
  source: 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-300',
  sink: 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-300',
  transform: 'bg-purple-100 text-purple-700 dark:bg-purple-900/30 dark:text-purple-300',
  connector: 'bg-orange-100 text-orange-700 dark:bg-orange-900/30 dark:text-orange-300',
};

export function PluginSelectStep({
  version,
  mirror,
  selectedPlugins,
  onPluginsChange,
}: PluginSelectStepProps) {
  const t = useTranslations();
  const [searchQuery, setSearchQuery] = useState('');
  const [activeCategory, setActiveCategory] = useState<PluginCategory | 'all'>('all');
  const [detailPlugin, setDetailPlugin] = useState<Plugin | null>(null);

  // Fetch available plugins / 获取可用插件
  const {
    plugins,
    loading,
    error,
    filterByCategory,
    searchPlugins,
  } = useAvailablePlugins(version, mirror);

  // Filter and search plugins / 过滤和搜索插件
  const filteredPlugins = useMemo(() => {
    let result = plugins;

    // Filter by category / 按分类过滤
    if (activeCategory !== 'all') {
      result = filterByCategory(activeCategory);
    }

    // Search / 搜索
    if (searchQuery.trim()) {
      const lowerQuery = searchQuery.toLowerCase();
      result = result.filter(
        (p) =>
          p.name.toLowerCase().includes(lowerQuery) ||
          p.display_name.toLowerCase().includes(lowerQuery) ||
          p.description?.toLowerCase().includes(lowerQuery)
      );
    }

    return result;
  }, [plugins, activeCategory, searchQuery, filterByCategory]);

  // Group plugins by category / 按分类分组插件
  const pluginsByCategory = useMemo(() => {
    const groups: Record<PluginCategory, Plugin[]> = {
      source: [],
      sink: [],
      transform: [],
      connector: [],
    };

    filteredPlugins.forEach((plugin) => {
      if (groups[plugin.category]) {
        groups[plugin.category].push(plugin);
      }
    });

    return groups;
  }, [filteredPlugins]);

  // Handle plugin selection / 处理插件选择
  const handlePluginToggle = useCallback((pluginName: string) => {
    if (selectedPlugins.includes(pluginName)) {
      onPluginsChange(selectedPlugins.filter((p) => p !== pluginName));
    } else {
      onPluginsChange([...selectedPlugins, pluginName]);
    }
  }, [selectedPlugins, onPluginsChange]);

  // Handle select all in category / 处理选择分类中的所有插件
  const handleSelectAllInCategory = useCallback((category: PluginCategory) => {
    const categoryPlugins = pluginsByCategory[category].map((p) => p.name);
    const allSelected = categoryPlugins.every((p) => selectedPlugins.includes(p));

    if (allSelected) {
      // Deselect all / 取消全选
      onPluginsChange(selectedPlugins.filter((p) => !categoryPlugins.includes(p)));
    } else {
      // Select all / 全选
      const newSelection = [...new Set([...selectedPlugins, ...categoryPlugins])];
      onPluginsChange(newSelection);
    }
  }, [pluginsByCategory, selectedPlugins, onPluginsChange]);

  // Check if all plugins in category are selected / 检查分类中的所有插件是否都已选择
  const isCategoryAllSelected = useCallback((category: PluginCategory) => {
    const categoryPlugins = pluginsByCategory[category];
    if (categoryPlugins.length === 0) return false;
    return categoryPlugins.every((p) => selectedPlugins.includes(p.name));
  }, [pluginsByCategory, selectedPlugins]);

  // Render plugin card / 渲染插件卡片
  const renderPluginCard = (plugin: Plugin) => {
    const isSelected = selectedPlugins.includes(plugin.name);
    const CategoryIcon = categoryIcons[plugin.category] || Package;

    return (
      <div
        key={plugin.name}
        className={cn(
          'relative flex items-start gap-3 p-3 rounded-lg border transition-colors cursor-pointer',
          isSelected
            ? 'border-primary bg-primary/5'
            : 'border-muted hover:border-muted-foreground/50'
        )}
        onClick={() => handlePluginToggle(plugin.name)}
      >
        {/* Checkbox / 复选框 */}
        <Checkbox
          checked={isSelected}
          onCheckedChange={() => handlePluginToggle(plugin.name)}
          className="mt-1"
        />

        {/* Plugin info / 插件信息 */}
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2 mb-1">
            <span className="font-medium text-sm truncate">
              {plugin.display_name || plugin.name}
            </span>
            <Badge
              variant="outline"
              className={cn('text-xs', categoryColors[plugin.category])}
            >
              <CategoryIcon className="h-3 w-3 mr-1" />
              {t(`plugin.category.${plugin.category}`)}
            </Badge>
          </div>

          <p className="text-xs text-muted-foreground line-clamp-2">
            {plugin.description || t('plugin.noDescription')}
          </p>

          {/* Dependencies indicator / 依赖指示器 */}
          {plugin.dependencies && plugin.dependencies.length > 0 && (
            <div className="mt-1 flex items-center gap-1 text-xs text-muted-foreground">
              <Package className="h-3 w-3" />
              <span>{plugin.dependencies.length} {t('plugin.dependencies')}</span>
            </div>
          )}
        </div>

        {/* Info button / 信息按钮 */}
        <Button
          variant="ghost"
          size="icon"
          className="h-6 w-6 flex-shrink-0"
          onClick={(e) => {
            e.stopPropagation();
            setDetailPlugin(plugin);
          }}
        >
          <Info className="h-4 w-4" />
        </Button>
      </div>
    );
  };

  // Render category section / 渲染分类部分
  const renderCategorySection = (category: PluginCategory, plugins: Plugin[]) => {
    if (plugins.length === 0) return null;

    const CategoryIcon = categoryIcons[category];
    const allSelected = isCategoryAllSelected(category);
    const selectedCount = plugins.filter((p) => selectedPlugins.includes(p.name)).length;

    return (
      <div key={category} className="space-y-3">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <CategoryIcon className="h-4 w-4" />
            <span className="font-medium">{t(`plugin.category.${category}`)}</span>
            <Badge variant="secondary" className="text-xs">
              {selectedCount}/{plugins.length}
            </Badge>
          </div>
          <Button
            variant="ghost"
            size="sm"
            onClick={() => handleSelectAllInCategory(category)}
          >
            {allSelected ? t('common.deselectAll') || 'Deselect All' : t('common.selectAll') || 'Select All'}
          </Button>
        </div>

        <div className="grid grid-cols-1 md:grid-cols-2 gap-2">
          {plugins.map(renderPluginCard)}
        </div>
      </div>
    );
  };

  return (
    <div className="space-y-4">
      {/* Header / 头部 */}
      <Card>
        <CardHeader className="pb-3">
          <div className="flex items-center justify-between">
            <div>
              <CardTitle className="text-lg flex items-center gap-2">
                <Package className="h-5 w-5" />
                {t('installer.pluginSelection') || 'Plugin Selection'}
              </CardTitle>
              <CardDescription>
                {t('installer.pluginSelectionDesc') || 'Select plugins to install with SeaTunnel'}
              </CardDescription>
            </div>
            <Badge variant="outline" className="gap-1">
              <CheckCircle2 className="h-3 w-3" />
              {selectedPlugins.length} {t('plugin.selectedCount') || 'selected'}
            </Badge>
          </div>
        </CardHeader>
        <CardContent>
          {/* Search and filter / 搜索和过滤 */}
          <div className="flex items-center gap-4">
            <div className="relative flex-1">
              <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
              <Input
                placeholder={t('plugin.searchPlaceholder')}
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                className="pl-9"
              />
            </div>
          </div>

          {/* Category tabs / 分类标签 */}
          <Tabs
            value={activeCategory}
            onValueChange={(v) => setActiveCategory(v as PluginCategory | 'all')}
            className="mt-4"
          >
            <TabsList>
              <TabsTrigger value="all" className="gap-1">
                <Package className="h-4 w-4" />
                {t('plugin.category.all')}
              </TabsTrigger>
              <TabsTrigger value="source" className="gap-1">
                <ArrowUpFromLine className="h-4 w-4" />
                {t('plugin.category.source')}
              </TabsTrigger>
              <TabsTrigger value="sink" className="gap-1">
                <ArrowDownToLine className="h-4 w-4" />
                {t('plugin.category.sink')}
              </TabsTrigger>
              <TabsTrigger value="transform" className="gap-1">
                <Shuffle className="h-4 w-4" />
                {t('plugin.category.transform')}
              </TabsTrigger>
            </TabsList>
          </Tabs>
        </CardContent>
      </Card>

      {/* Plugin list / 插件列表 */}
      <Card>
        <CardContent className="pt-6">
          {loading ? (
            <div className="flex items-center justify-center py-12">
              <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
            </div>
          ) : error ? (
            <div className="text-center py-12 text-destructive">
              <p>{error}</p>
            </div>
          ) : filteredPlugins.length === 0 ? (
            <div className="text-center py-12 text-muted-foreground">
              <Package className="h-12 w-12 mx-auto mb-4 opacity-50" />
              <p>{t('plugin.noPluginsFound')}</p>
            </div>
          ) : (
            <ScrollArea className="h-[350px] pr-4">
              <div className="space-y-6">
                {activeCategory === 'all' ? (
                  // Show all categories / 显示所有分类
                  <>
                    {renderCategorySection('source', pluginsByCategory.source)}
                    {renderCategorySection('sink', pluginsByCategory.sink)}
                    {renderCategorySection('transform', pluginsByCategory.transform)}
                  </>
                ) : (
                  // Show single category / 显示单个分类
                  <div className="grid grid-cols-1 md:grid-cols-2 gap-2">
                    {filteredPlugins.map(renderPluginCard)}
                  </div>
                )}
              </div>
            </ScrollArea>
          )}
        </CardContent>
      </Card>

      {/* Plugin detail dialog / 插件详情对话框 */}
      <Dialog open={!!detailPlugin} onOpenChange={() => setDetailPlugin(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              {detailPlugin?.display_name || detailPlugin?.name}
              {detailPlugin && (
                <Badge
                  variant="outline"
                  className={cn('text-xs', categoryColors[detailPlugin.category])}
                >
                  {t(`plugin.category.${detailPlugin.category}`)}
                </Badge>
              )}
            </DialogTitle>
            <DialogDescription>
              {detailPlugin?.description || t('plugin.noDescription')}
            </DialogDescription>
          </DialogHeader>

          {detailPlugin && (
            <div className="space-y-4">
              {/* Version / 版本 */}
              <div>
                <h4 className="text-sm font-medium mb-1">{t('plugin.version')}</h4>
                <p className="text-sm text-muted-foreground">{detailPlugin.version}</p>
              </div>

              {/* Maven coordinates / Maven 坐标 */}
              <div>
                <h4 className="text-sm font-medium mb-1">{t('plugin.mavenCoordinates')}</h4>
                <code className="text-xs bg-muted px-2 py-1 rounded">
                  {detailPlugin.group_id}:{detailPlugin.artifact_id}:{detailPlugin.version}
                </code>
              </div>

              {/* Dependencies / 依赖 */}
              {detailPlugin.dependencies && detailPlugin.dependencies.length > 0 && (
                <div>
                  <h4 className="text-sm font-medium mb-2">{t('plugin.dependencies')}</h4>
                  <div className="space-y-1">
                    {detailPlugin.dependencies.map((dep, index) => (
                      <div
                        key={index}
                        className="text-xs bg-muted px-2 py-1 rounded flex items-center justify-between"
                      >
                        <code>{dep.group_id}:{dep.artifact_id}:{dep.version}</code>
                        <span className="text-muted-foreground">→ {dep.target_dir}</span>
                      </div>
                    ))}
                  </div>
                </div>
              )}

              {/* Install path / 安装路径 */}
              <div>
                <h4 className="text-sm font-medium mb-1">{t('plugin.installPath')}</h4>
                <div className="text-sm text-muted-foreground space-y-1">
                  <p>• {t('plugin.connectorPath')}: connectors/</p>
                  <p>• {t('plugin.libPath')}: lib/</p>
                </div>
              </div>

              {/* Documentation link / 文档链接 */}
              {detailPlugin.doc_url && (
                <Button variant="outline" size="sm" asChild>
                  <a href={detailPlugin.doc_url} target="_blank" rel="noopener noreferrer">
                    <ExternalLink className="h-4 w-4 mr-2" />
                    {t('plugin.viewDocumentation')}
                  </a>
                </Button>
              )}
            </div>
          )}
        </DialogContent>
      </Dialog>
    </div>
  );
}

export default PluginSelectStep;
