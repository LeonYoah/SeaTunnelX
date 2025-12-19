'use client';

/**
 * Cluster Plugins Component
 * 集群插件组件
 *
 * Displays installed plugins for a cluster with enable/disable functionality.
 * 显示集群的已安装插件，支持启用/禁用功能。
 */

import {useState, useEffect, useCallback} from 'react';
import {useTranslations} from 'next-intl';
import {Button} from '@/components/ui/button';
import {Badge} from '@/components/ui/badge';
import {Card, CardContent, CardHeader, CardTitle} from '@/components/ui/card';
import {Checkbox} from '@/components/ui/checkbox';
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
import {toast} from 'sonner';
import {
  Puzzle,
  RefreshCw,
  Power,
  PowerOff,
  Trash2,
  Loader2,
  Plus,
  CheckSquare,
} from 'lucide-react';
import {useRouter} from 'next/navigation';
import services from '@/lib/services';
import {InstalledPlugin, PluginStatus} from '@/lib/services/plugin/types';

interface ClusterPluginsProps {
  clusterId: number;
}

/**
 * Get status badge variant
 * 获取状态徽章变体
 */
function getStatusBadgeVariant(
  status: PluginStatus,
): 'default' | 'secondary' | 'destructive' | 'outline' {
  switch (status) {
    case 'installed':
    case 'enabled':
      return 'default';
    case 'disabled':
      return 'secondary';
    default:
      return 'secondary';
  }
}

/**
 * Cluster Plugins Component
 * 集群插件组件
 */
