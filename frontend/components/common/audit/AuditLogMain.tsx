'use client';

/**
 * Audit Log Main Component
 * 审计日志主组件
 *
 * This component provides the main interface for audit log management,
 * including listing, searching, and filtering operations.
 * 本组件提供审计日志管理的主界面，包括列表、搜索和过滤操作。
 */

import {useState, useEffect, useCallback} from 'react';
import {useTranslations} from 'next-intl';
import {Button} from '@/components/ui/button';
import {Input} from '@/components/ui/input';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import {Separator} from '@/components/ui/separator';
import {toast} from 'sonner';
import {Search, FileText, RefreshCw} from 'lucide-react';
import {motion} from 'motion/react';
import services from '@/lib/services';
import {AuditLogInfo, ListAuditLogsRequest} from '@/lib/services/audit/types';
import {AuditLogTable} from './AuditLogTable';

const PAGE_SIZE = 10;

/**
 * Audit Log Main Component
 * 审计日志主组件
 */
export function AuditLogMain() {
  const t = useTranslations();

  // Data state / 数据状态
  const [logs, setLogs] = useState<AuditLogInfo[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [currentPage, setCurrentPage] = useState(1);

  // Filter state / 过滤状态
  const [searchUsername, setSearchUsername] = useState('');
  const [filterAction, setFilterAction] = useState<string>('all');
  const [filterResourceType, setFilterResourceType] = useState<string>('all');


  /**
   * Load audit logs list
   * 加载审计日志列表
   */
  const loadLogs = useCallback(async () => {
    setLoading(true);
    try {
      const params: ListAuditLogsRequest = {
        current: currentPage,
        size: PAGE_SIZE,
        username: searchUsername || undefined,
        action: filterAction !== 'all' ? filterAction : undefined,
        resource_type:
          filterResourceType !== 'all' ? filterResourceType : undefined,
      };

      const result = await services.audit.getAuditLogsSafe(params);

      if (result.success && result.data) {
        setLogs(result.data.logs || []);
        setTotal(result.data.total || 0);
      } else {
        toast.error(result.error || t('audit.loadAuditLogsError'));
        setLogs([]);
        setTotal(0);
      }
    } catch (error) {
      toast.error(
        error instanceof Error ? error.message : t('audit.loadAuditLogsError'),
      );
      setLogs([]);
      setTotal(0);
    } finally {
      setLoading(false);
    }
  }, [currentPage, searchUsername, filterAction, filterResourceType, t]);

  useEffect(() => {
    loadLogs();
  }, [loadLogs]);

  /**
   * Handle search
   * 处理搜索
   */
  const handleSearch = () => {
    setCurrentPage(1);
    loadLogs();
  };

  /**
   * Handle refresh
   * 处理刷新
   */
  const handleRefresh = () => {
    loadLogs();
  };

  /**
   * Handle page change
   * 处理页面变化
   */
  const handlePageChange = (page: number) => {
    setCurrentPage(page);
  };

  /**
   * Clear all filters
   * 清除所有过滤条件
   */
  const handleClearFilters = () => {
    setSearchUsername('');
    setFilterAction('all');
    setFilterResourceType('all');
    setCurrentPage(1);
  };

  const totalPages = Math.ceil(total / PAGE_SIZE);

  const containerVariants = {
    hidden: {opacity: 0},
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
    hidden: {opacity: 0, y: 20},
    visible: {
      opacity: 1,
      y: 0,
      transition: {duration: 0.6, ease: 'easeOut'},
    },
  };

  return (
    <motion.div
      className='space-y-6'
      initial='hidden'
      animate='visible'
      variants={containerVariants}
    >
      {/* Header / 标题 */}
      <motion.div
        className='flex items-center justify-between'
        variants={itemVariants}
      >
        <div className='flex items-center gap-2'>
          <FileText className='h-6 w-6' />
          <div>
            <h1 className='text-2xl font-bold tracking-tight'>
              {t('audit.auditLogsTitle')}
            </h1>
            <p className='text-muted-foreground mt-1'>
              {t('audit.auditLogsDescription')}
            </p>
          </div>
        </div>
        <Button variant='outline' onClick={handleRefresh}>
          <RefreshCw className='h-4 w-4 mr-2' />
          {t('common.refresh')}
        </Button>
      </motion.div>

      <Separator />

      {/* Filters / 过滤器 */}
      <motion.div
        className='flex flex-wrap gap-4 items-end'
        variants={itemVariants}
      >
        <div className='flex-1 min-w-[200px] max-w-sm'>
          <Input
            placeholder={t('audit.searchUsernamePlaceholder')}
            value={searchUsername}
            onChange={(e) => setSearchUsername(e.target.value)}
            onKeyDown={(e) => e.key === 'Enter' && handleSearch()}
          />
        </div>

        <Select value={filterAction} onValueChange={setFilterAction}>
          <SelectTrigger className='w-[150px]'>
            <SelectValue placeholder={t('audit.action')} />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value='all'>{t('audit.allActions')}</SelectItem>
            <SelectItem value='create'>{t('audit.actions.create')}</SelectItem>
            <SelectItem value='update'>{t('audit.actions.update')}</SelectItem>
            <SelectItem value='delete'>{t('audit.actions.delete')}</SelectItem>
            <SelectItem value='start'>{t('audit.actions.start')}</SelectItem>
            <SelectItem value='stop'>{t('audit.actions.stop')}</SelectItem>
            <SelectItem value='restart'>{t('audit.actions.restart')}</SelectItem>
          </SelectContent>
        </Select>

        <Select
          value={filterResourceType}
          onValueChange={setFilterResourceType}
        >
          <SelectTrigger className='w-[150px]'>
            <SelectValue placeholder={t('audit.resourceType')} />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value='all'>{t('audit.allResourceTypes')}</SelectItem>
            <SelectItem value='host'>{t('audit.resourceTypes.host')}</SelectItem>
            <SelectItem value='cluster'>
              {t('audit.resourceTypes.cluster')}
            </SelectItem>
            <SelectItem value='user'>{t('audit.resourceTypes.user')}</SelectItem>
            <SelectItem value='project'>
              {t('audit.resourceTypes.project')}
            </SelectItem>
          </SelectContent>
        </Select>

        <Button variant='outline' onClick={handleSearch}>
          <Search className='h-4 w-4 mr-2' />
          {t('common.search')}
        </Button>

        <Button variant='ghost' onClick={handleClearFilters}>
          {t('common.clearFilters')}
        </Button>
      </motion.div>

      {/* Audit Log Table / 审计日志表格 */}
      <motion.div variants={itemVariants}>
        <AuditLogTable
          logs={logs}
          loading={loading}
          currentPage={currentPage}
          totalPages={totalPages}
          total={total}
          onPageChange={handlePageChange}
        />
      </motion.div>
    </motion.div>
  );
}
