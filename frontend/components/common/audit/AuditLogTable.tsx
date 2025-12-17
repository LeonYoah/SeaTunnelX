'use client';

/**
 * Audit Log Table Component
 * 审计日志表格组件
 *
 * Displays a table of audit logs with filtering and pagination.
 * 显示审计日志表格，支持过滤和分页。
 */

import {useTranslations} from 'next-intl';
import {Button} from '@/components/ui/button';
import {Badge} from '@/components/ui/badge';
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
import {User, Globe} from 'lucide-react';
import {AuditLogInfo} from '@/lib/services/audit/types';

interface AuditLogTableProps {
  logs: AuditLogInfo[];
  loading: boolean;
  currentPage: number;
  totalPages: number;
  total: number;
  onPageChange: (page: number) => void;
}

/**
 * Get action badge variant
 * 获取操作徽章变体
 */
function getActionBadgeVariant(
  action: string,
): 'default' | 'secondary' | 'destructive' | 'outline' {
  switch (action.toLowerCase()) {
    case 'create':
      return 'default';
    case 'update':
      return 'secondary';
    case 'delete':
      return 'destructive';
    case 'start':
    case 'stop':
    case 'restart':
      return 'outline';
    default:
      return 'secondary';
  }
}


/**
 * Audit Log Table Component
 * 审计日志表格组件
 */
export function AuditLogTable({
  logs,
  loading,
  currentPage,
  totalPages,
  total,
  onPageChange,
}: AuditLogTableProps) {
  const t = useTranslations();

  return (
    <div className='space-y-4'>
      <div className='border rounded-lg'>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className='w-[50px]'>ID</TableHead>
              <TableHead>{t('audit.user')}</TableHead>
              <TableHead>{t('audit.action')}</TableHead>
              <TableHead>{t('audit.resourceType')}</TableHead>
              <TableHead>{t('audit.resourceName')}</TableHead>
              <TableHead>{t('audit.ipAddress')}</TableHead>
              <TableHead>{t('audit.createdAt')}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {loading ? (
              <TableRow>
                <TableCell colSpan={7} className='text-center py-8'>
                  {t('common.loading')}
                </TableCell>
              </TableRow>
            ) : logs.length === 0 ? (
              <TableRow>
                <TableCell
                  colSpan={7}
                  className='text-center py-8 text-muted-foreground'
                >
                  {t('audit.noAuditLogs')}
                </TableCell>
              </TableRow>
            ) : (
              logs.map((log) => (
                <TableRow key={log.id}>
                  <TableCell>{log.id}</TableCell>
                  <TableCell>
                    <div className='flex items-center gap-2'>
                      <User className='h-4 w-4 text-muted-foreground' />
                      <span>{log.username || '-'}</span>
                    </div>
                  </TableCell>
                  <TableCell>
                    <Badge variant={getActionBadgeVariant(log.action)}>
                      {log.action}
                    </Badge>
                  </TableCell>
                  <TableCell>
                    <Badge variant='outline'>{log.resource_type}</Badge>
                  </TableCell>
                  <TableCell>
                    <TooltipProvider>
                      <Tooltip>
                        <TooltipTrigger asChild>
                          <span className='truncate max-w-[150px] block cursor-default'>
                            {log.resource_name || log.resource_id || '-'}
                          </span>
                        </TooltipTrigger>
                        <TooltipContent>
                          <p>ID: {log.resource_id || '-'}</p>
                          <p>Name: {log.resource_name || '-'}</p>
                        </TooltipContent>
                      </Tooltip>
                    </TooltipProvider>
                  </TableCell>
                  <TableCell>
                    <div className='flex items-center gap-2'>
                      <Globe className='h-4 w-4 text-muted-foreground' />
                      <span className='font-mono text-sm'>
                        {log.ip_address || '-'}
                      </span>
                    </div>
                  </TableCell>
                  <TableCell>
                    {new Date(log.created_at).toLocaleString()}
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
