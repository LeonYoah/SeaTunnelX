'use client';

/**
 * Cluster Plugins Component
 * 集群插件组件
 *
 * Displays installed plugins for a cluster. SeaTunnel plugins are not dynamically
 * loaded, so enable/disable state is not used.
 * 显示集群的已安装插件。SeaTunnel 插件非动态加载，无启用/禁用状态。
 */

import {useState, useEffect, useCallback} from 'react';
import {useTranslations} from 'next-intl';
import {Button} from '@/components/ui/button';
import {Badge} from '@/components/ui/badge';
import {Card, CardContent, CardHeader, CardTitle} from '@/components/ui/card';
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
import {Puzzle, RefreshCw, Trash2, Loader2, Plus} from 'lucide-react';
import {useRouter} from 'next/navigation';
import services from '@/lib/services';
import {InstalledPlugin} from '@/lib/services/plugin/types';

interface ClusterPluginsProps {
  clusterId: number;
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
                  <TableHead>{t('plugin.name')}</TableHead>
                  <TableHead>{t('plugin.category.label')}</TableHead>
                  <TableHead>{t('plugin.version')}</TableHead>
                  <TableHead>{t('plugin.installedAt')}</TableHead>
                  <TableHead>{t('plugin.actions')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {plugins.map((plugin) => {
                  const isOperating = operating === plugin.plugin_name;
                  return (
                    <TableRow key={plugin.id}>
                      <TableCell className="font-medium">{plugin.plugin_name}</TableCell>
                      <TableCell>
                        {plugin.category ? (
                          <Badge variant="outline">
                            {t(`plugin.category.${plugin.category}`)}
                          </Badge>
                        ) : (
                          <span className="text-muted-foreground">—</span>
                        )}
                      </TableCell>
                      <TableCell>{plugin.version}</TableCell>
                      <TableCell>
                        {new Date(plugin.installed_at).toLocaleString()}
                      </TableCell>
                      <TableCell>
                        <div className="flex items-center gap-1">
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
