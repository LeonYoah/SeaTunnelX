/**
 * Installation Progress Component
 * 安装进度组件
 *
 * Displays the current installation progress with step details.
 * Can be embedded in host detail page or used standalone.
 * 显示当前安装进度和步骤详情。
 * 可嵌入主机详情页或独立使用。
 */

'use client';

import { useEffect } from 'react';
import { useInstallation } from '@/hooks/use-installer';
import { StepStatusBadge, StepStatusIcon } from './StepStatusBadge';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Progress } from '@/components/ui/progress';
import { Separator } from '@/components/ui/separator';
import { ScrollArea } from '@/components/ui/scroll-area';
import {
  RefreshCw,
  XCircle,
  RotateCcw,
  CheckCircle2,
  AlertCircle,
  Clock,
  Loader2,
} from 'lucide-react';
import { cn } from '@/lib/utils';
import { useTranslations } from 'next-intl';
import type { InstallationStatus, StepInfo, StepStatus } from '@/lib/services/installer/types';

interface InstallationProgressProps {
  /** Host ID to track installation / 要跟踪安装的主机 ID */
  hostId: number | string;
  /** Poll interval in ms (default: 2000) / 轮询间隔毫秒数（默认：2000） */
  pollInterval?: number;
  /** Compact mode for embedding / 嵌入式紧凑模式 */
  compact?: boolean;
  /** Callback when installation completes / 安装完成时的回调 */
  onComplete?: (status: InstallationStatus) => void;
  /** Callback when installation fails / 安装失败时的回调 */
  onFailed?: (status: InstallationStatus) => void;
  /** Custom class name / 自定义类名 */
  className?: string;
}

/**
 * Step name translations
 * 步骤名称翻译
 */
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

/**
 * Format duration from start time
 * 从开始时间格式化持续时间
 */
function formatDuration(startTime?: string, endTime?: string): string {
  if (!startTime) return '-';
  
  const start = new Date(startTime).getTime();
  const end = endTime ? new Date(endTime).getTime() : Date.now();
  const duration = Math.floor((end - start) / 1000);
  
  if (duration < 60) return `${duration}s`;
  if (duration < 3600) return `${Math.floor(duration / 60)}m ${duration % 60}s`;
  return `${Math.floor(duration / 3600)}h ${Math.floor((duration % 3600) / 60)}m`;
}

/**
 * Get overall status icon
 * 获取整体状态图标
 */
function getOverallStatusIcon(status: StepStatus) {
  switch (status) {
    case 'success':
      return <CheckCircle2 className="h-6 w-6 text-green-500" />;
    case 'failed':
      return <XCircle className="h-6 w-6 text-red-500" />;
    case 'running':
      return <Loader2 className="h-6 w-6 text-blue-500 animate-spin" />;
    default:
      return <Clock className="h-6 w-6 text-muted-foreground" />;
  }
}

export function InstallationProgress({
  hostId,
  pollInterval = 2000,
  compact = false,
  onComplete,
  onFailed,
  className,
}: InstallationProgressProps) {
  const t = useTranslations();
  const {
    status,
    loading,
    error,
    retryStep,
    cancelInstallation,
    refresh,
  } = useInstallation(hostId, pollInterval);

  // Handle completion/failure callbacks
  // 处理完成/失败回调
  useEffect(() => {
    if (status?.status === 'success' && onComplete) {
      onComplete(status);
    } else if (status?.status === 'failed' && onFailed) {
      onFailed(status);
    }
  }, [status?.status, status, onComplete, onFailed]);

  // No installation in progress
  // 没有正在进行的安装
  if (!status && !loading) {
    return null;
  }

  // Loading state
  // 加载状态
  if (loading && !status) {
    return (
      <Card className={cn('animate-pulse', className)}>
        <CardContent className="py-6">
          <div className="flex items-center justify-center gap-2 text-muted-foreground">
            <Loader2 className="h-4 w-4 animate-spin" />
            <span>{t('common.loading')}</span>
          </div>
        </CardContent>
      </Card>
    );
  }

  if (!status) return null;

  const currentStepInfo = status.steps.find(s => s.step === status.current_step);
  const failedStep = status.steps.find(s => s.status === 'failed');

  // Compact mode for embedding
  // 嵌入式紧凑模式
  if (compact) {
    return (
      <div className={cn('space-y-3', className)}>
        {/* Progress bar / 进度条 */}
        <div className="space-y-1">
          <div className="flex items-center justify-between text-sm">
            <span className="text-muted-foreground">
              {stepNames[status.current_step]?.en || status.current_step}
            </span>
            <span className="font-medium">{status.progress}%</span>
          </div>
          <Progress value={status.progress} className="h-2" />
        </div>

        {/* Status and actions / 状态和操作 */}
        <div className="flex items-center justify-between">
          <StepStatusBadge status={status.status} size="sm" />
          {status.status === 'failed' && failedStep?.retryable && (
            <Button
              variant="outline"
              size="sm"
              onClick={() => retryStep(failedStep.step)}
              disabled={loading}
            >
              <RotateCcw className="h-3 w-3 mr-1" />
              {t('common.retry')}
            </Button>
          )}
        </div>

        {/* Error message / 错误信息 */}
        {status.error && (
          <p className="text-xs text-destructive">{status.error}</p>
        )}
      </div>
    );
  }

  // Full mode
  // 完整模式
  return (
    <Card className={className}>
      <CardHeader className="pb-3">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-3">
            {getOverallStatusIcon(status.status)}
            <div>
              <CardTitle className="text-lg">
                {t('installer.installationProgress')}
              </CardTitle>
              <CardDescription>
                {status.message || t('installer.installingSeaTunnel')}
              </CardDescription>
            </div>
          </div>
          <div className="flex items-center gap-2">
            <Button
              variant="ghost"
              size="icon"
              onClick={() => refresh()}
              disabled={loading}
            >
              <RefreshCw className={cn('h-4 w-4', loading && 'animate-spin')} />
            </Button>
            {status.status === 'running' && (
              <Button
                variant="outline"
                size="sm"
                onClick={() => cancelInstallation()}
                disabled={loading}
              >
                <XCircle className="h-4 w-4 mr-1" />
                {t('common.cancel')}
              </Button>
            )}
          </div>
        </div>
      </CardHeader>

      <CardContent className="space-y-4">
        {/* Overall progress / 整体进度 */}
        <div className="space-y-2">
          <div className="flex items-center justify-between text-sm">
            <span className="text-muted-foreground">
              {t('installer.overallProgress')}
            </span>
            <span className="font-medium">{status.progress}%</span>
          </div>
          <Progress value={status.progress} className="h-3" />
        </div>

        {/* Duration / 持续时间 */}
        <div className="flex items-center gap-4 text-sm text-muted-foreground">
          <div className="flex items-center gap-1">
            <Clock className="h-4 w-4" />
            <span>{t('installer.duration')}: {formatDuration(status.start_time, status.end_time ?? undefined)}</span>
          </div>
        </div>

        <Separator />

        {/* Steps list / 步骤列表 */}
        <ScrollArea className="h-[300px] pr-4">
          <div className="space-y-3">
            {status.steps.map((step, index) => (
              <StepItem
                key={step.step}
                step={step}
                index={index}
                isCurrentStep={step.step === status.current_step}
                onRetry={step.retryable && step.status === 'failed' ? () => retryStep(step.step) : undefined}
                loading={loading}
              />
            ))}
          </div>
        </ScrollArea>

        {/* Error display / 错误显示 */}
        {(error || status.error) && (
          <>
            <Separator />
            <div className="flex items-start gap-2 p-3 rounded-md bg-destructive/10 text-destructive">
              <AlertCircle className="h-5 w-5 mt-0.5 flex-shrink-0" />
              <div className="text-sm">
                <p className="font-medium">{t('installer.installationFailed')}</p>
                <p className="mt-1">{error || status.error}</p>
              </div>
            </div>
          </>
        )}
      </CardContent>
    </Card>
  );
}


