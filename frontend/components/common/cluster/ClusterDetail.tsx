'use client';

/**
 * Cluster Detail Component
 * 集群详情组件
 *
 * Displays detailed information about a cluster including nodes and status.
 * 显示集群的详细信息，包括节点和状态。
 */

import {useState, useEffect, useCallback} from 'react';
import {useRouter} from 'next/navigation';
import {useTranslations} from 'next-intl';
import {Button} from '@/components/ui/button';
import {Badge} from '@/components/ui/badge';
import {Card, CardContent, CardHeader, CardTitle} from '@/components/ui/card';
import {Separator} from '@/components/ui/separator';
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
  ArrowLeft,
  Play,
  Square,
  RotateCcw,
  Pencil,
  Trash2,
  Plus,
  RefreshCw,
  Server,
  Activity,
  Loader2,
  FileText,
} from 'lucide-react';
import {Checkbox} from '@/components/ui/checkbox';
import {motion} from 'motion/react';
import services from '@/lib/services';
import {
  ClusterInfo,
  ClusterStatus,
  ClusterStatusInfo,
  NodeInfo,
  NodeStatus,
  HealthStatus,
} from '@/lib/services/cluster/types';
import {EditClusterDialog} from './EditClusterDialog';
import {AddNodeDialog} from './AddNodeDialog';
import {EditNodeDialog} from './EditNodeDialog';
import {ClusterPlugins} from './ClusterPlugins';

interface ClusterDetailProps {
  clusterId: number;
}

/**
 * Get status badge variant
 * 获取状态徽章变体
 */
function getStatusBadgeVariant(
  status: ClusterStatus | NodeStatus,
): 'default' | 'secondary' | 'destructive' | 'outline' {
  switch (status) {
    case ClusterStatus.RUNNING:
    case NodeStatus.RUNNING:
      return 'default';
    case ClusterStatus.CREATED:
    case ClusterStatus.STOPPED:
    case NodeStatus.PENDING:
    case NodeStatus.STOPPED:
      return 'secondary';
    case ClusterStatus.DEPLOYING:
    case NodeStatus.INSTALLING:
      return 'outline';
    case ClusterStatus.ERROR:
    case NodeStatus.ERROR:
      return 'destructive';
    default:
      return 'secondary';
  }
}

/**
 * Get health status badge variant
 * 获取健康状态徽章变体
 */
function getHealthBadgeVariant(
  status: HealthStatus,
): 'default' | 'secondary' | 'destructive' | 'outline' {
  switch (status) {
    case HealthStatus.HEALTHY:
      return 'default';
    case HealthStatus.UNHEALTHY:
      return 'destructive';
    case HealthStatus.UNKNOWN:
    default:
      return 'secondary';
  }
}

/**
 * Get role translation key
 * 获取角色翻译键
 * Handles special case for "master/worker" role
 * 处理 "master/worker" 角色的特殊情况
 */
function getRoleTranslationKey(role: string): string {
  // Replace "/" with "_" for i18n key compatibility
  // 将 "/" 替换为 "_" 以兼容 i18n 键
  if (role === 'master/worker') {
    return 'masterWorker';
  }
  return role;
}

/**
 * Cluster Detail Component
 * 集群详情组件
 */