export function ClusterPlugins({clusterId}: ClusterPluginsProps) {
  const t = useTranslations();
  const router = useRouter();

  // Data state / 数据状态
  const [plugins, setPlugins] = useState<InstalledPlugin[]>([]);
  const [loading, setLoading] = useState(true);
  const [operating, setOperating] = useState<string | null>(null);

  // Dialog state / 对话框状态
  const [pluginToUninstall, setPluginToUninstall] = useState<InstalledPlugin | null>(null);

  // Batch selection state / 批量选择状态
  const [selectedPlugins, setSelectedPlugins] = useState<Set<string>>(new Set());
  const [batchOperating, setBatchOperating] = useState(false);

  /**
   * Load installed plugins
   * 加载已安装插件
   */
  const loadPlugins = useCallback(async () => {
    setLoading(true);
    try {
      const data = await services.plugin.listInstalledPlugins(clusterId);
      setPlugins(data || []);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('plugin.loadError'));
    } finally {
      setLoading(false);
    }
  }, [clusterId, t]);

  useEffect(() => {
    loadPlugins();
  }, [loadPlugins]);

  /**
   * Handle enable plugin
   * 处理启用插件
   */
  const handleEnable = async (plugin: InstalledPlugin) => {
    setOperating(plugin.plugin_name);
    try {
      await services.plugin.enablePlugin(clusterId, plugin.plugin_name);
      toast.success(t('plugin.enableSuccess'));
      loadPlugins();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('plugin.enableError'));
    } finally {
      setOperating(null);
    }
  };

  /**
   * Handle disable plugin
   * 处理禁用插件
   */
  const handleDisable = async (plugin: InstalledPlugin) => {
    setOperating(plugin.plugin_name);
    try {
      await services.plugin.disablePlugin(clusterId, plugin.plugin_name);
      toast.success(t('plugin.disableSuccess'));
      loadPlugins();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('plugin.disableError'));
    } finally {
      setOperating(null);
    }
  };

  /**
   * Handle uninstall plugin
   * 处理卸载插件
   */
  const handleUninstall = async () => {
    if (!pluginToUninstall) return;

    setOperating(pluginToUninstall.plugin_name);
    try {
      await services.plugin.uninstallPlugin(clusterId, pluginToUninstall.plugin_name);
      toast.success(t('plugin.uninstallSuccess'));
      loadPlugins();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('plugin.uninstallError'));
    } finally {
      setOperating(null);
      setPluginToUninstall(null);
    }
  };

  /**
   * Navigate to plugins page
   * 导航到插件页面
   */
  const goToPluginsPage = () => {
    router.push('/plugins');
  };

  /**
   * Handle select all plugins
   * 处理全选插件
   */
  const handleSelectAll = (checked: boolean) => {
    if (checked) {
      setSelectedPlugins(new Set(plugins.map(p => p.plugin_name)));
    } else {
      setSelectedPlugins(new Set());
    }
  };

  /**
   * Handle select single plugin
   * 处理选择单个插件
   */
  const handleSelectPlugin = (pluginName: string, checked: boolean) => {
    setSelectedPlugins(prev => {
      const next = new Set(prev);
      if (checked) {
        next.add(pluginName);
      } else {
        next.delete(pluginName);
      }
      return next;
    });
  };

  /**
   * Handle batch enable
   * 处理批量启用
   */
  const handleBatchEnable = async () => {
    if (selectedPlugins.size === 0) return;
    
    setBatchOperating(true);
    let successCount = 0;
    let errorCount = 0;

    for (const pluginName of selectedPlugins) {
      try {
        await services.plugin.enablePlugin(clusterId, pluginName);
        successCount++;
      } catch {
        errorCount++;
      }
    }

    setBatchOperating(false);
    setSelectedPlugins(new Set());
    loadPlugins();

    if (errorCount === 0) {
      toast.success(t('plugin.batchEnableSuccess', { count: successCount }));
    } else {
      toast.warning(t('plugin.batchEnablePartial', { success: successCount, failed: errorCount }));
    }
  };

  /**
   * Handle batch disable
   * 处理批量禁用
   */
  const handleBatchDisable = async () => {
    if (selectedPlugins.size === 0) return;
    
    setBatchOperating(true);
    let successCount = 0;
    let errorCount = 0;

    for (const pluginName of selectedPlugins) {
      try {
        await services.plugin.disablePlugin(clusterId, pluginName);
        successCount++;
      } catch {
        errorCount++;
      }
    }

    setBatchOperating(false);
    setSelectedPlugins(new Set());
    loadPlugins();

    if (errorCount === 0) {
      toast.success(t('plugin.batchDisableSuccess', { count: successCount }));
    } else {
      toast.warning(t('plugin.batchDisablePartial', { success: successCount, failed: errorCount }));
    }
  };

  if (loading) {
    return (
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Puzzle className="h-5 w-5" />
            {t('plugin.installedPlugins')}
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div className="flex items-center justify-center py-8">
            <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
          </div>
        </CardContent>
      </Card>
    );
  }

  return (
    <>
      <Card>
        <CardHeader className="flex flex-row items-center justify-between">
          <CardTitle className="flex items-center gap-2">
            <Puzzle className="h-5 w-5" />
            {t('plugin.installedPlugins')}
            {plugins.length > 0 && (
              <Badge variant="secondary" className="ml-2">
                {plugins.length}
              </Badge>
            )}
          </CardTitle>
          <div className="flex gap-2">
            {/* Batch actions / 批量操作 */}
            {selectedPlugins.size > 0 && (
              <>
                <Badge variant="secondary">
                  {t('plugin.selectedCount', { count: selectedPlugins.size })}
                </Badge>
                <Button
                  variant="outline"
                  size="sm"
                  onClick={handleBatchEnable}
                  disabled={batchOperating}
                >
                  {batchOperating ? (
                    <Loader2 className="h-4 w-4 mr-2 animate-spin" />
                  ) : (
                    <Power className="h-4 w-4 mr-2 text-green-600" />
                  )}
                  {t('plugin.batchEnable')}
                </Button>
                <Button
                  variant="outline"
                  size="sm"
                  onClick={handleBatchDisable}
                  disabled={batchOperating}
                >
                  {batchOperating ? (
                    <Loader2 className="h-4 w-4 mr-2 animate-spin" />
                  ) : (
                    <PowerOff className="h-4 w-4 mr-2 text-orange-600" />
                  )}
                  {t('plugin.batchDisable')}
                </Button>
              </>
            )}
            <Button variant="outline" size="sm" onClick={loadPlugins}>
              <RefreshCw className="h-4 w-4 mr-2" />
              {t('common.refresh')}
            </Button>
            <Button size="sm" onClick={goToPluginsPage}>
              <Plus className="h-4 w-4 mr-2" />
              {t('plugin.addPlugin')}
            </Button>
          </div>
        </CardHeader>
        <CardContent>
          {plugins.length === 0 ? (
            <div className="text-center py-8 text-muted-foreground">
              <Puzzle className="h-12 w-12 mx-auto mb-4 opacity-50" />
              <p>{t('plugin.noInstalledPlugins')}</p>
              <Button variant="outline" className="mt-4" onClick={goToPluginsPage}>
                <Plus className="h-4 w-4 mr-2" />
                {t('plugin.browsePlugins')}
              </Button>
            </div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="w-[50px]">
                    <Checkbox
                      checked={plugins.length > 0 && plugins.every(p => selectedPlugins.has(p.plugin_name))}
                      onCheckedChange={handleSelectAll}
                    />
                  </TableHead>
                  <TableHead>{t('plugin.name')}</TableHead>
                  <TableHead>{t('plugin.category.label')}</TableHead>
                  <TableHead>{t('plugin.version')}</TableHead>
                  <TableHead>{t('plugin.status.label')}</TableHead>
                  <TableHead>{t('plugin.installedAt')}</TableHead>
                  <TableHead>{t('plugin.actions')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {plugins.map((plugin) => {
                  const isOperating = operating === plugin.plugin_name;
                  const isEnabled = plugin.status === 'enabled' || plugin.status === 'installed';

                  return (
                    <TableRow key={plugin.id}>
                      <TableCell>
                        <Checkbox
                          checked={selectedPlugins.has(plugin.plugin_name)}
                          onCheckedChange={(checked) => handleSelectPlugin(plugin.plugin_name, checked as boolean)}
                        />
                      </TableCell>
                      <TableCell className="font-medium">{plugin.plugin_name}</TableCell>
                      <TableCell>
                        <Badge variant="outline">{plugin.category}</Badge>
                      </TableCell>
                      <TableCell>{plugin.version}</TableCell>
                      <TableCell>
                        <Badge variant={getStatusBadgeVariant(plugin.status)}>
                          {t(`plugin.statuses.${plugin.status}`)}
                        </Badge>
                      </TableCell>
                      <TableCell>
                        {new Date(plugin.installed_at).toLocaleString()}
                      </TableCell>
                      <TableCell>
                        <div className="flex items-center gap-1">
                          {isEnabled ? (
                            <Button
                              variant="ghost"
                              size="icon"
                              onClick={() => handleDisable(plugin)}
                              disabled={isOperating}
                              title={t('plugin.disable')}
                            >
                              {isOperating ? (
                                <Loader2 className="h-4 w-4 animate-spin" />
                              ) : (
                                <PowerOff className="h-4 w-4 text-orange-600" />
                              )}
                            </Button>
                          ) : (
                            <Button
                              variant="ghost"
                              size="icon"
                              onClick={() => handleEnable(plugin)}
                              disabled={isOperating}
                              title={t('plugin.enable')}
                            >
                              {isOperating ? (
                                <Loader2 className="h-4 w-4 animate-spin" />
                              ) : (
                                <Power className="h-4 w-4 text-green-600" />
                              )}
                            </Button>
                          )}
                          <Button
                            variant="ghost"
                            size="icon"
                            onClick={() => setPluginToUninstall(plugin)}
                            disabled={isOperating}
                            title={t('plugin.uninstall')}
                          >
                            <Trash2 className="h-4 w-4 text-destructive" />
                          </Button>
                        </div>
                      </TableCell>
                    </TableRow>
                  );
                })}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>

      {/* Uninstall Confirmation Dialog / 卸载确认对话框 */}
      <AlertDialog open={!!pluginToUninstall} onOpenChange={() => setPluginToUninstall(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('plugin.uninstallPlugin')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t('plugin.uninstallConfirm', {name: pluginToUninstall?.plugin_name})}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('common.cancel')}</AlertDialogCancel>
            <AlertDialogAction onClick={handleUninstall}>
              {t('common.delete')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}
