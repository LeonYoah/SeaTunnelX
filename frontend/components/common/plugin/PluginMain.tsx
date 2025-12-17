/**
 * Plugin Marketplace Main Component
 * 插件市场主组件
 */

'use client';

import { useState, useEffect, useCallback } from 'react';
import { useTranslations } from 'next-intl';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { Separator } from '@/components/ui/separator';
import { Badge } from '@/components/ui/badge';
import { toast } from 'sonner';
import { RefreshCw, Search, Puzzle, Package, Download, CheckCircle, Info, Upload } from 'lucide-react';
import { motion } from 'motion/react';
import { PluginService } from '@/lib/services/plugin';
import type { Plugin, MirrorSource, AvailablePluginsResponse } from '@/lib/services/plugin';
import { PluginGrid } from './PluginGrid';
import { PluginDetailDialog } from './PluginDetailDialog';
import { InstallPluginDialog } from './InstallPluginDialog';

// Available SeaTunnel versions / 可用的 SeaTunnel 版本
const AVAILABLE_VERSIONS = [
  '2.3.12',
  '2.3.11',
  '2.3.10',
  '2.3.9',
  '2.3.8',
  '2.3.7',
  '2.3.6',
  '2.3.5',
];

/**
 * Plugin Marketplace Main Component
 * 插件市场主组件
 */