/**
 * Individual step item component
 * 单个步骤项组件
 */
interface StepItemProps {
  step: StepInfo;
  index: number;
  isCurrentStep: boolean;
  onRetry?: () => void;
  loading?: boolean;
}

function StepItem({ step, index, isCurrentStep, onRetry, loading }: StepItemProps) {
  const t = useTranslations();
  const stepName = stepNames[step.step] || { en: step.name, zh: step.name };

  return (
    <div
      className={cn(
        'flex items-start gap-3 p-3 rounded-lg transition-colors',
        isCurrentStep && 'bg-muted/50',
        step.status === 'failed' && 'bg-destructive/5'
      )}
    >
      {/* Step number and status icon / 步骤编号和状态图标 */}
      <div className="flex-shrink-0 mt-0.5">
        <div
          className={cn(
            'w-8 h-8 rounded-full flex items-center justify-center text-sm font-medium',
            step.status === 'success' && 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-300',
            step.status === 'failed' && 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-300',
            step.status === 'running' && 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-300',
            step.status === 'pending' && 'bg-muted text-muted-foreground',
            step.status === 'skipped' && 'bg-gray-100 text-gray-500 dark:bg-gray-800 dark:text-gray-400'
          )}
        >
          {step.status === 'running' ? (
            <Loader2 className="h-4 w-4 animate-spin" />
          ) : step.status === 'success' ? (
            <CheckCircle2 className="h-4 w-4" />
          ) : step.status === 'failed' ? (
            <XCircle className="h-4 w-4" />
          ) : (
            index + 1
          )}
        </div>
      </div>

      {/* Step content / 步骤内容 */}
      <div className="flex-1 min-w-0">
        <div className="flex items-center justify-between gap-2">
          <div>
            <p className="font-medium text-sm">{stepName.en}</p>
            <p className="text-xs text-muted-foreground">{stepName.zh}</p>
          </div>
          <StepStatusBadge status={step.status} size="sm" showLabel={false} />
        </div>

        {/* Step progress (for running step) / 步骤进度（运行中的步骤） */}
        {step.status === 'running' && step.progress > 0 && (
          <div className="mt-2">
            <Progress value={step.progress} className="h-1.5" />
          </div>
        )}

        {/* Step message / 步骤消息 */}
        {step.message && (
          <p className="mt-1 text-xs text-muted-foreground truncate">
            {step.message}
          </p>
        )}

        {/* Error message and retry button / 错误消息和重试按钮 */}
        {step.status === 'failed' && step.error && (
          <div className="mt-2 space-y-2">
            <p className="text-xs text-destructive">{step.error}</p>
            {onRetry && (
              <Button
                variant="outline"
                size="sm"
                onClick={onRetry}
                disabled={loading}
                className="h-7 text-xs"
              >
                <RotateCcw className="h-3 w-3 mr-1" />
                {t('common.retry')}
              </Button>
            )}
          </div>
        )}

        {/* Duration / 持续时间 */}
        {(step.start_time || step.end_time) && (
          <p className="mt-1 text-xs text-muted-foreground">
            {formatDuration(step.start_time, step.end_time)}
          </p>
        )}
      </div>
    </div>
  );
}

export default InstallationProgress;
