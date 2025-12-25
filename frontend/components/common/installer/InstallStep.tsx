/**
 * Installation Step Component
 * 安装步骤组件
 *
 * Displays installation progress and allows retry/cancel.
 * 显示安装进度并允许重试/取消。
 */

'use client';

import { useTranslations } from 'next-intl';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Progress } from '@/components/ui/progress';
import { ScrollArea } from '@/components/ui/scroll-area';
import { Separator } from '@/components/ui/separator';
import { cn } from '@/lib/utils';
import {
  CheckCircle2,
  XCircle,
  Loader2,
  Clock,
  RefreshCw,
  RotateCcw,
  AlertCircle,
  Circle,
  SkipForward,
} from 'lucide-react';
import type { InstallationStatus, StepInfo, StepStatus } from '@/lib/services/installer/types';

interface InstallStepProps {
  /** Host ID / 主机 ID */
  hostId: number | string;
  /** Installation status / 安装状态 */
  status: InstallationStatus | null;
  /** Loading state / 加载状态 */
  loading: boolean;
  /** Error message / 错误信息 */
  error: string | null;
  /** Callback to retry a step / 重试步骤的回调 */
  onRetry: (step: string) => Promise<InstallationStatus>;
  /** Callback to cancel installation / 取消安装的回调 */
  onCancel: () => Promise<InstallationStatus>;
  /** Callback to refresh status / 刷新状态的回调 */
  onRefresh: () => Promise<InstallationStatus | null>;
}

// Step name translations / 步骤名称翻译
const stepNames: Record<string, { en: string; zh: string }> = {
  download: { en: 'Download Package', zh: '下载安装包' },
  verify: { en: 'Verify Checksum', zh: '校验文件' },
  extract: { en: 'Extract Package', zh: '解压安装包' },
  configure_cluster: { en: 'Configure Cluster', zh: '配置集群' },
  configure_checkpoint: { en: 'Configure Checkpoint', zh: '配置检查点' },
  configure_jvm: { en: 'Configure JVM', zh: '配置 JVM' },
  install_plugins: { en: 'Install Plugins', zh: '安装插件' },
  register_cluster: { en: 'Register Cluster', zh: '注册集群' },
  complete: { en: 'Complete', zh: '完成' },
};

// Status icons / 状态图标
const statusIcons: Record<StepStatus, React.ComponentType<{ className?: string }>> = {
  pending: Circle,
  running: Loader2,
  success: CheckCircle2,
  failed: XCircle,
  skipped: SkipForward,
};

// Status colors / 状态颜色
const statusColors: Record<StepStatus, string> = {
  pending: 'text-muted-foreground',
  running: 'text-blue-500',
  success: 'text-green-500',
  failed: 'text-red-500',
  skipped: 'text-gray-400',
};

// Format duration / 格式化持续时间
function formatDuration(startTime?: string, endTime?: string): string {
  if (!startTime) return '-';

  const start = new Date(startTime).getTime();
  const end = endTime ? new Date(endTime).getTime() : Date.now();
  const duration = Math.floor((end - start) / 1000);

  if (duration < 60) return `${duration}s`;
  if (duration < 3600) return `${Math.floor(duration / 60)}m ${duration % 60}s`;
  return `${Math.floor(duration / 3600)}h ${Math.floor((duration % 3600) / 60)}m`;
}

