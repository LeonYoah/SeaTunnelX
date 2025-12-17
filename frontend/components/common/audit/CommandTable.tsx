'use client';

/**
 * Command Table Component
 * 命令表格组件
 *
 * Displays a table of command logs with actions for viewing details.
 * 显示命令日志表格，支持查看详情操作。
 */

import {useTranslations} from 'next-intl';
import {Button} from '@/components/ui/button';
import {Badge} from '@/components/ui/badge';
import {Progress} from '@/components/ui/progress';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip';
import {Eye} from 'lucide-react';
import {CommandLogInfo, CommandStatus} from '@/lib/services/audit/types';

interface CommandTableProps {
  commands: CommandLogInfo[];
  loading: boolean;
  currentPage: number;
  totalPages: number;
  total: number;
  onPageChange: (page: number) => void;
  onViewDetail: (command: CommandLogInfo) => void;
}

/**
 * Get status badge variant
 * 获取状态徽章变体
 */
function getStatusBadgeVariant(
  status: CommandStatus,
): 'default' | 'secondary' | 'destructive' | 'outline' {
  switch (status) {
    case CommandStatus.SUCCESS:
      return 'default';
    case CommandStatus.RUNNING:
      return 'secondary';
    case CommandStatus.PENDING:
      return 'outline';
    case CommandStatus.FAILED:
      return 'destructive';
    case CommandStatus.CANCELLED:
      return 'outline';
    default:
      return 'secondary';
  }
}


/**
 * Format duration between two dates
 * 格式化两个日期之间的时长
 */
function formatDuration(
  startedAt: string | null,
  finishedAt: string | null,
): string {
  if (!startedAt) return '-';
  const start = new Date(startedAt);
  const end = finishedAt ? new Date(finishedAt) : new Date();
  const durationMs = end.getTime() - start.getTime();
  const seconds = Math.floor(durationMs / 1000);
  if (seconds < 60) return `${seconds}s`;
  const minutes = Math.floor(seconds / 60);
  const remainingSeconds = seconds % 60;
  if (minutes < 60) return `${minutes}m ${remainingSeconds}s`;
  const hours = Math.floor(minutes / 60);
  const remainingMinutes = minutes % 60;
  return `${hours}h ${remainingMinutes}m`;
}

/**
 * Command Table Component
 * 命令表格组件
 */
export function CommandTable({
  commands,
  loading,
  currentPage,
  totalPages,
  total,
  onPageChange,
  onViewDetail,
}: CommandTableProps) {
  const t = useTranslations();

  return (
    <div className='space-y-4'>
      <div className='border rounded-lg'>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className='w-[50px]'>ID</TableHead>
              <TableHead>{t('audit.commandId')}</TableHead>
              <TableHead>{t('audit.commandType')}</TableHead>
              <TableHead>{t('audit.agentId')}</TableHead>
              <TableHead>{t('audit.status')}</TableHead>
              <TableHead>{t('audit.progress')}</TableHead>
              <TableHead>{t('audit.duration')}</TableHead>
              <TableHead>{t('audit.createdAt')}</TableHead>
              <TableHead className='w-[80px]'>{t('common.view')}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {loading ? (
              <TableRow>
                <TableCell colSpan={9} className='text-center py-8'>
                  {t('common.loading')}
                </TableCell>
              </TableRow>
            ) : commands.length === 0 ? (
              <TableRow>
                <TableCell
                  colSpan={9}
                  className='text-center py-8 text-muted-foreground'
                >
                  {t('audit.noCommands')}
                </TableCell>
              </TableRow>
            ) : (
              commands.map((command) => (
                <TableRow key={command.id}>
                  <TableCell>{command.id}</TableCell>
                  <TableCell>
                    <span className='font-mono text-sm truncate max-w-[150px] block'>
                      {command.command_id}
                    </span>
                  </TableCell>
                  <TableCell>
                    <Badge variant='outline'>{command.command_type}</Badge>
                  </TableCell>
                  <TableCell>
                    <span className='font-mono text-sm truncate max-w-[120px] block'>
                      {command.agent_id || '-'}
                    </span>
                  </TableCell>
                  <TableCell>
                    <Badge variant={getStatusBadgeVariant(command.status)}>
                      {t(`audit.statuses.${command.status}`)}
                    </Badge>
                  </TableCell>
                  <TableCell>
                    <div className='flex items-center gap-2 min-w-[100px]'>
                      <Progress value={command.progress} className='h-2' />
                      <span className='text-xs text-muted-foreground'>
                        {command.progress}%
                      </span>
                    </div>
                  </TableCell>
                  <TableCell>
                    {formatDuration(command.started_at, command.finished_at)}
                  </TableCell>
                  <TableCell>
                    {new Date(command.created_at).toLocaleString()}
                  </TableCell>
                  <TableCell>
                    <TooltipProvider>
                      <Tooltip>
                        <TooltipTrigger asChild>
                          <Button
                            variant='ghost'
                            size='icon'
                            onClick={() => onViewDetail(command)}
                          >
                            <Eye className='h-4 w-4' />
                          </Button>
                        </TooltipTrigger>
                        <TooltipContent>{t('common.view')}</TooltipContent>
                      </Tooltip>
                    </TooltipProvider>
                  </TableCell>
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </div>

      {/* Pagination / 分页 */}
      {totalPages > 1 && (
        <div className='flex items-center justify-between'>
          <div className='text-sm text-muted-foreground'>
            {t('common.totalItems', {total})}
          </div>
          <div className='flex gap-2'>
            <Button
              variant='outline'
              size='sm'
              disabled={currentPage === 1}
              onClick={() => onPageChange(currentPage - 1)}
            >
              {t('common.previous')}
            </Button>
            <span className='flex items-center px-4 text-sm'>
              {currentPage} / {totalPages}
            </span>
            <Button
              variant='outline'
              size='sm'
              disabled={currentPage === totalPages}
              onClick={() => onPageChange(currentPage + 1)}
            >
              {t('common.next')}
            </Button>
          </div>
        </div>
      )}
    </div>
  );
}
