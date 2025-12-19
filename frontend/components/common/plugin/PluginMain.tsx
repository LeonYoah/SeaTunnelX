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
import { Checkbox } from '@/components/ui/checkbox';
import { toast } from 'sonner';
import { RefreshCw, Search, Puzzle, Package, Download, CheckCircle, Info, Upload, HardDrive, Trash2, CheckSquare, Server } from 'lucide-react';
import { motion } from 'motion/react';
import { PluginService } from '@/lib/services/plugin';
import { ClusterService } from '@/lib/services/cluster';
import type { Plugin, MirrorSource, AvailablePluginsResponse, LocalPlugin, PluginDownloadProgress, InstalledPlugin } from '@/lib/services/plugin';
import type { ClusterInfo } from '@/lib/services/cluster';
import { Progress } from '@/components/ui/progress';
import { PluginGrid } from './PluginGrid';
import { PluginDetailDialog } from './PluginDetailDialog';
import { InstallPluginDialog } from './InstallPluginDialog';
import { BatchInstallDialog } from './BatchInstallDialog';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog';

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
  const [activeTab, setActiveTab] = useState<'available' | 'local' | 'custom'>('available');

  // Local plugins state / 本地插件状态
  const [localPlugins, setLocalPlugins] = useState<LocalPlugin[]>([]);
  const [localPluginsLoading, setLocalPluginsLoading] = useState(false);
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);
  const [pluginToDelete, setPluginToDelete] = useState<LocalPlugin | null>(null);

  // Dialog state / 对话框状态
  const [selectedPlugin, setSelectedPlugin] = useState<Plugin | null>(null);
  const [isDetailOpen, setIsDetailOpen] = useState(false);
  const [isInstallOpen, setIsInstallOpen] = useState(false);
  const [pluginToInstall, setPluginToInstall] = useState<Plugin | null>(null);

  // Download state / 下载状态
  const [downloadingPlugins, setDownloadingPlugins] = useState<Set<string>>(new Set());
  const [downloadedPlugins, setDownloadedPlugins] = useState<Set<string>>(new Set());

  // Active downloads state / 活动下载状态
  const [activeDownloads, setActiveDownloads] = useState<PluginDownloadProgress[]>([]);

  // Batch selection state / 批量选择状态
  const [selectedLocalPlugins, setSelectedLocalPlugins] = useState<Set<string>>(new Set());
  const [isBatchInstallOpen, setIsBatchInstallOpen] = useState(false);

  // Plugin installation status per cluster / 每个集群的插件安装状态
  // Map: pluginName -> { clusterId -> InstalledPlugin }
  const [pluginClusterStatus, setPluginClusterStatus] = useState<Map<string, Map<number, InstalledPlugin>>>(new Map());
  const [clusters, setClusters] = useState<ClusterInfo[]>([]);

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

  /**
   * Load local downloaded plugins and their cluster installation status
   * 加载本地已下载插件列表及其集群安装状态
   */
  const loadLocalPlugins = useCallback(async () => {
    setLocalPluginsLoading(true);
    try {
      // Load local plugins, active downloads, and clusters in parallel / 并行加载本地插件、活动下载和集群
      const [localResult, downloadsResult, clustersResult] = await Promise.all([
        PluginService.listLocalPlugins(),
        PluginService.listActiveDownloads(),
        ClusterService.getClusters({ current: 1, size: 100 })
      ]);
      
      setLocalPlugins(localResult || []);
      setActiveDownloads(downloadsResult || []);
      
      // Filter available clusters / 过滤可用集群
      const availableClusters = (clustersResult?.clusters || []).filter(
        (c: ClusterInfo) => c.status === 'running' || c.status === 'stopped'
      );
      setClusters(availableClusters);
      
      // Update downloadingPlugins set / 更新下载中集合
      const downloading = new Set(downloadsResult?.filter(d => d.status === 'downloading').map(d => d.plugin_name) || []);
      setDownloadingPlugins(downloading);
      
      // Load installed plugins for each cluster / 加载每个集群的已安装插件
      const statusMap = new Map<string, Map<number, InstalledPlugin>>();
      await Promise.all(
        availableClusters.map(async (cluster: ClusterInfo) => {
          try {
            const installedPlugins = await PluginService.listInstalledPlugins(cluster.id);
            for (const plugin of installedPlugins) {
              if (!statusMap.has(plugin.plugin_name)) {
                statusMap.set(plugin.plugin_name, new Map());
              }
              statusMap.get(plugin.plugin_name)!.set(cluster.id, plugin);
            }
          } catch {
            // Ignore errors / 忽略错误
          }
        })
      );
      setPluginClusterStatus(statusMap);
    } catch (err) {
      console.error('Failed to load local plugins:', err);
      setLocalPlugins([]);
    } finally {
      setLocalPluginsLoading(false);
    }
  }, []);

  useEffect(() => {
    loadPlugins();
  }, [loadPlugins]);

  // Load local plugins when switching to local tab / 切换到本地插件标签时加载
  useEffect(() => {
    if (activeTab === 'local') {
      loadLocalPlugins();
    }
  }, [activeTab, loadLocalPlugins]);

  // Poll for active downloads when there are downloading plugins / 有下载中的插件时轮询
  useEffect(() => {
    if (activeTab !== 'local' || activeDownloads.filter(d => d.status === 'downloading').length === 0) {
      return;
    }

    const interval = setInterval(() => {
      loadLocalPlugins();
    }, 2000); // Poll every 2 seconds / 每2秒轮询一次

    return () => clearInterval(interval);
  }, [activeTab, activeDownloads, loadLocalPlugins]);

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
    if (activeTab === 'local') {
      loadLocalPlugins();
    } else {
      loadPlugins();
    }
  };

  /**
   * Handle delete local plugin
   * 处理删除本地插件
   */
  const handleDeleteLocalPlugin = async () => {
    if (!pluginToDelete) {return;}
    
    try {
      await PluginService.deleteLocalPlugin(pluginToDelete.name, pluginToDelete.version);
      toast.success(t('plugin.deleteSuccess'));
      loadLocalPlugins();
      // Also remove from downloadedPlugins set / 同时从已下载集合中移除
      setDownloadedPlugins(prev => {
        const next = new Set(prev);
        next.delete(pluginToDelete.name);
        return next;
      });
    } catch (err) {
      const errorMsg = err instanceof Error ? err.message : t('plugin.deleteFailed');
      toast.error(errorMsg);
    } finally {
      setDeleteDialogOpen(false);
      setPluginToDelete(null);
    }
  };

  /**
   * Format file size
   * 格式化文件大小
   */
  const formatFileSize = (bytes: number): string => {
    if (bytes === 0) {return '0 B';}
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
  };

  /**
   * Get filtered local plugins
   * 获取过滤后的本地插件
   */
  const getFilteredLocalPlugins = useCallback(() => {
    return localPlugins.filter(plugin => {
      if (searchKeyword) {
        const keyword = searchKeyword.toLowerCase();
        if (!plugin.name.toLowerCase().includes(keyword)) {
          return false;
        }
      }
      if (filterCategory !== 'all' && plugin.category !== filterCategory) {
        return false;
      }
      if (selectedVersion && plugin.version !== selectedVersion) {
        return false;
      }
      return true;
    });
  }, [localPlugins, searchKeyword, filterCategory, selectedVersion]);

  /**
   * Handle select all local plugins
   * 处理全选本地插件
   */
  const handleSelectAllLocalPlugins = (checked: boolean) => {
    if (checked) {
      const filteredPlugins = getFilteredLocalPlugins();
      const allKeys = new Set(filteredPlugins.map(p => `${p.name}:${p.version}`));
      setSelectedLocalPlugins(allKeys);
    } else {
      setSelectedLocalPlugins(new Set());
    }
  };

  /**
   * Handle select single local plugin
   * 处理选择单个本地插件
   */
  const handleSelectLocalPlugin = (plugin: LocalPlugin, checked: boolean) => {
    const key = `${plugin.name}:${plugin.version}`;
    setSelectedLocalPlugins(prev => {
      const next = new Set(prev);
      if (checked) {
        next.add(key);
      } else {
        next.delete(key);
      }
      return next;
    });
  };

  /**
   * Get selected plugins for batch install
   * 获取批量安装的选中插件
   */
  const getSelectedPluginsForBatchInstall = (): Plugin[] => {
    return localPlugins
      .filter(p => selectedLocalPlugins.has(`${p.name}:${p.version}`))
      .map(p => ({
        name: p.name,
        display_name: p.name,
        category: p.category,
        version: p.version,
        description: '',
        group_id: 'org.apache.seatunnel',
        artifact_id: `connector-${p.name}`,
      }));
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

  /**
   * Handle download plugin
   * 处理下载插件到 Control Plane
   */
  const handleDownloadPlugin = async (plugin: Plugin) => {
    try {
      // Add to downloading set / 添加到下载中集合
      setDownloadingPlugins(prev => new Set(prev).add(plugin.name));
      
      toast.info(t('plugin.downloadStarted', { name: plugin.display_name || plugin.name }));
      
      // Call download API / 调用下载 API
      await PluginService.downloadPlugin(plugin.name, selectedVersion, selectedMirror);
      
      // Move to downloaded set / 移动到已下载集合
      setDownloadingPlugins(prev => {
        const next = new Set(prev);
        next.delete(plugin.name);
        return next;
      });
      setDownloadedPlugins(prev => new Set(prev).add(plugin.name));
      
      toast.success(t('plugin.downloadComplete'));
    } catch (err) {
      // Remove from downloading set / 从下载中集合移除
      setDownloadingPlugins(prev => {
        const next = new Set(prev);
        next.delete(plugin.name);
        return next;
      });
      
      const errorMsg = err instanceof Error ? err.message : t('plugin.downloadFailed');
      toast.error(errorMsg);
    }
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
      <Tabs value={activeTab} onValueChange={(v) => setActiveTab(v as 'available' | 'local' | 'custom')}>
        <TabsList>
          <TabsTrigger value="available" className="flex items-center gap-2">
            <Package className="h-4 w-4" />
            {t('plugin.available')}
          </TabsTrigger>
          <TabsTrigger value="local" className="flex items-center gap-2">
            <HardDrive className="h-4 w-4" />
            {t('plugin.localPlugins')}
            {(localPlugins.length > 0 || activeDownloads.filter(d => d.status === 'downloading').length > 0) && (
              <Badge variant="secondary" className="ml-1">
                {localPlugins.length}
                {activeDownloads.filter(d => d.status === 'downloading').length > 0 && (
                  <span className="ml-1 text-blue-500">+{activeDownloads.filter(d => d.status === 'downloading').length}</span>
                )}
              </Badge>
            )}
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
                onDownload={handleDownloadPlugin}
                downloadingPlugins={downloadingPlugins}
                downloadedPlugins={downloadedPlugins}
              />
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="local" className="mt-4">
          <Card>
            <CardHeader className="flex flex-row items-center justify-between">
              <div>
                <CardTitle className="flex items-center gap-2">
                  {t('plugin.localPlugins')}
                  <Badge variant="secondary">{localPlugins.length}</Badge>
                </CardTitle>
                <CardDescription>
                  {t('plugin.localPluginsDesc')}
                </CardDescription>
              </div>
              {/* Batch actions / 批量操作 */}
              {selectedLocalPlugins.size > 0 && (
                <div className="flex items-center gap-2">
                  <Badge variant="secondary">
                    {t('plugin.selectedCount', { count: selectedLocalPlugins.size })}
                  </Badge>
                  <Button
                    size="sm"
                    onClick={() => setIsBatchInstallOpen(true)}
                  >
                    <CheckSquare className="h-4 w-4 mr-2" />
                    {t('plugin.batchInstall')}
                  </Button>
                </div>
              )}
            </CardHeader>
            <CardContent>
              {localPluginsLoading && localPlugins.length === 0 && activeDownloads.length === 0 ? (
                <div className="text-center py-8 text-muted-foreground">
                  <RefreshCw className="h-8 w-8 mx-auto animate-spin mb-4" />
                  <p>{t('common.loading')}</p>
                </div>
              ) : localPlugins.length === 0 && activeDownloads.length === 0 ? (
                <div className="text-center py-8 text-muted-foreground">
                  <HardDrive className="h-12 w-12 mx-auto mb-4 opacity-50" />
                  <p>{t('plugin.noDownloadedPlugins')}</p>
                  <p className="text-sm mt-2">{t('plugin.downloadFromAvailable')}</p>
                </div>
              ) : (
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead className="w-[50px]">
                        <Checkbox
                          checked={
                            getFilteredLocalPlugins().length > 0 &&
                            getFilteredLocalPlugins().every(p => selectedLocalPlugins.has(`${p.name}:${p.version}`))
                          }
                          onCheckedChange={handleSelectAllLocalPlugins}
                        />
                      </TableHead>
                      <TableHead>{t('plugin.name')}</TableHead>
                      <TableHead>{t('plugin.category.label')}</TableHead>
                      <TableHead>{t('plugin.version')}</TableHead>
                      <TableHead>{t('plugin.installedClusters')}</TableHead>
                      <TableHead>{t('plugin.fileSize')}</TableHead>
                      <TableHead className="text-right">{t('common.actions')}</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {/* Active downloads / 活动下载 */}
                    {activeDownloads.filter(d => d.status === 'downloading').map((download) => (
                      <TableRow key={`downloading-${download.plugin_name}-${download.version}`} className="bg-blue-50 dark:bg-blue-950">
                        <TableCell>
                          <Checkbox disabled />
                        </TableCell>
                        <TableCell className="font-medium">
                          <div className="flex items-center gap-2">
                            <RefreshCw className="h-4 w-4 animate-spin text-blue-500" />
                            {download.plugin_name}
                          </div>
                        </TableCell>
                        <TableCell>
                          <Badge variant="outline">{t('plugin.downloading')}</Badge>
                        </TableCell>
                        <TableCell>v{download.version}</TableCell>
                        <TableCell>
                          <span className="text-muted-foreground text-sm">-</span>
                        </TableCell>
                        <TableCell>
                          <div className="space-y-1">
                            <div className="flex items-center justify-between text-sm">
                              <span>{download.current_step || t('plugin.downloading')}</span>
                              <span>{download.progress}%</span>
                            </div>
                            <Progress value={download.progress} className="h-2" />
                            {download.total_bytes && download.total_bytes > 0 && (
                              <div className="text-xs text-muted-foreground">
                                {formatFileSize(download.downloaded_bytes || 0)} / {formatFileSize(download.total_bytes)}
                                {download.speed && download.speed > 0 && (
                                  <span className="ml-2">({formatFileSize(download.speed)}/s)</span>
                                )}
                              </div>
                            )}
                          </div>
                        </TableCell>
                        <TableCell className="text-right">
                          <Badge variant="secondary">{t('plugin.downloading')}</Badge>
                        </TableCell>
                      </TableRow>
                    ))}
                    {/* Filtered local plugins / 过滤后的本地插件 */}
                    {getFilteredLocalPlugins().map((plugin) => {
                      // Get cluster installation status for this plugin / 获取此插件的集群安装状态
                      const clusterStatusMap = pluginClusterStatus.get(plugin.name);
                      const installedClusters = clusterStatusMap 
                        ? clusters.filter(c => clusterStatusMap.has(c.id))
                        : [];
                      
                      return (
                        <TableRow key={`${plugin.name}-${plugin.version}`}>
                          <TableCell>
                            <Checkbox
                              checked={selectedLocalPlugins.has(`${plugin.name}:${plugin.version}`)}
                              onCheckedChange={(checked) => handleSelectLocalPlugin(plugin, checked as boolean)}
                            />
                          </TableCell>
                          <TableCell className="font-medium">{plugin.name}</TableCell>
                          <TableCell>
                            <Badge variant="outline">
                              {plugin.category === 'source' ? t('plugin.category.source') :
                               plugin.category === 'sink' ? t('plugin.category.sink') :
                               plugin.category === 'transform' ? t('plugin.category.transform') :
                               plugin.category}
                            </Badge>
                          </TableCell>
                          <TableCell>v{plugin.version}</TableCell>
                          <TableCell>
                            {installedClusters.length === 0 ? (
                              <span className="text-muted-foreground text-sm">{t('plugin.notInstalled')}</span>
                            ) : (
                              <div className="flex flex-wrap gap-1">
                                {installedClusters.map(cluster => {
                                  const status = clusterStatusMap?.get(cluster.id);
                                  const isEnabled = status?.status === 'enabled' || status?.status === 'installed';
                                  return (
                                    <Badge 
                                      key={cluster.id} 
                                      variant={isEnabled ? 'default' : 'secondary'}
                                      className="text-xs"
                                    >
                                      {cluster.name}
                                      {!isEnabled && <span className="ml-1 opacity-70">({t('plugin.status.disabled')})</span>}
                                    </Badge>
                                  );
                                })}
                              </div>
                            )}
                          </TableCell>
                          <TableCell>{formatFileSize(plugin.size)}</TableCell>
                          <TableCell className="text-right space-x-1">
                            <Button
                              variant="outline"
                              size="sm"
                              onClick={() => {
                                const pluginForInstall: Plugin = {
                                  name: plugin.name,
                                  display_name: plugin.name,
                                  category: plugin.category,
                                  version: plugin.version,
                                  description: '',
                                  group_id: 'org.apache.seatunnel',
                                  artifact_id: `connector-${plugin.name}`,
                                };
                                setPluginToInstall(pluginForInstall);
                                setIsInstallOpen(true);
                              }}
                            >
                              <Server className="h-4 w-4 mr-1" />
                              {t('plugin.managePlugin')}
                            </Button>
                            <Button
                              variant="ghost"
                              size="sm"
                              className="text-destructive hover:text-destructive"
                              onClick={() => {
                                setPluginToDelete(plugin);
                                setDeleteDialogOpen(true);
                              }}
                            >
                              <Trash2 className="h-4 w-4" />
                            </Button>
                          </TableCell>
                        </TableRow>
                      );
                    })}
                  </TableBody>
                </Table>
              )}
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

      {/* Delete Confirmation Dialog / 删除确认对话框 */}
      <AlertDialog open={deleteDialogOpen} onOpenChange={setDeleteDialogOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('plugin.deleteConfirmTitle')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t('plugin.deleteConfirmDesc', { name: pluginToDelete?.name || '' })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('common.cancel')}</AlertDialogCancel>
            <AlertDialogAction
              onClick={handleDeleteLocalPlugin}
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
            >
              {t('common.delete')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      {/* Batch Install Dialog / 批量安装对话框 */}
      <BatchInstallDialog
        open={isBatchInstallOpen}
        onOpenChange={(open: boolean) => {
          setIsBatchInstallOpen(open);
          if (!open) {
            setSelectedLocalPlugins(new Set());
          }
        }}
        plugins={getSelectedPluginsForBatchInstall()}
        version={selectedVersion}
      />
    </motion.div>
  );
}
