/**
 * Step Status Badge Component
 * 步骤状态徽章组件
 *
 * Displays the status of an installation step with appropriate styling.
 * 显示安装步骤的状态，带有适当的样式。
 */

'use client';

import { cn } from '@/lib/utils';
import {
  CheckCircle2,
  Circle,
  Loader2,
  XCircle,
  SkipForward,
} from 'lucide-react';
import type { StepStatus } from '@/lib/services/installer/types';

interface StepStatusBadgeProps {
  /** Step status / 步骤状态 */
  status: StepStatus;
  /** Show text label / 显示文本标签 */
  showLabel?: boolean;
  /** Custom class name / 自定义类名 */
  className?: string;
  /** Size variant / 尺寸变体 */
  size?: 'sm' | 'md' | 'lg';
}

/**
 * Status configuration for each step status
 * 每个步骤状态的配置
 */
const statusConfig: Record<
  StepStatus,
  {
    icon: React.ComponentType<{ className?: string }>;
    label: string;
    labelZh: string;
    bgColor: string;
    textColor: string;
    iconColor: string;
  }
> = {
  pending: {
    icon: Circle,
    label: 'Pending',
    labelZh: '等待中',
    bgColor: 'bg-muted',
    textColor: 'text-muted-foreground',
    iconColor: 'text-muted-foreground',
  },
  running: {
    icon: Loader2,
    label: 'Running',
    labelZh: '运行中',
    bgColor: 'bg-blue-100 dark:bg-blue-900/30',
    textColor: 'text-blue-700 dark:text-blue-300',
    iconColor: 'text-blue-500 animate-spin',
  },
  success: {
    icon: CheckCircle2,
    label: 'Success',
    labelZh: '成功',
    bgColor: 'bg-green-100 dark:bg-green-900/30',
    textColor: 'text-green-700 dark:text-green-300',
    iconColor: 'text-green-500',
  },
  failed: {
    icon: XCircle,
    label: 'Failed',
    labelZh: '失败',
    bgColor: 'bg-red-100 dark:bg-red-900/30',
    textColor: 'text-red-700 dark:text-red-300',
    iconColor: 'text-red-500',
  },
  skipped: {
    icon: SkipForward,
    label: 'Skipped',
    labelZh: '已跳过',
    bgColor: 'bg-gray-100 dark:bg-gray-800',
    textColor: 'text-gray-600 dark:text-gray-400',
    iconColor: 'text-gray-400',
  },
};

/**
 * Size configuration
 * 尺寸配置
 */
const sizeConfig = {
  sm: {
    badge: 'px-1.5 py-0.5 text-xs',
    icon: 'h-3 w-3',
    gap: 'gap-1',
  },
  md: {
    badge: 'px-2 py-1 text-sm',
    icon: 'h-4 w-4',
    gap: 'gap-1.5',
  },
  lg: {
    badge: 'px-3 py-1.5 text-base',
    icon: 'h-5 w-5',
    gap: 'gap-2',
  },
};

export function StepStatusBadge({
  status,
  showLabel = true,
  className,
  size = 'md',
}: StepStatusBadgeProps) {
  const config = statusConfig[status];
  const sizeStyles = sizeConfig[size];
  const Icon = config.icon;

  return (
    <span
      className={cn(
        'inline-flex items-center rounded-full font-medium',
        sizeStyles.badge,
        sizeStyles.gap,
        config.bgColor,
        config.textColor,
        className
      )}
    >
      <Icon className={cn(sizeStyles.icon, config.iconColor)} />
      {showLabel && <span>{config.label}</span>}
    </span>
  );
}

/**
 * Icon-only status indicator
 * 仅图标的状态指示器
 */
export function StepStatusIcon({
  status,
  className,
  size = 'md',
}: Omit<StepStatusBadgeProps, 'showLabel'>) {
  const config = statusConfig[status];
  const sizeStyles = sizeConfig[size];
  const Icon = config.icon;

  return <Icon className={cn(sizeStyles.icon, config.iconColor, className)} />;
}

export default StepStatusBadge;