export function PluginMain() {
  const t = useTranslations();

  // Data state / 数据状态
  const [plugins, setPlugins] = useState<Plugin[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Filter state / 过滤状态
  const [searchKeyword, setSearchKeyword] = useState('');
  const [filterCategory, setFilterCategory] = useState<string>('all');
  const [selectedMirror, setSelectedMirror] = useState<MirrorSource>('aliyun');
  const [selectedVersion, setSelectedVersion] = useState<string>('2.3.12');
  const [activeTab, setActiveTab] = useState<'available' | 'installed' | 'custom'>('available');

  // Dialog state / 对话框状态
  const [selectedPlugin, setSelectedPlugin] = useState<Plugin | null>(null);
  const [isDetailOpen, setIsDetailOpen] = useState(false);
  const [isInstallOpen, setIsInstallOpen] = useState(false);
  const [pluginToInstall, setPluginToInstall] = useState<Plugin | null>(null);

  /**
   * Load available plugins
   * 加载可用插件列表
   */
  const loadPlugins = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const result: AvailablePluginsResponse = await PluginService.listAvailablePlugins(
        selectedVersion || undefined,
        selectedMirror
      );
      
      let filteredPlugins = result.plugins || [];
      
      // Apply category filter / 应用分类过滤
      if (filterCategory !== 'all') {
        filteredPlugins = filteredPlugins.filter(p => p.category === filterCategory);
      }
      
      // Apply search filter / 应用搜索过滤
      if (searchKeyword) {
        const keyword = searchKeyword.toLowerCase();
        filteredPlugins = filteredPlugins.filter(p => 
          p.name.toLowerCase().includes(keyword) ||
          p.display_name.toLowerCase().includes(keyword) ||
          p.description.toLowerCase().includes(keyword)
        );
      }
      
      setPlugins(filteredPlugins);
      setTotal(result.total || filteredPlugins.length);
      if (result.version && !selectedVersion) {
        setSelectedVersion(result.version);
      }
    } catch (err) {
      const errorMsg = err instanceof Error ? err.message : t('plugin.loadError');
      setError(errorMsg);
      toast.error(errorMsg);
      setPlugins([]);
      setTotal(0);
    } finally {
      setLoading(false);
    }
  }, [selectedVersion, selectedMirror, filterCategory, searchKeyword, t]);

  useEffect(() => {
    loadPlugins();
  }, [loadPlugins]);

  /**
   * Handle search
   * 处理搜索
   */
  const handleSearch = () => {
    loadPlugins();
  };

  /**
   * Handle refresh
   * 处理刷新
   */
  const handleRefresh = () => {
    loadPlugins();
  };

  /**
   * Handle view plugin detail
   * 处理查看插件详情
   */
  const handleViewDetail = (plugin: Plugin) => {
    setSelectedPlugin(plugin);
    setIsDetailOpen(true);
  };

  /**
   * Clear all filters
   * 清除所有过滤条件
   */
  const handleClearFilters = () => {
    setSearchKeyword('');
    setFilterCategory('all');
  };

  /**
   * Handle install plugin
   * 处理安装插件
   */
  const handleInstallPlugin = (plugin: Plugin) => {
    setPluginToInstall(plugin);
    setIsInstallOpen(true);
  };

  // Count plugins by category / 按分类统计插件数量
  const sourceCount = plugins.filter(p => p.category === 'source').length;
  const sinkCount = plugins.filter(p => p.category === 'sink').length;
  const transformCount = plugins.filter(p => p.category === 'transform').length;

  const containerVariants = {
    hidden: { opacity: 0 },
    visible: {
      opacity: 1,
      transition: {
        duration: 0.5,
        staggerChildren: 0.1,
        ease: 'easeOut',
      },
    },
  };

  const itemVariants = {
    hidden: { opacity: 0, y: 20 },
    visible: {
      opacity: 1,
      y: 0,
      transition: { duration: 0.6, ease: 'easeOut' },
    },
  };

  return (
    <motion.div
      className="space-y-6"
      initial="hidden"
      animate="visible"
      variants={containerVariants}
    >
      {/* Header / 标题 */}
      <motion.div
        className="flex items-center justify-between"
        variants={itemVariants}
      >
        <div className="flex items-center gap-2">
          <Puzzle className="h-6 w-6" />
          <div>
            <h1 className="text-2xl font-bold tracking-tight">
              {t('plugin.marketplace')}
            </h1>
            <p className="text-muted-foreground mt-1">
              {t('plugin.marketplaceDesc')}
            </p>
          </div>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" onClick={handleRefresh} disabled={loading}>
            <RefreshCw className={`h-4 w-4 mr-2 ${loading ? 'animate-spin' : ''}`} />
            {t('common.refresh')}
          </Button>
        </div>
      </motion.div>

      <Separator />

      {/* Stats cards / 统计卡片 */}
      <motion.div
        className="grid grid-cols-1 md:grid-cols-4 gap-4"
        variants={itemVariants}
      >
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground flex items-center gap-2">
              <Package className="h-4 w-4" />
              {t('plugin.totalPlugins')}
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{total}</div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground flex items-center gap-2">
              <Download className="h-4 w-4" />
              {t('plugin.category.source')}
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-blue-600">{sourceCount}</div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground flex items-center gap-2">
              <CheckCircle className="h-4 w-4" />
              {t('plugin.category.sink')}
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-green-600">{sinkCount}</div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground flex items-center gap-2">
              <Puzzle className="h-4 w-4" />
              {t('plugin.category.transform')}
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-purple-600">{transformCount}</div>
          </CardContent>
        </Card>
      </motion.div>

      {/* Error display / 错误显示 */}
      {error && (
        <Card className="border-destructive">
          <CardContent className="pt-6">
            <p className="text-destructive">{error}</p>
          </CardContent>
        </Card>
      )}

      {/* Filters / 过滤器 */}
      <motion.div
        className="flex flex-wrap gap-4 items-end"
        variants={itemVariants}
      >
        <div className="flex-1 min-w-[200px] max-w-sm">
          <Input
            placeholder={t('plugin.searchPlaceholder')}
            value={searchKeyword}
            onChange={(e) => setSearchKeyword(e.target.value)}
            onKeyDown={(e) => e.key === 'Enter' && handleSearch()}
          />
        </div>

        {/* Version selector / 版本选择器 */}
        <Select value={selectedVersion} onValueChange={setSelectedVersion}>
          <SelectTrigger className="w-[130px]">
            <SelectValue placeholder={t('plugin.version')} />
          </SelectTrigger>
          <SelectContent>
            {AVAILABLE_VERSIONS.map((version) => (
              <SelectItem key={version} value={version}>
                v{version}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>

        <Select value={filterCategory} onValueChange={setFilterCategory}>
          <SelectTrigger className="w-[150px]">
            <SelectValue placeholder={t('plugin.category.all')} />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">{t('plugin.category.all')}</SelectItem>
            <SelectItem value="source">{t('plugin.category.source')}</SelectItem>
            <SelectItem value="sink">{t('plugin.category.sink')}</SelectItem>
            <SelectItem value="transform">{t('plugin.category.transform')}</SelectItem>
          </SelectContent>
        </Select>

        <Select value={selectedMirror} onValueChange={(v) => setSelectedMirror(v as MirrorSource)}>
          <SelectTrigger className="w-[150px]">
            <SelectValue placeholder={t('plugin.mirror')} />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="aliyun">{t('installer.mirrors.aliyun')}</SelectItem>
            <SelectItem value="huaweicloud">{t('installer.mirrors.huaweicloud')}</SelectItem>
            <SelectItem value="apache">{t('installer.mirrors.apache')}</SelectItem>
          </SelectContent>
        </Select>

        <Button variant="outline" onClick={handleSearch}>
          <Search className="h-4 w-4 mr-2" />
          {t('common.search')}
        </Button>

        <Button variant="ghost" onClick={handleClearFilters}>
          {t('common.clearFilters')}
        </Button>
      </motion.div>

      {/* Transform info banner / Transform 信息横幅 */}
      {filterCategory === 'transform' && (
        <motion.div variants={itemVariants}>
          <Card className="border-blue-200 bg-blue-50 dark:border-blue-800 dark:bg-blue-950">
            <CardContent className="pt-4 pb-4">
              <div className="flex items-start gap-3">
                <Info className="h-5 w-5 text-blue-600 mt-0.5" />
                <div>
                  <p className="text-sm font-medium text-blue-800 dark:text-blue-200">
                    {t('plugin.transformBuiltIn')}
                  </p>
                  <p className="text-sm text-blue-600 dark:text-blue-300 mt-1">
                    {t('plugin.transformBuiltInDesc')}
                  </p>
                </div>
              </div>
            </CardContent>
          </Card>
        </motion.div>
      )}

      {/* Plugin tabs / 插件标签页 */}
      <Tabs value={activeTab} onValueChange={(v) => setActiveTab(v as 'available' | 'installed' | 'custom')}>
        <TabsList>
          <TabsTrigger value="available" className="flex items-center gap-2">
            <Package className="h-4 w-4" />
            {t('plugin.available')}
          </TabsTrigger>
          <TabsTrigger value="installed" className="flex items-center gap-2">
            <CheckCircle className="h-4 w-4" />
            {t('plugin.installed')}
          </TabsTrigger>
          <TabsTrigger value="custom" className="flex items-center gap-2">
            <Upload className="h-4 w-4" />
            {t('plugin.custom')}
          </TabsTrigger>
        </TabsList>

        <TabsContent value="available" className="mt-4">
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2">
                {t('plugin.available')}
                <Badge variant="secondary">v{selectedVersion}</Badge>
              </CardTitle>
              <CardDescription>
                {t('plugin.availableDesc')}
              </CardDescription>
            </CardHeader>
            <CardContent>
              <PluginGrid
                plugins={plugins}
                loading={loading}
                onViewDetail={handleViewDetail}
                showInstallButton={true}
                isTransformBuiltIn={true}
                onInstall={handleInstallPlugin}
              />
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="installed" className="mt-4">
          <Card>
            <CardHeader>
              <CardTitle>{t('plugin.installed')}</CardTitle>
              <CardDescription>
                {t('plugin.installedDesc')}
              </CardDescription>
            </CardHeader>
            <CardContent>
              <div className="text-center py-8 text-muted-foreground">
                <p>{t('plugin.selectClusterToViewInstalled')}</p>
                <p className="text-sm mt-2">{t('plugin.goToClusterDetail')}</p>
              </div>
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="custom" className="mt-4">
          <Card>
            <CardHeader>
              <CardTitle>{t('plugin.custom')}</CardTitle>
              <CardDescription>
                {t('plugin.customDesc')}
              </CardDescription>
            </CardHeader>
            <CardContent>
              <div className="text-center py-8">
                <Upload className="h-12 w-12 mx-auto text-muted-foreground mb-4" />
                <p className="text-muted-foreground mb-4">{t('plugin.uploadCustomPlugin')}</p>
                <Button variant="outline" disabled>
                  <Upload className="h-4 w-4 mr-2" />
                  {t('plugin.uploadPlugin')}
                </Button>
                <p className="text-xs text-muted-foreground mt-4">
                  {t('plugin.customPluginNote')}
                </p>
              </div>
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>

      {/* Plugin Detail Dialog / 插件详情对话框 */}
      {selectedPlugin && (
        <PluginDetailDialog
          open={isDetailOpen}
          onOpenChange={setIsDetailOpen}
          plugin={selectedPlugin}
        />
      )}

      {/* Install Plugin Dialog / 安装插件对话框 */}
      {pluginToInstall && (
        <InstallPluginDialog
          open={isInstallOpen}
          onOpenChange={setIsInstallOpen}
          plugin={pluginToInstall}
          version={selectedVersion}
        />
      )}
    </motion.div>
  );
}