export function ClusterDetail({clusterId}: ClusterDetailProps) {
  const t = useTranslations();
  const router = useRouter();

  // Data state / 数据状态
  const [cluster, setCluster] = useState<ClusterInfo | null>(null);
  const [nodes, setNodes] = useState<NodeInfo[]>([]);
  const [clusterStatus, setClusterStatus] = useState<ClusterStatusInfo | null>(null);
  const [loading, setLoading] = useState(true);
  const [isOperating, setIsOperating] = useState(false);

  // Dialog state / 对话框状态
  const [isEditDialogOpen, setIsEditDialogOpen] = useState(false);
  const [isAddNodeDialogOpen, setIsAddNodeDialogOpen] = useState(false);
  const [isEditNodeDialogOpen, setIsEditNodeDialogOpen] = useState(false);
  const [nodeToEdit, setNodeToEdit] = useState<NodeInfo | null>(null);
  const [isDeleteDialogOpen, setIsDeleteDialogOpen] = useState(false);
  const [nodeToRemove, setNodeToRemove] = useState<NodeInfo | null>(null);

  // Node selection state / 节点选择状态
  const [selectedNodeIds, setSelectedNodeIds] = useState<Set<number>>(new Set());
  const [nodeOperating, setNodeOperating] = useState<number | null>(null);
  const [isLogDialogOpen, setIsLogDialogOpen] = useState(false);
  const [logContent, setLogContent] = useState<string>('');
  const [logLoading, setLogLoading] = useState(false);
  const [logNodeInfo, setLogNodeInfo] = useState<NodeInfo | null>(null);
  // Log query parameters / 日志查询参数
  const [logLines, setLogLines] = useState<number>(100);
  const [logMode, setLogMode] = useState<string>('tail');
  const [logFilter, setLogFilter] = useState<string>('');
  const [logDate, setLogDate] = useState<string>('');

  /**
   * Load cluster data
   * 加载集群数据
   */
  const loadClusterData = useCallback(async () => {
    setLoading(true);
    try {
      const [clusterResult, nodesResult, statusResult] = await Promise.all([
        services.cluster.getClusterSafe(clusterId),
        services.cluster.getNodesSafe(clusterId),
        services.cluster.getClusterStatusSafe(clusterId),
      ]);

      if (clusterResult.success && clusterResult.data) {
        setCluster(clusterResult.data);
      } else {
        toast.error(clusterResult.error || t('cluster.loadError'));
      }

      if (nodesResult.success && nodesResult.data) {
        setNodes(nodesResult.data);
      }

      if (statusResult.success && statusResult.data) {
        setClusterStatus(statusResult.data);
      }
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('cluster.loadError'));
    } finally {
      setLoading(false);
    }
  }, [clusterId, t]);

  useEffect(() => {
    loadClusterData();
  }, [loadClusterData]);

  /**
   * Handle start cluster
   * 处理启动集群
   */
  const handleStart = async () => {
    setIsOperating(true);
    try {
      const result = await services.cluster.startClusterSafe(clusterId);
      if (result.success) {
        toast.success(t('cluster.startSuccess'));
        loadClusterData();
      } else {
        toast.error(result.error || t('cluster.startError'));
      }
    } finally {
      setIsOperating(false);
    }
  };

  /**
   * Handle stop cluster
   * 处理停止集群
   */
  const handleStop = async () => {
    setIsOperating(true);
    try {
      const result = await services.cluster.stopClusterSafe(clusterId);
      if (result.success) {
        toast.success(t('cluster.stopSuccess'));
        loadClusterData();
      } else {
        toast.error(result.error || t('cluster.stopError'));
      }
    } finally {
      setIsOperating(false);
    }
  };

  /**
   * Handle restart cluster
   * 处理重启集群
   */
  const handleRestart = async () => {
    setIsOperating(true);
    try {
      const result = await services.cluster.restartClusterSafe(clusterId);
      if (result.success) {
        toast.success(t('cluster.restartSuccess'));
        loadClusterData();
      } else {
        toast.error(result.error || t('cluster.restartError'));
      }
    } finally {
      setIsOperating(false);
    }
  };

  /**
   * Handle delete cluster
   * 处理删除集群
   */
  const handleDelete = async () => {
    const result = await services.cluster.deleteClusterSafe(clusterId);
    if (result.success) {
      toast.success(t('cluster.deleteSuccess'));
      router.push('/clusters');
    } else {
      toast.error(result.error || t('cluster.deleteError'));
    }
    setIsDeleteDialogOpen(false);
  };

  /**
   * Handle remove node
   * 处理移除节点
   */
  const handleRemoveNode = async () => {
    if (!nodeToRemove) {return;}

    const result = await services.cluster.removeNodeSafe(clusterId, nodeToRemove.id);
    if (result.success) {
      toast.success(t('cluster.removeNodeSuccess'));
      loadClusterData();
    } else {
      toast.error(result.error || t('cluster.removeNodeError'));
    }
    setNodeToRemove(null);
  };

  /**
   * Handle cluster updated
   * 处理集群更新完成
   */
  const handleClusterUpdated = () => {
    setIsEditDialogOpen(false);
    loadClusterData();
    toast.success(t('cluster.updateSuccess'));
  };

  /**
   * Handle node added
   * 处理节点添加完成
   */
  const handleNodeAdded = () => {
    setIsAddNodeDialogOpen(false);
    loadClusterData();
    toast.success(t('cluster.addNodeSuccess'));
  };

  /**
   * Handle node edited
   * 处理节点编辑完成
   */
  const handleNodeEdited = () => {
    setIsEditNodeDialogOpen(false);
    setNodeToEdit(null);
    loadClusterData();
    toast.success(t('cluster.editNodeSuccess'));
  };

  /**
   * Open edit node dialog
   * 打开编辑节点对话框
   */
  const openEditNodeDialog = (node: NodeInfo) => {
    setNodeToEdit(node);
    setIsEditNodeDialogOpen(true);
  };

  /**
   * Toggle node selection
   * 切换节点选择
   */
  const toggleNodeSelection = (nodeId: number) => {
    setSelectedNodeIds((prev) => {
      const newSet = new Set(prev);
      if (newSet.has(nodeId)) {
        newSet.delete(nodeId);
      } else {
        newSet.add(nodeId);
      }
      return newSet;
    });
  };

  /**
   * Toggle all nodes selection
   * 切换全选
   */
  const toggleAllNodes = () => {
    if (selectedNodeIds.size === nodes.length) {
      setSelectedNodeIds(new Set());
    } else {
      setSelectedNodeIds(new Set(nodes.map((n) => n.id)));
    }
  };

  /**
   * Handle single node start
   * 处理单个节点启动
   */
  const handleNodeStart = async (node: NodeInfo) => {
    setNodeOperating(node.id);
    try {
      const result = await services.cluster.startNodeSafe(clusterId, node.id);
      if (result.success) {
        toast.success(t('cluster.nodeStartSuccess'));
        loadClusterData();
      } else {
        toast.error(result.error || t('cluster.nodeStartError'));
      }
    } finally {
      setNodeOperating(null);
    }
  };

  /**
   * Handle single node stop
   * 处理单个节点停止
   */
  const handleNodeStop = async (node: NodeInfo) => {
    setNodeOperating(node.id);
    try {
      const result = await services.cluster.stopNodeSafe(clusterId, node.id);
      if (result.success) {
        toast.success(t('cluster.nodeStopSuccess'));
        loadClusterData();
      } else {
        toast.error(result.error || t('cluster.nodeStopError'));
      }
    } finally {
      setNodeOperating(null);
    }
  };

  /**
   * Handle single node restart
   * 处理单个节点重启
   */
  const handleNodeRestart = async (node: NodeInfo) => {
    setNodeOperating(node.id);
    try {
      const result = await services.cluster.restartNodeSafe(clusterId, node.id);
      if (result.success) {
        toast.success(t('cluster.nodeRestartSuccess'));
        loadClusterData();
      } else {
        toast.error(result.error || t('cluster.nodeRestartError'));
      }
    } finally {
      setNodeOperating(null);
    }
  };

  /**
   * Handle view node logs
   * 处理查看节点日志
   */
  const handleViewLogs = async (node: NodeInfo) => {
    setLogNodeInfo(node);
    setIsLogDialogOpen(true);
    // Reset parameters / 重置参数
    setLogLines(100);
    setLogMode('tail');
    setLogFilter('');
    setLogDate('');
    await fetchLogs(node.id, {lines: 100, mode: 'tail'});
  };

  /**
   * Fetch logs with parameters
   * 使用参数获取日志
   */
  const fetchLogs = async (
    nodeId: number,
    params: {lines?: number; mode?: string; filter?: string; date?: string},
  ) => {
    setLogLoading(true);
    setLogContent('');
    try {
      const result = await services.cluster.getNodeLogsSafe(clusterId, nodeId, params);
      if (result.success && result.data) {
        setLogContent(result.data.logs || t('cluster.noLogs'));
      } else {
        setLogContent(result.error || t('cluster.getLogsError'));
      }
    } finally {
      setLogLoading(false);
    }
  };

  /**
   * Handle refresh logs with current parameters
   * 使用当前参数刷新日志
   */
  const handleRefreshLogs = () => {
    if (!logNodeInfo) {return;}
    fetchLogs(logNodeInfo.id, {
      lines: logLines,
      mode: logMode,
      filter: logFilter || undefined,
      date: logDate || undefined,
    });
  };

  /**
   * Handle batch start selected nodes
   * 处理批量启动选中节点
   */
  const handleBatchStart = async () => {
    if (selectedNodeIds.size === 0) {
      toast.warning(t('cluster.selectNodesFirst'));
      return;
    }
    setIsOperating(true);
    try {
      for (const nodeId of selectedNodeIds) {
        await services.cluster.startNodeSafe(clusterId, nodeId);
      }
      toast.success(t('cluster.batchStartSuccess'));
      loadClusterData();
    } finally {
      setIsOperating(false);
    }
  };

  /**
   * Handle batch stop selected nodes
   * 处理批量停止选中节点
   */
  const handleBatchStop = async () => {
    if (selectedNodeIds.size === 0) {
      toast.warning(t('cluster.selectNodesFirst'));
      return;
    }
    setIsOperating(true);
    try {
      for (const nodeId of selectedNodeIds) {
        await services.cluster.stopNodeSafe(clusterId, nodeId);
      }
      toast.success(t('cluster.batchStopSuccess'));
      loadClusterData();
    } finally {
      setIsOperating(false);
    }
  };

  /**
   * Handle batch restart selected nodes
   * 处理批量重启选中节点
   */
  const handleBatchRestart = async () => {
    if (selectedNodeIds.size === 0) {
      toast.warning(t('cluster.selectNodesFirst'));
      return;
    }
    setIsOperating(true);
    try {
      for (const nodeId of selectedNodeIds) {
        await services.cluster.restartNodeSafe(clusterId, nodeId);
      }
      toast.success(t('cluster.batchRestartSuccess'));
      loadClusterData();
    } finally {
      setIsOperating(false);
    }
  };

  if (loading) {
    return (
      <div className='flex items-center justify-center py-12'>
        <Loader2 className='h-8 w-8 animate-spin text-muted-foreground' />
      </div>
    );
  }

  if (!cluster) {
    return (
      <div className='text-center py-12'>
        <p className='text-muted-foreground'>{t('cluster.notFound')}</p>
        <Button variant='outline' className='mt-4' onClick={() => router.push('/clusters')}>
          <ArrowLeft className='h-4 w-4 mr-2' />
          {t('cluster.backToList')}
        </Button>
      </div>
    );
  }

  const canStart = cluster.status === ClusterStatus.CREATED || cluster.status === ClusterStatus.STOPPED;
  const canStop = cluster.status === ClusterStatus.RUNNING;
  const canRestart = cluster.status === ClusterStatus.RUNNING;
  const canDelete = cluster.status !== ClusterStatus.RUNNING && cluster.status !== ClusterStatus.DEPLOYING;

  const containerVariants = {
    hidden: {opacity: 0},
    visible: {
      opacity: 1,
      transition: {duration: 0.5, staggerChildren: 0.1},
    },
  };

  const itemVariants = {
    hidden: {opacity: 0, y: 20},
    visible: {opacity: 1, y: 0, transition: {duration: 0.6}},
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
        <div className='flex items-center gap-4'>
          <Button variant='ghost' size='icon' onClick={() => router.push('/clusters')}>
            <ArrowLeft className='h-5 w-5' />
          </Button>
          <div>
            <h1 className='text-2xl font-bold tracking-tight'>{cluster.name}</h1>
            {cluster.description && (
              <p className='text-muted-foreground mt-1'>{cluster.description}</p>
            )}
          </div>
        </div>
        <div className='flex gap-2'>
          <Button variant='outline' onClick={loadClusterData}>
            <RefreshCw className='h-4 w-4 mr-2' />
            {t('common.refresh')}
          </Button>
          <Button variant='outline' onClick={() => setIsEditDialogOpen(true)}>
            <Pencil className='h-4 w-4 mr-2' />
            {t('common.edit')}
          </Button>
          <Button
            variant='destructive'
            onClick={() => setIsDeleteDialogOpen(true)}
            disabled={!canDelete}
          >
            <Trash2 className='h-4 w-4 mr-2' />
            {t('common.delete')}
          </Button>
        </div>
      </motion.div>

      <Separator />

      {/* Cluster Info Cards / 集群信息卡片 */}
      <motion.div
        className='grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4'
        variants={itemVariants}
      >
        <Card>
          <CardHeader className='pb-2'>
            <CardTitle className='text-sm font-medium text-muted-foreground'>
              {t('cluster.status')}
            </CardTitle>
          </CardHeader>
          <CardContent>
            <Badge variant={getStatusBadgeVariant(cluster.status)} className='text-sm'>
              {t(`cluster.statuses.${cluster.status}`)}
            </Badge>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className='pb-2'>
            <CardTitle className='text-sm font-medium text-muted-foreground'>
              {t('cluster.healthStatus')}
            </CardTitle>
          </CardHeader>
          <CardContent>
            {clusterStatus ? (
              <Badge variant={getHealthBadgeVariant(clusterStatus.health_status)} className='text-sm'>
                {t(`cluster.healthStatuses.${clusterStatus.health_status}`)}
              </Badge>
            ) : (
              <span className='text-muted-foreground'>-</span>
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader className='pb-2'>
            <CardTitle className='text-sm font-medium text-muted-foreground'>
              {t('cluster.nodes')}
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className='flex items-center gap-2'>
              <Server className='h-5 w-5 text-muted-foreground' />
              <span className='text-2xl font-bold'>{cluster.node_count}</span>
              {clusterStatus && (
                <span className='text-sm text-muted-foreground'>
                  ({clusterStatus.online_nodes} {t('cluster.online')})
                </span>
              )}
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className='pb-2'>
            <CardTitle className='text-sm font-medium text-muted-foreground'>
              {t('cluster.deploymentMode')}
            </CardTitle>
          </CardHeader>
          <CardContent>
            <Badge variant='outline' className='text-sm'>
              {t(`cluster.modes.${cluster.deployment_mode}`)}
            </Badge>
          </CardContent>
        </Card>
      </motion.div>

      {/* Cluster Actions / 集群操作 */}
      <motion.div variants={itemVariants}>
        <Card>
          <CardHeader>
            <CardTitle className='flex items-center gap-2'>
              <Activity className='h-5 w-5' />
              {t('cluster.operations')}
              {selectedNodeIds.size > 0 && (
                <Badge variant='secondary' className='ml-2'>
                  {t('cluster.selectedNodes', {count: selectedNodeIds.size})}
                </Badge>
              )}
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className='flex gap-2'>
              <Button
                variant='outline'
                onClick={handleBatchStart}
                disabled={selectedNodeIds.size === 0 || isOperating}
              >
                {isOperating ? (
                  <Loader2 className='h-4 w-4 mr-2 animate-spin' />
                ) : (
                  <Play className='h-4 w-4 mr-2' />
                )}
                {t('cluster.start')}
              </Button>
              <Button
                variant='outline'
                onClick={handleBatchStop}
                disabled={selectedNodeIds.size === 0 || isOperating}
              >
                {isOperating ? (
                  <Loader2 className='h-4 w-4 mr-2 animate-spin' />
                ) : (
                  <Square className='h-4 w-4 mr-2' />
                )}
                {t('cluster.stop')}
              </Button>
              <Button
                variant='outline'
                onClick={handleBatchRestart}
                disabled={selectedNodeIds.size === 0 || isOperating}
              >
                {isOperating ? (
                  <Loader2 className='h-4 w-4 mr-2 animate-spin' />
                ) : (
                  <RotateCcw className='h-4 w-4 mr-2' />
                )}
                {t('cluster.restart')}
              </Button>
            </div>
            {selectedNodeIds.size === 0 && (
              <p className='text-sm text-muted-foreground mt-2'>
                {t('cluster.selectNodesToOperate')}
              </p>
            )}
          </CardContent>
        </Card>
      </motion.div>

      {/* Nodes Table / 节点表格 */}
      <motion.div variants={itemVariants}>
        <Card>
          <CardHeader className='flex flex-row items-center justify-between'>
            <CardTitle className='flex items-center gap-2'>
              <Server className='h-5 w-5' />
              {t('cluster.nodeList')}
            </CardTitle>
            <Button onClick={() => setIsAddNodeDialogOpen(true)}>
              <Plus className='h-4 w-4 mr-2' />
              {t('cluster.addNode')}
            </Button>
          </CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className='w-12'>
                    <Checkbox
                      checked={nodes.length > 0 && selectedNodeIds.size === nodes.length}
                      onCheckedChange={toggleAllNodes}
                    />
                  </TableHead>
                  <TableHead>ID</TableHead>
                  <TableHead>{t('cluster.hostName')}</TableHead>
                  <TableHead>{t('cluster.hostIP')}</TableHead>
                  <TableHead>{t('cluster.nodeRole')}</TableHead>
                  <TableHead>{t('cluster.installDir')}</TableHead>
                  <TableHead>{t('cluster.nodeStatus')}</TableHead>
                  <TableHead>{t('cluster.processPID')}</TableHead>
                  <TableHead>{t('cluster.actions')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {nodes.length === 0 ? (
                  <TableRow>
                    <TableCell colSpan={9} className='text-center py-8 text-muted-foreground'>
                      {t('cluster.noNodes')}
                    </TableCell>
                  </TableRow>
                ) : (
                  nodes.map((node) => (
                    <TableRow key={node.id}>
                      <TableCell>
                        <Checkbox
                          checked={selectedNodeIds.has(node.id)}
                          onCheckedChange={() => toggleNodeSelection(node.id)}
                        />
                      </TableCell>
                      <TableCell>{node.id}</TableCell>
                      <TableCell>{node.host_name || '-'}</TableCell>
                      <TableCell>{node.host_ip || '-'}</TableCell>
                      <TableCell>
                        <Badge variant='outline'>
                          {t(`cluster.roles.${getRoleTranslationKey(node.role)}`)}
                        </Badge>
                      </TableCell>
                      <TableCell className='font-mono text-sm'>{node.install_dir || '-'}</TableCell>
                      <TableCell>
                        <Badge variant={getStatusBadgeVariant(node.status)}>
                          {t(`cluster.nodeStatuses.${node.status}`)}
                        </Badge>
                      </TableCell>
                      <TableCell>{node.process_pid || '-'}</TableCell>
                      <TableCell>
                        <div className='flex items-center gap-1'>
                          <Button
                            variant='ghost'
                            size='icon'
                            onClick={() => handleNodeStart(node)}
                            disabled={nodeOperating === node.id || node.status === NodeStatus.RUNNING}
                            title={t('cluster.start')}
                          >
                            {nodeOperating === node.id ? (
                              <Loader2 className='h-4 w-4 animate-spin' />
                            ) : (
                              <Play className='h-4 w-4 text-green-600' />
                            )}
                          </Button>
                          <Button
                            variant='ghost'
                            size='icon'
                            onClick={() => handleNodeStop(node)}
                            disabled={nodeOperating === node.id || node.status !== NodeStatus.RUNNING}
                            title={t('cluster.stop')}
                          >
                            <Square className='h-4 w-4 text-orange-600' />
                          </Button>
                          <Button
                            variant='ghost'
                            size='icon'
                            onClick={() => handleNodeRestart(node)}
                            disabled={nodeOperating === node.id || node.status !== NodeStatus.RUNNING}
                            title={t('cluster.restart')}
                          >
                            <RotateCcw className='h-4 w-4 text-blue-600' />
                          </Button>
                          <Button
                            variant='ghost'
                            size='icon'
                            onClick={() => handleViewLogs(node)}
                            title={t('cluster.viewLogs')}
                          >
                            <FileText className='h-4 w-4' />
                          </Button>
                          <Button
                            variant='ghost'
                            size='icon'
                            onClick={() => openEditNodeDialog(node)}
                            title={t('cluster.editNode')}
                          >
                            <Pencil className='h-4 w-4' />
                          </Button>
                          <Button
                            variant='ghost'
                            size='icon'
                            onClick={() => setNodeToRemove(node)}
                            title={t('cluster.removeNode')}
                          >
                            <Trash2 className='h-4 w-4 text-destructive' />
                          </Button>
                        </div>
                      </TableCell>
                    </TableRow>
                  ))
                )}
              </TableBody>
            </Table>
          </CardContent>
        </Card>
      </motion.div>

      {/* Installed Plugins / 已安装插件 */}
      <motion.div variants={itemVariants}>
        <ClusterPlugins clusterId={clusterId} />
      </motion.div>

      {/* Cluster Info / 集群信息 */}
      <motion.div variants={itemVariants}>
        <Card>
          <CardHeader>
            <CardTitle>{t('cluster.clusterInfo')}</CardTitle>
          </CardHeader>
          <CardContent>
            <div className='grid grid-cols-3 gap-4'>
              <div>
                <span className='text-sm text-muted-foreground'>{t('cluster.version')}</span>
                <p className='font-medium'>{cluster.version || '-'}</p>
              </div>
              <div>
                <span className='text-sm text-muted-foreground'>{t('cluster.createdAt')}</span>
                <p className='font-medium'>{new Date(cluster.created_at).toLocaleString()}</p>
              </div>
              <div>
                <span className='text-sm text-muted-foreground'>{t('cluster.updatedAt')}</span>
                <p className='font-medium'>{new Date(cluster.updated_at).toLocaleString()}</p>
              </div>
            </div>
          </CardContent>
        </Card>
      </motion.div>

      {/* Edit Cluster Dialog / 编辑集群对话框 */}
      <EditClusterDialog
        open={isEditDialogOpen}
        onOpenChange={setIsEditDialogOpen}
        cluster={cluster}
        onSuccess={handleClusterUpdated}
      />

      {/* Add Node Dialog / 添加节点对话框 */}
      <AddNodeDialog
        open={isAddNodeDialogOpen}
        onOpenChange={setIsAddNodeDialogOpen}
        clusterId={clusterId}
        deploymentMode={cluster.deployment_mode}
        onSuccess={handleNodeAdded}
      />

      {/* Edit Node Dialog / 编辑节点对话框 */}
      <EditNodeDialog
        open={isEditNodeDialogOpen}
        onOpenChange={setIsEditNodeDialogOpen}
        node={nodeToEdit}
        deploymentMode={cluster.deployment_mode}
        onSuccess={handleNodeEdited}
      />

      {/* Delete Cluster Dialog / 删除集群对话框 */}
      <AlertDialog open={isDeleteDialogOpen} onOpenChange={setIsDeleteDialogOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('cluster.deleteCluster')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t('cluster.deleteConfirm', {name: cluster.name})}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('common.cancel')}</AlertDialogCancel>
            <AlertDialogAction onClick={handleDelete}>
              {t('common.delete')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      {/* Remove Node Dialog / 移除节点对话框 */}
      <AlertDialog open={!!nodeToRemove} onOpenChange={() => setNodeToRemove(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('cluster.removeNode')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t('cluster.removeNodeConfirm', {name: nodeToRemove?.host_name || String(nodeToRemove?.id || '')})}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('common.cancel')}</AlertDialogCancel>
            <AlertDialogAction onClick={handleRemoveNode}>
              {t('common.delete')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      {/* View Logs Dialog / 查看日志对话框 */}
      <AlertDialog open={isLogDialogOpen} onOpenChange={setIsLogDialogOpen}>
        <AlertDialogContent className='max-h-[90vh]' style={{maxWidth: '90vw', width: '1200px'}}>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {t('cluster.viewLogs')} - {logNodeInfo?.host_name} ({t(`cluster.roles.${getRoleTranslationKey(logNodeInfo?.role || '')}`)})
            </AlertDialogTitle>
          </AlertDialogHeader>
          {/* Log query parameters / 日志查询参数 */}
          <div className='flex flex-wrap gap-3 items-end'>
            <div className='flex flex-col gap-1'>
              <label className='text-xs text-muted-foreground'>{t('cluster.logLines')}</label>
              <input
                type='number'
                value={logLines}
                onChange={(e) => setLogLines(Number(e.target.value) || 100)}
                className='w-20 h-8 px-2 text-sm border rounded-md bg-background text-foreground'
                min={1}
                max={10000}
              />
            </div>
            <div className='flex flex-col gap-1'>
              <label className='text-xs text-muted-foreground'>{t('cluster.logMode')}</label>
              <select
                value={logMode}
                onChange={(e) => setLogMode(e.target.value)}
                className='h-8 px-2 text-sm border rounded-md bg-background text-foreground'
              >
                <option value='tail'>{t('cluster.logModeTail')}</option>
                <option value='head'>{t('cluster.logModeHead')}</option>
                <option value='all'>{t('cluster.logModeAll')}</option>
              </select>
            </div>
            <div className='flex flex-col gap-1'>
              <label className='text-xs text-muted-foreground'>{t('cluster.logFilter')}</label>
              <input
                type='text'
                value={logFilter}
                onChange={(e) => setLogFilter(e.target.value)}
                placeholder='grep pattern'
                className='w-32 h-8 px-2 text-sm border rounded-md bg-background text-foreground placeholder:text-muted-foreground'
              />
            </div>
            <div className='flex flex-col gap-1'>
              <label className='text-xs text-muted-foreground'>{t('cluster.logDate')}</label>
              <input
                type='text'
                value={logDate}
                onChange={(e) => setLogDate(e.target.value)}
                placeholder='2025-11-12-1'
                className='w-36 h-8 px-2 text-sm border rounded-md bg-background text-foreground placeholder:text-muted-foreground'
              />
            </div>
            <Button
              variant='outline'
              size='sm'
              onClick={handleRefreshLogs}
              disabled={logLoading}
            >
              {logLoading ? (
                <Loader2 className='h-4 w-4 animate-spin' />
              ) : (
                <RefreshCw className='h-4 w-4' />
              )}
              <span className='ml-1'>{t('common.refresh')}</span>
            </Button>
          </div>
          <div className='overflow-auto h-[60vh] bg-muted rounded-md p-4'>
            {logLoading ? (
              <div className='flex items-center justify-center py-8'>
                <Loader2 className='h-6 w-6 animate-spin' />
              </div>
            ) : (
              <pre className='text-xs font-mono whitespace-pre-wrap'>{logContent}</pre>
            )}
          </div>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('common.close')}</AlertDialogCancel>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </motion.div>
  );
}
