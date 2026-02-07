'use client';

/**
 * Host Management Main Component
 * 主机管理主组件
 *
 * This component provides the main interface for host management,
 * including listing, searching, filtering, and CRUD operations.
 * 本组件提供主机管理的主界面，包括列表、搜索、过滤和 CRUD 操作。
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
import {Plus, Search, Server, RefreshCw} from 'lucide-react';
import {motion} from 'motion/react';
import services from '@/lib/services';
import {
  HostInfo,
  HostType,
  HostStatus,
  ListHostsRequest,
} from '@/lib/services/host/types';
import {HostTable} from './HostTable';
import {HostDetail} from './HostDetail';
import {CreateHostDialog} from './CreateHostDialog';
import {EditHostDialog} from './EditHostDialog';

const PAGE_SIZE = 10;

/**
 * Host Management Main Component
 * 主机管理主组件
 */
export function HostMain() {
  const t = useTranslations();

  // Data state / 数据状态
  const [hosts, setHosts] = useState<HostInfo[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [currentPage, setCurrentPage] = useState(1);


  // Filter state / 过滤状态
  const [searchName, setSearchName] = useState('');
  const [filterHostType, setFilterHostType] = useState<string>('all');
  const [filterStatus, setFilterStatus] = useState<string>('all');

  // Dialog state / 对话框状态
  const [isCreateDialogOpen, setIsCreateDialogOpen] = useState(false);
  const [isEditDialogOpen, setIsEditDialogOpen] = useState(false);
  const [isDetailOpen, setIsDetailOpen] = useState(false);
  const [selectedHost, setSelectedHost] = useState<HostInfo | null>(null);

  /**
   * Load hosts list
   * 加载主机列表
   */
  const loadHosts = useCallback(async () => {
    setLoading(true);
    try {
      const params: ListHostsRequest = {
        current: currentPage,
        size: PAGE_SIZE,
        name: searchName || undefined,
        host_type:
          filterHostType !== 'all' ? (filterHostType as HostType) : undefined,
        status: filterStatus !== 'all' ? (filterStatus as HostStatus) : undefined,
      };

      const result = await services.host.getHostsSafe(params);

      if (result.success && result.data) {
        setHosts(result.data.hosts || []);
        setTotal(result.data.total || 0);
      } else {
        toast.error(result.error || t('host.loadError'));
        setHosts([]);
        setTotal(0);
      }
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('host.loadError'));
      setHosts([]);
      setTotal(0);
    } finally {
      setLoading(false);
    }
  }, [currentPage, searchName, filterHostType, filterStatus, t]);

  useEffect(() => {
    loadHosts();
  }, [loadHosts]);

  /**
   * Handle search
   * 处理搜索
   */
  const handleSearch = () => {
    setCurrentPage(1);
    loadHosts();
  };

  /**
   * Handle refresh
   * 处理刷新
   */
  const handleRefresh = () => {
    loadHosts();
  };

  /**
   * Handle page change
   * 处理页面变化
   */
  const handlePageChange = (page: number) => {
    setCurrentPage(page);
  };

  /**
   * Handle view host detail
   * 处理查看主机详情
   */
  const handleViewDetail = (host: HostInfo) => {
    setSelectedHost(host);
    setIsDetailOpen(true);
  };

  /**
   * Handle edit host
   * 处理编辑主机
   */
  const handleEdit = (host: HostInfo) => {
    setSelectedHost(host);
    setIsEditDialogOpen(true);
  };

  /**
   * Handle delete host
   * 处理删除主机
   */
  const handleDelete = async (host: HostInfo) => {
    const result = await services.host.deleteHostSafe(host.id);
    if (result.success) {
      toast.success(t('host.deleteSuccess'));
      loadHosts();
    } else {
      toast.error(result.error || t('host.deleteError'));
    }
  };

  /**
   * Handle host created
   * 处理主机创建完成
   */
  const handleHostCreated = () => {
    setIsCreateDialogOpen(false);
    loadHosts();
    toast.success(t('host.createSuccess'));
  };

  /**
   * Handle host updated
   * 处理主机更新完成
   */
  const handleHostUpdated = () => {
    setIsEditDialogOpen(false);
    loadHosts();
    toast.success(t('host.updateSuccess'));
  };

  /**
   * Clear all filters
   * 清除所有过滤条件
   */
  const handleClearFilters = () => {
    setSearchName('');
    setFilterHostType('all');
    setFilterStatus('all');
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
          <Server className='h-6 w-6' />
          <div>
            <h1 className='text-2xl font-bold tracking-tight'>
              {t('host.title')}
            </h1>
            <p className='text-muted-foreground mt-1'>{t('host.description')}</p>
          </div>
        </div>
        <div className='flex gap-2'>
          <Button variant='outline' onClick={handleRefresh}>
            <RefreshCw className='h-4 w-4 mr-2' />
            {t('common.refresh')}
          </Button>
          <Button onClick={() => setIsCreateDialogOpen(true)}>
            <Plus className='h-4 w-4 mr-2' />
            {t('host.createHost')}
          </Button>
        </div>
      </motion.div>

      <Separator />

      {/* Filters / 过滤器 */}
      <motion.div
        className='flex flex-wrap gap-4 items-end'
        variants={itemVariants}
      >
        <div className='flex-1 min-w-[200px] max-w-sm'>
          <Input
            placeholder={t('host.searchPlaceholder')}
            value={searchName}
            onChange={(e) => setSearchName(e.target.value)}
            onKeyDown={(e) => e.key === 'Enter' && handleSearch()}
          />
        </div>

        <Select value={filterHostType} onValueChange={setFilterHostType}>
          <SelectTrigger className='w-[150px]'>
            <SelectValue placeholder={t('host.hostType')} />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value='all'>{t('host.allTypes')}</SelectItem>
            <SelectItem value={HostType.BARE_METAL}>
              {t('host.types.bareMetal')}
            </SelectItem>
            <SelectItem value={HostType.DOCKER}>
              {t('host.types.docker')}
            </SelectItem>
            <SelectItem value={HostType.KUBERNETES}>
              {t('host.types.kubernetes')}
            </SelectItem>
          </SelectContent>
        </Select>

        <Select value={filterStatus} onValueChange={setFilterStatus}>
          <SelectTrigger className='w-[150px]'>
            <SelectValue placeholder={t('host.status')} />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value='all'>{t('host.allStatuses')}</SelectItem>
            <SelectItem value={HostStatus.PENDING}>
              {t('host.statuses.pending')}
            </SelectItem>
            <SelectItem value={HostStatus.CONNECTED}>
              {t('host.statuses.connected')}
            </SelectItem>
            <SelectItem value={HostStatus.OFFLINE}>
              {t('host.statuses.offline')}
            </SelectItem>
            <SelectItem value={HostStatus.ERROR}>
              {t('host.statuses.error')}
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

      {/* Host Table / 主机表格 */}
      <motion.div variants={itemVariants}>
        <HostTable
          hosts={hosts}
          loading={loading}
          currentPage={currentPage}
          totalPages={totalPages}
          total={total}
          onPageChange={handlePageChange}
          onViewDetail={handleViewDetail}
          onEdit={handleEdit}
          onDelete={handleDelete}
        />
      </motion.div>

      {/* Create Host Dialog / 创建主机对话框 */}
      <CreateHostDialog
        open={isCreateDialogOpen}
        onOpenChange={setIsCreateDialogOpen}
        onSuccess={handleHostCreated}
      />

      {/* Edit Host Dialog / 编辑主机对话框 */}
      {selectedHost && (
        <EditHostDialog
          open={isEditDialogOpen}
          onOpenChange={setIsEditDialogOpen}
          host={selectedHost}
          onSuccess={handleHostUpdated}
        />
      )}

      {/* Host Detail / 主机详情 */}
      {selectedHost && (
        <HostDetail
          open={isDetailOpen}
          onOpenChange={setIsDetailOpen}
          host={selectedHost}
          onEdit={() => {
            setIsDetailOpen(false);
            setIsEditDialogOpen(true);
          }}
        />
      )}
    </motion.div>
  );
}