export function InstallStep({
  hostId,
  status,
  loading,
  error,
  onRetry,
  onCancel,
  onRefresh,
}: InstallStepProps) {
  const t = useTranslations();

  // Get overall status icon / 获取整体状态图标
  const getOverallStatusIcon = () => {
    if (!status) return <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />;

    switch (status.status) {
      case 'success':
        return <CheckCircle2 className="h-6 w-6 text-green-500" />;
      case 'failed':
        return <XCircle className="h-6 w-6 text-red-500" />;
      case 'running':
        return <Loader2 className="h-6 w-6 animate-spin text-blue-500" />;
      default:
        return <Clock className="h-6 w-6 text-muted-foreground" />;
    }
  };

  // Get overall status text / 获取整体状态文本
  const getOverallStatusText = () => {
    if (!status) return t('common.loading');

    switch (status.status) {
      case 'success':
        return t('installer.installationSuccess');
      case 'failed':
        return t('installer.installationFailed');
      case 'running':
        return t('installer.installing');
      default:
        return t('installer.stepStatuses.pending');
    }
  };

  // Render step item / 渲染步骤项
  const renderStepItem = (step: StepInfo, index: number) => {
    const StatusIcon = statusIcons[step.status];
    const stepName = stepNames[step.step] || { en: step.name, zh: step.name };
    const isCurrentStep = status?.current_step === step.step;

    return (
      <div
        key={step.step}
        className={cn(
          'flex items-start gap-4 p-4 rounded-lg transition-colors',
          isCurrentStep && 'bg-muted/50',
          step.status === 'failed' && 'bg-red-50/50 dark:bg-red-900/10'
        )}
      >
        {/* Step number and icon / 步骤编号和图标 */}
        <div
          className={cn(
            'flex items-center justify-center w-10 h-10 rounded-full border-2 flex-shrink-0',
            step.status === 'success' && 'border-green-500 bg-green-50 dark:bg-green-900/20',
            step.status === 'failed' && 'border-red-500 bg-red-50 dark:bg-red-900/20',
            step.status === 'running' && 'border-blue-500 bg-blue-50 dark:bg-blue-900/20',
            step.status === 'pending' && 'border-muted-foreground/30',
            step.status === 'skipped' && 'border-gray-300 bg-gray-50 dark:bg-gray-800'
          )}
        >
          <StatusIcon
            className={cn(
              'h-5 w-5',
              statusColors[step.status],
              step.status === 'running' && 'animate-spin'
            )}
          />
        </div>

        {/* Step content / 步骤内容 */}
        <div className="flex-1 min-w-0">
          <div className="flex items-center justify-between gap-2">
            <div>
              <h4 className="font-medium">{stepName.en}</h4>
              <p className="text-sm text-muted-foreground">{stepName.zh}</p>
            </div>
            {step.start_time && (
              <span className="text-xs text-muted-foreground">
                {formatDuration(step.start_time, step.end_time)}
              </span>
            )}
          </div>

          {/* Progress bar for running step / 运行中步骤的进度条 */}
          {step.status === 'running' && step.progress > 0 && (
            <div className="mt-2">
              <Progress value={step.progress} className="h-2" />
              <p className="text-xs text-muted-foreground mt-1">{step.progress}%</p>
            </div>
          )}

          {/* Message / 消息 */}
          {step.message && (
            <p className="text-sm text-muted-foreground mt-1">{step.message}</p>
          )}

          {/* Error and retry / 错误和重试 */}
          {step.status === 'failed' && (
            <div className="mt-2 space-y-2">
              {step.error && (
                <p className="text-sm text-red-600 dark:text-red-400">{step.error}</p>
              )}
              {step.retryable && (
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => onRetry(step.step)}
                  disabled={loading}
                  className="gap-1"
                >
                  <RotateCcw className="h-3 w-3" />
                  {t('installer.retryStep')}
                </Button>
              )}
            </div>
          )}
        </div>
      </div>
    );
  };

  return (
    <div className="space-y-4">
      {/* Overall status card / 整体状态卡片 */}
      <Card>
        <CardHeader className="pb-3">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-3">
              {getOverallStatusIcon()}
              <div>
                <CardTitle className="text-lg">{getOverallStatusText()}</CardTitle>
                <CardDescription>
                  {status?.message || t('installer.installationDesc')}
                </CardDescription>
              </div>
            </div>
            <div className="flex items-center gap-2">
              <Button
                variant="ghost"
                size="icon"
                onClick={() => onRefresh()}
                disabled={loading}
              >
                <RefreshCw className={cn('h-4 w-4', loading && 'animate-spin')} />
              </Button>
              {status?.status === 'running' && (
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => onCancel()}
                  disabled={loading}
                  className="gap-1"
                >
                  <XCircle className="h-4 w-4" />
                  {t('installer.cancelInstallation')}
                </Button>
              )}
            </div>
          </div>
        </CardHeader>
        <CardContent>
          {/* Overall progress / 整体进度 */}
          <div className="space-y-2">
            <div className="flex items-center justify-between text-sm">
              <span className="text-muted-foreground">{t('installer.overallProgress')}</span>
              <span className="font-medium">{status?.progress || 0}%</span>
            </div>
            <Progress value={status?.progress || 0} className="h-3" />
          </div>

          {/* Duration / 持续时间 */}
          {status?.start_time && (
            <div className="flex items-center gap-2 mt-3 text-sm text-muted-foreground">
              <Clock className="h-4 w-4" />
              <span>
                {t('installer.duration')}: {formatDuration(status.start_time, status.end_time ?? undefined)}
              </span>
            </div>
          )}
        </CardContent>
      </Card>

      {/* Error display / 错误显示 */}
      {(error || status?.error) && (
        <Card className="border-destructive">
          <CardContent className="pt-6">
            <div className="flex items-start gap-2 text-destructive">
              <AlertCircle className="h-5 w-5 flex-shrink-0 mt-0.5" />
              <div>
                <p className="font-medium">{t('installer.installationFailed')}</p>
                <p className="text-sm mt-1">{error || status?.error}</p>
              </div>
            </div>
          </CardContent>
        </Card>
      )}

      {/* Steps list / 步骤列表 */}
      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="text-base">{t('installer.installProgress')}</CardTitle>
        </CardHeader>
        <CardContent>
          <ScrollArea className="h-[300px] pr-4">
            <div className="space-y-2">
              {status?.steps.map((step, index) => renderStepItem(step, index))}

              {/* Loading state / 加载状态 */}
              {!status && loading && (
                <div className="flex items-center justify-center py-12">
                  <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
                </div>
              )}
            </div>
          </ScrollArea>
        </CardContent>
      </Card>
    </div>
  );
}

export default InstallStep;
