/**
 * Complete Step Component
 * 完成步骤组件
 *
 * Displays installation result and provides next actions.
 * 显示安装结果并提供后续操作。
 */

'use client';

import { useTranslations } from 'next-intl';
import { useRouter } from 'next/navigation';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Separator } from '@/components/ui/separator';
import { cn } from '@/lib/utils';
import {
  CheckCircle2,
  XCircle,
  Server,
  Play,
  Package,
  ExternalLink,
  Clock,
  ArrowRight,
  PartyPopper,
  AlertTriangle,
} from 'lucide-react';
import type { InstallationStatus } from '@/lib/services/installer/types';

interface CompleteStepProps {
  /** Installation status / 安装状态 */
  status: InstallationStatus | null;
  /** Host ID / 主机 ID */
  hostId: number | string;
  /** Host name / 主机名称 */
  hostName?: string;
  /** Cluster ID / 集群 ID */
  clusterId?: number | string;
  /** Selected plugins / 已选择的插件 */
  selectedPlugins: string[];
  /** Callback to close wizard / 关闭向导的回调 */
  onClose: () => void;
}

// Format duration / 格式化持续时间
function formatDuration(startTime?: string, endTime?: string): string {
  if (!startTime || !endTime) return '-';

  const start = new Date(startTime).getTime();
  const end = new Date(endTime).getTime();
  const duration = Math.floor((end - start) / 1000);

  if (duration < 60) return `${duration} seconds`;
  if (duration < 3600) return `${Math.floor(duration / 60)} minutes ${duration % 60} seconds`;
  return `${Math.floor(duration / 3600)} hours ${Math.floor((duration % 3600) / 60)} minutes`;
}

