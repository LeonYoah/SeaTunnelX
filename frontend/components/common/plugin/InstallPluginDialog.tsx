/**
 * Install Plugin Dialog Component
 * 安装插件对话框组件
 * 
 * Allows users to select a target cluster and install a plugin
 * 允许用户选择目标集群并安装插件
 */

'use client';

import { useState, useEffect, useCallback } from 'react';
import { useTranslations } from 'next-intl';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Label } from '@/components/ui/label';
import { Card, CardContent } from '@/components/ui/card';
import { Progress } from '@/components/ui/progress';
import { Loader2, Download, Server, AlertCircle, CheckCircle, Info, CloudDownload } from 'lucide-react';
import { toast } from 'sonner';
import type { Plugin, PluginDownloadProgress } from '@/lib/services/plugin';
import { PluginService } from '@/lib/services/plugin';
import { ClusterService } from '@/lib/services/cluster';
import type { ClusterInfo } from '@/lib/services/cluster';

interface InstallPluginDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  plugin: Plugin;
  version: string;
}

/**
 * Install Plugin Dialog Component
 * 安装插件对话框组件
 */
export function InstallPluginDialog({
  open,
  onOpenChange,
  plugin,
  version,
}: InstallPluginDialogProps) {
  const t = useTranslations();

  // State / 状态
  const [clusters, setClusters] = useState<ClusterInfo[]>([]);
  const [selectedClusterId, setSelectedClusterId] = useState<string>('');
  const [loadingClusters, setLoadingClusters] = useState(true);
  const [installing, setInstalling] = useState(false);
  const [downloading, setDownloading] = useState(false);
  const [downloadProgress, setDownloadProgress] = useState<PluginDownloadProgress | null>(null);
  const [isDownloaded, setIsDownloaded] = useState(false);
  const [error, setError] = useState<string | null>(null);

  /**
   * Check download status
   * 检查下载状态
   */
  const checkDownloadStatus = useCallback(async () => {
    try {
      const status = await PluginService.getDownloadStatus(plugin.name, version);
      setDownloadProgress(status);
      if (status.status === 'completed') {
        setIsDownloaded(true);
        setDownloading(false);
      } else if (status.status === 'downloading') {
        setDownloading(true);
      } else if (status.status === 'failed') {
        setDownloading(false);
        setError(status.error || t('plugin.downloadFailed'));
      }
    } catch {
      // If status check fails, assume not downloaded / 如果状态检查失败，假设未下载
      setIsDownloaded(false);
    }
  }, [plugin.name, version, t]);

  /**
   * Load available clusters and check download status
   * 加载可用集群列表并检查下载状态
   */
  useEffect(() => {
    if (open) {
      loadClusters();
      checkDownloadStatus();
    }
  }, [open, checkDownloadStatus]);

  /**
   * Poll download status while downloading
   * 下载时轮询下载状态
   */
  useEffect(() => {
    if (!downloading) return;

    const interval = setInterval(checkDownloadStatus, 1000);
    return () => clearInterval(interval);
  }, [downloading, checkDownloadStatus]);

  const loadClusters = async () => {
    setLoadingClusters(true);
    setError(null);
    try {
      const result = await ClusterService.getClusters({ current: 1, size: 100 });
      // Filter clusters that are online/running / 过滤在线/运行中的集群
      const availableClusters = result.clusters.filter(
        (c: ClusterInfo) => c.status === 'running' || c.status === 'stopped'
      );
      setClusters(availableClusters);
      
      // Auto-select if only one cluster / 如果只有一个集群则自动选择
      if (availableClusters.length === 1) {
        setSelectedClusterId(String(availableClusters[0].id));
      }
    } catch (err) {
      const errorMsg = err instanceof Error ? err.message : t('cluster.loadError');
      setError(errorMsg);
      setClusters([]);
    } finally {
      setLoadingClusters(false);
    }
  };

  /**
   * Handle download plugin
   * 处理下载插件
   */
  const handleDownload = async () => {
    setDownloading(true);
    setError(null);
    try {
      await PluginService.downloadPlugin(plugin.name, version);
      toast.info(t('plugin.downloadStarted', { name: plugin.display_name || plugin.name }));
    } catch (err) {
      const errorMsg = err instanceof Error ? err.message : t('plugin.downloadFailed');
      setError(errorMsg);
      toast.error(errorMsg);
      setDownloading(false);
    }
  };

  /**
   * Handle install plugin
   * 处理安装插件
   */
  const handleInstall = async () => {
    if (!selectedClusterId) {
      toast.error(t('plugin.selectClusterFirst'));
      return;
    }

    // Check if plugin is downloaded first / 首先检查插件是否已下载
    if (!isDownloaded) {
      toast.error(t('plugin.downloadFirst'));
      return;
    }

    setInstalling(true);
    setError(null);
    try {
      await PluginService.installPlugin(
        Number(selectedClusterId), 
        plugin.name,
        version
      );
      toast.success(t('plugin.installSuccess', { name: plugin.display_name || plugin.name }));
      onOpenChange(false);
    } catch (err) {
      const errorMsg = err instanceof Error ? err.message : t('plugin.installFailed');
      setError(errorMsg);
      toast.error(errorMsg);
    } finally {
      setInstalling(false);
    }
  };

  /**
   * Handle dialog close
   * 处理对话框关闭
   */
  const handleClose = () => {
    if (!installing && !downloading) {
      setSelectedClusterId('');
      setError(null);
      setDownloadProgress(null);
      onOpenChange(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={handleClose}>
      <DialogContent className="sm:max-w-[500px]">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <Download className="h-5 w-5" />
            {t('plugin.installPlugin')}
          </DialogTitle>
          <DialogDescription>
            {t('plugin.installPluginDesc')}
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4 py-4">
          {/* Plugin info / 插件信息 */}
          <Card className="bg-muted/50">
            <CardContent className="pt-4 pb-4 space-y-2">
              <div className="flex items-center justify-between">
                <span className="font-medium">{plugin.display_name || plugin.name}</span>
                <Badge variant="secondary">v{version}</Badge>
              </div>
              <p className="text-sm text-muted-foreground">{plugin.name}</p>
              {plugin.dependencies && plugin.dependencies.length > 0 && (
                <p className="text-xs text-muted-foreground">
                  {t('plugin.dependencies')}: {plugin.dependencies.length}
                </p>
              )}
            </CardContent>
          </Card>

          {/* Cluster selector / 集群选择器 */}
          <div className="space-y-2">
            <Label htmlFor="cluster-select">{t('plugin.selectTargetCluster')}</Label>
            {loadingClusters ? (
              <div className="flex items-center gap-2 text-sm text-muted-foreground">
                <Loader2 className="h-4 w-4 animate-spin" />
                {t('common.loading')}
              </div>
            ) : clusters.length === 0 ? (
              <Card className="border-amber-200 bg-amber-50 dark:border-amber-800 dark:bg-amber-950">
                <CardContent className="pt-4 pb-4">
                  <div className="flex items-center gap-2 text-amber-800 dark:text-amber-200">
                    <AlertCircle className="h-4 w-4" />
                    <span className="text-sm">{t('plugin.noClustersAvailable')}</span>
                  </div>
                </CardContent>
              </Card>
            ) : (
              <Select
                value={selectedClusterId}
                onValueChange={setSelectedClusterId}
                disabled={installing}
              >
                <SelectTrigger id="cluster-select">
                  <SelectValue placeholder={t('plugin.selectClusterPlaceholder')} />
                </SelectTrigger>
                <SelectContent>
                  {clusters.map((cluster) => (
                    <SelectItem key={cluster.id} value={String(cluster.id)}>
                      <div className="flex items-center gap-2">
                        <Server className="h-4 w-4" />
                        <span>{cluster.name}</span>
                        <Badge
                          variant={cluster.status === 'running' ? 'default' : 'secondary'}
                          className="ml-2"
                        >
                          {t(`cluster.statuses.${cluster.status}`)}
                        </Badge>
                      </div>
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            )}
          </div>

          {/* Error display / 错误显示 */}
          {error && (
            <Card className="border-destructive bg-destructive/10">
              <CardContent className="pt-4 pb-4">
                <div className="flex items-center gap-2 text-destructive">
                  <AlertCircle className="h-4 w-4" />
                  <span className="text-sm">{error}</span>
                </div>
              </CardContent>
            </Card>
          )}

          {/* Install note / 安装说明 */}
          <Card className="bg-blue-50 dark:bg-blue-950 border-blue-200 dark:border-blue-800">
            <CardContent className="pt-4 pb-4">
              <div className="flex items-start gap-2">
                <Info className="h-4 w-4 text-blue-600 mt-0.5" />
                <p className="text-xs text-blue-800 dark:text-blue-200">
                  {t('plugin.installNote')}
                </p>
              </div>
            </CardContent>
          </Card>
        </div>

        {/* Download progress / 下载进度 */}
        {downloading && downloadProgress && (
          <Card className="bg-blue-50 dark:bg-blue-950 border-blue-200 dark:border-blue-800">
            <CardContent className="pt-4 pb-4 space-y-2">
              <div className="flex items-center justify-between text-sm">
                <span className="text-blue-800 dark:text-blue-200">
                  {downloadProgress.current_step || t('plugin.downloading')}
                </span>
                <span className="text-blue-600">{downloadProgress.progress}%</span>
              </div>
              <Progress value={downloadProgress.progress} className="h-2" />
            </CardContent>
          </Card>
        )}

        <DialogFooter>
          <Button variant="outline" onClick={handleClose} disabled={installing || downloading}>
            {t('common.cancel')}
          </Button>
          {!isDownloaded && !downloading ? (
            <Button onClick={handleDownload}>
              <CloudDownload className="h-4 w-4 mr-2" />
              {t('plugin.download')}
            </Button>
          ) : (
            <Button
              onClick={handleInstall}
              disabled={!selectedClusterId || installing || clusters.length === 0 || !isDownloaded}
            >
              {installing ? (
                <>
                  <Loader2 className="h-4 w-4 mr-2 animate-spin" />
                  {t('plugin.installing')}
                </>
              ) : (
                <>
                  <Download className="h-4 w-4 mr-2" />
                  {t('plugin.install')}
                </>
              )}
            </Button>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