export function CompleteStep({
  status,
  hostId,
  hostName,
  clusterId,
  selectedPlugins,
  onClose,
}: CompleteStepProps) {
  const t = useTranslations();
  const router = useRouter();

  const isSuccess = status?.status === 'success';
  const isFailed = status?.status === 'failed';

  // Navigate to host detail / 导航到主机详情
  const handleViewHost = () => {
    onClose();
    router.push(`/hosts?id=${hostId}`);
  };

  // Navigate to cluster detail / 导航到集群详情
  const handleViewCluster = () => {
    if (clusterId) {
      onClose();
      router.push(`/clusters/${clusterId}`);
    }
  };

  // Navigate to plugin marketplace / 导航到插件市场
  const handleGoToPlugins = () => {
    onClose();
    router.push('/plugins');
  };

  // Count successful steps / 统计成功步骤数
  const successfulSteps = status?.steps.filter((s) => s.status === 'success').length || 0;
  const totalSteps = status?.steps.length || 0;

  return (
    <div className="space-y-6">
      {/* Result card / 结果卡片 */}
      <Card className={cn(
        'border-2',
        isSuccess && 'border-green-500/50',
        isFailed && 'border-red-500/50'
      )}>
        <CardContent className="pt-8 pb-6">
          <div className="text-center">
            {/* Status icon / 状态图标 */}
            <div className={cn(
              'mx-auto w-20 h-20 rounded-full flex items-center justify-center mb-4',
              isSuccess && 'bg-green-100 dark:bg-green-900/30',
              isFailed && 'bg-red-100 dark:bg-red-900/30'
            )}>
              {isSuccess ? (
                <PartyPopper className="h-10 w-10 text-green-600 dark:text-green-400" />
              ) : (
                <AlertTriangle className="h-10 w-10 text-red-600 dark:text-red-400" />
              )}
            </div>

            {/* Status text / 状态文本 */}
            <h2 className={cn(
              'text-2xl font-bold mb-2',
              isSuccess && 'text-green-600 dark:text-green-400',
              isFailed && 'text-red-600 dark:text-red-400'
            )}>
              {isSuccess ? t('installer.installationSuccess') : t('installer.installationFailed')}
            </h2>

            <p className="text-muted-foreground">
              {isSuccess
                ? t('installer.installationComplete')
                : status?.error || t('installer.installationFailedDesc') || 'Installation encountered an error'}
            </p>
          </div>
        </CardContent>
      </Card>

      {/* Summary card / 摘要卡片 */}
      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="text-base">{t('installer.installationSummary') || 'Installation Summary'}</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          {/* Host info / 主机信息 */}
          <div className="flex items-center justify-between">
            <span className="text-sm text-muted-foreground">{t('host.name')}</span>
            <span className="font-medium">{hostName || `Host #${hostId}`}</span>
          </div>

          {/* Duration / 持续时间 */}
          <div className="flex items-center justify-between">
            <span className="text-sm text-muted-foreground">{t('installer.duration')}</span>
            <div className="flex items-center gap-1">
              <Clock className="h-4 w-4 text-muted-foreground" />
              <span className="font-medium">
                {formatDuration(status?.start_time, status?.end_time ?? undefined)}
              </span>
            </div>
          </div>

          {/* Steps completed / 完成的步骤 */}
          <div className="flex items-center justify-between">
            <span className="text-sm text-muted-foreground">{t('installer.stepsCompleted') || 'Steps Completed'}</span>
            <Badge variant={isSuccess ? 'default' : 'secondary'}>
              {successfulSteps}/{totalSteps}
            </Badge>
          </div>

          <Separator />

          {/* Installed plugins / 已安装的插件 */}
          {selectedPlugins.length > 0 && (
            <div>
              <h4 className="text-sm font-medium mb-2">
                {t('installer.installedPlugins') || 'Installed Plugins'}
              </h4>
              <div className="flex flex-wrap gap-2">
                {selectedPlugins.map((plugin) => (
                  <Badge key={plugin} variant="outline" className="gap-1">
                    <Package className="h-3 w-3" />
                    {plugin}
                  </Badge>
                ))}
              </div>
            </div>
          )}

          {selectedPlugins.length === 0 && (
            <div className="flex items-center justify-between">
              <span className="text-sm text-muted-foreground">
                {t('installer.installedPlugins') || 'Installed Plugins'}
              </span>
              <span className="text-sm text-muted-foreground">
                {t('installer.noPluginsInstalled') || 'No plugins installed'}
              </span>
            </div>
          )}
        </CardContent>
      </Card>

      {/* Action buttons / 操作按钮 */}
      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="text-base">{t('installer.nextSteps') || 'Next Steps'}</CardTitle>
          <CardDescription>
            {t('installer.nextStepsDesc') || 'What would you like to do next?'}
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
            {/* View host / 查看主机 */}
            <Button
              variant="outline"
              className="h-auto py-4 flex flex-col items-center gap-2"
              onClick={handleViewHost}
            >
              <Server className="h-6 w-6" />
              <span>{t('installer.viewHost')}</span>
            </Button>

            {/* Start cluster / 启动集群 */}
            {clusterId && (
              <Button
                variant="outline"
                className="h-auto py-4 flex flex-col items-center gap-2"
                onClick={handleViewCluster}
                disabled={!isSuccess}
              >
                <Play className="h-6 w-6" />
                <span>{t('installer.startCluster')}</span>
              </Button>
            )}

            {/* Go to plugins / 进入插件市场 */}
            <Button
              variant="outline"
              className="h-auto py-4 flex flex-col items-center gap-2"
              onClick={handleGoToPlugins}
            >
              <Package className="h-6 w-6" />
              <span>{t('installer.goToPlugins') || 'Plugin Marketplace'}</span>
            </Button>
          </div>
        </CardContent>
      </Card>

      {/* Close button / 关闭按钮 */}
      <div className="flex justify-center">
        <Button onClick={onClose} className="gap-2">
          {t('common.close')}
          <ArrowRight className="h-4 w-4" />
        </Button>
      </div>
    </div>
  );
}

export default CompleteStep;
