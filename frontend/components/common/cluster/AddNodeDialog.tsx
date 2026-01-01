'use client';

/**
 * Add Node Dialog Component
 * 添加节点对话框组件
 *
 * Dialog for adding a node to a cluster with installation directory.
 * Supports process discovery to auto-fill install directory and role.
 * 用于向集群添加节点的对话框，包含安装目录配置。
 * 支持进程发现以自动填充安装目录和角色。
 */

import {useState, useEffect} from 'react';
import {useTranslations} from 'next-intl';
import {Button} from '@/components/ui/button';
import {Input} from '@/components/ui/input';
import {Label} from '@/components/ui/label';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import {Loader2, Server, CheckCircle2, XCircle, AlertCircle, Search, Cpu} from 'lucide-react';
import {toast} from 'sonner';
import services from '@/lib/services';
import {NodeRole, AddNodeRequest, DefaultPorts, DeploymentMode, PrecheckResult, PrecheckCheckItem} from '@/lib/services/cluster/types';
import {HostInfo, AgentStatus} from '@/lib/services/host/types';
import {DiscoveredProcess} from '@/lib/services/discovery/discovery.service';

interface AddNodeDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  clusterId: number;
  deploymentMode: DeploymentMode;
  onSuccess: () => void;
}

/**
 * Add Node Dialog Component
 * 添加节点对话框组件
 */
export function AddNodeDialog({
  open,
  onOpenChange,
  clusterId,
  deploymentMode,
  onSuccess,
}: AddNodeDialogProps) {
  const t = useTranslations();
  const [loading, setLoading] = useState(false);
  const [loadingHosts, setLoadingHosts] = useState(false);

  // Form state / 表单状态
  const [hostId, setHostId] = useState<string>('');
  const [role, setRole] = useState<NodeRole>(NodeRole.WORKER);
  const [installDir, setInstallDir] = useState('/opt/seatunnel');

  // Port configuration / 端口配置
  const [hazelcastPort, setHazelcastPort] = useState<number>(DefaultPorts.WORKER_HAZELCAST);
  const [apiPort, setApiPort] = useState<number>(DefaultPorts.MASTER_API);
  const [workerPort, setWorkerPort] = useState<number>(DefaultPorts.WORKER_HAZELCAST);

  // Available hosts / 可用主机
  const [availableHosts, setAvailableHosts] = useState<HostInfo[]>([]);

  // Precheck state / 预检查状态
  const [precheckLoading, setPrecheckLoading] = useState(false);
  const [precheckResult, setPrecheckResult] = useState<PrecheckResult | null>(null);

  // Process discovery state / 进程发现状态
  const [discoveryLoading, setDiscoveryLoading] = useState(false);
  const [discoveredProcesses, setDiscoveredProcesses] = useState<DiscoveredProcess[]>([]);
  const [selectedProcess, setSelectedProcess] = useState<string>('');

  // Update default ports when role changes / 角色变化时更新默认端口
  useEffect(() => {
    if (role === NodeRole.MASTER) {
      setHazelcastPort(DefaultPorts.MASTER_HAZELCAST);
      setApiPort(DefaultPorts.MASTER_API);
      if (deploymentMode === DeploymentMode.HYBRID) {
        setWorkerPort(DefaultPorts.WORKER_HAZELCAST);
      }
    } else {
      setHazelcastPort(DefaultPorts.WORKER_HAZELCAST);
    }
  }, [role, deploymentMode]);

  /**
   * Load available hosts
   * 加载可用主机
   */
  useEffect(() => {
    if (open) {
      loadAvailableHosts();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open]);

  /**
   * Load available hosts with Agent installed
   * 加载已安装 Agent 的可用主机
   */
  const loadAvailableHosts = async () => {
    setLoadingHosts(true);
    try {
      const result = await services.host.getHostsSafe({
        current: 1,
        size: 100,
        agent_status: AgentStatus.INSTALLED,
      });

      if (result.success && result.data) {
        setAvailableHosts(result.data.hosts || []);
      } else {
        toast.error(result.error || t('host.loadError'));
        setAvailableHosts([]);
      }
    } finally {
      setLoadingHosts(false);
    }
  };

  /**
   * Reset form
   * 重置表单
   */
  const resetForm = () => {
    setHostId('');
    setRole(NodeRole.WORKER);
    setInstallDir('/opt/seatunnel');
    setHazelcastPort(DefaultPorts.WORKER_HAZELCAST);
    setApiPort(DefaultPorts.MASTER_API);
    setWorkerPort(DefaultPorts.WORKER_HAZELCAST);
    setPrecheckResult(null);
    setDiscoveredProcesses([]);
    setSelectedProcess('');
  };

  /**
   * Handle process discovery
   * 处理进程发现
   */
  const handleDiscoverProcesses = async () => {
    if (!hostId) {
      toast.error(t('cluster.hostRequired'));
      return;
    }

    setDiscoveryLoading(true);
    setDiscoveredProcesses([]);
    setSelectedProcess('');
    try {
      const result = await services.discovery.discoverProcesses(parseInt(hostId, 10));
      
      if (result.success && result.processes) {
        setDiscoveredProcesses(result.processes);
        if (result.processes.length === 0) {
          toast.info(t('discovery.noProcessesFound'));
        } else {
          toast.success(t('discovery.processesFound', {count: result.processes.length}));
        }
      } else {
        toast.error(result.message || t('discovery.discoverError'));
      }
    } catch {
      toast.error(t('discovery.discoverError'));
    } finally {
      setDiscoveryLoading(false);
    }
  };

  /**
   * Handle process selection - auto-fill form
   * 处理进程选择 - 自动填充表单
   */
  const handleProcessSelect = (processKey: string) => {
    setSelectedProcess(processKey);
    
    const process = discoveredProcesses.find(
      p => `${p.pid}-${p.install_dir}` === processKey
    );
    
    if (process) {
      // Auto-fill install directory / 自动填充安装目录
      setInstallDir(process.install_dir);
      
      // Auto-fill role based on discovered role / 根据发现的角色自动填充
      if (process.role === 'master') {
        setRole(NodeRole.MASTER);
      } else if (process.role === 'worker') {
        setRole(NodeRole.WORKER);
      }
      // For hybrid, keep current selection / 对于 hybrid，保持当前选择
      
      toast.success(t('discovery.processSelected'));
    }
  };

  /**
   * Handle precheck
   * 处理预检查
   */
  const handlePrecheck = async () => {
    if (!hostId) {
      toast.error(t('cluster.hostRequired'));
      return;
    }

    if (!hazelcastPort || hazelcastPort <= 0) {
      toast.error(t('cluster.hazelcastPortRequired'));
      return;
    }

    setPrecheckLoading(true);
    setPrecheckResult(null);
    try {
      const result = await services.cluster.precheckNodeSafe(clusterId, {
        host_id: parseInt(hostId, 10),
        role: role,
        install_dir: installDir.trim() || '/opt/seatunnel',
        hazelcast_port: hazelcastPort,
        api_port: role === NodeRole.MASTER && apiPort > 0 ? apiPort : undefined,
        worker_port: deploymentMode === DeploymentMode.HYBRID && role === NodeRole.MASTER ? workerPort : undefined,
      });

      if (result.success && result.data) {
        setPrecheckResult(result.data);
        if (result.data.success) {
          toast.success(t('cluster.precheckPassed'));
        } else {
          toast.warning(t('cluster.precheckFailed'));
        }
      } else {
        toast.error(result.error || t('cluster.precheckError'));
      }
    } finally {
      setPrecheckLoading(false);
    }
  };

  /**
   * Get status icon for precheck item
   * 获取预检查项的状态图标
   */
  const getStatusIcon = (status: string) => {
    switch (status) {
      case 'passed':
        return <CheckCircle2 className='h-4 w-4 text-green-500' />;
      case 'failed':
        return <XCircle className='h-4 w-4 text-red-500' />;
      case 'skipped':
        return <AlertCircle className='h-4 w-4 text-yellow-500' />;
      default:
        return null;
    }
  };

  /**
   * Handle submit
   * 处理提交
   */
  const handleSubmit = async () => {
    if (!hostId) {
      toast.error(t('cluster.hostRequired'));
      return;
    }

    if (!installDir.trim()) {
      toast.error(t('cluster.installDirRequired'));
      return;
    }

    setLoading(true);
    try {
      const data: AddNodeRequest = {
        host_id: parseInt(hostId, 10),
        role: role,
        install_dir: installDir.trim(),
        hazelcast_port: hazelcastPort,
        api_port: role === NodeRole.MASTER && apiPort > 0 ? apiPort : undefined,
        worker_port: deploymentMode === DeploymentMode.HYBRID && role === NodeRole.MASTER ? workerPort : undefined,
      };

      const result = await services.cluster.addNodeSafe(clusterId, data);

      if (result.success) {
        resetForm();
        onSuccess();
      } else {
        toast.error(result.error || t('cluster.addNodeError'));
      }
    } finally {
      setLoading(false);
    }
  };

  /**
   * Handle close
   * 处理关闭
   */
  const handleClose = (open: boolean) => {
    if (!open) {
      resetForm();
    }
    onOpenChange(open);
  };

  return (
    <Dialog open={open} onOpenChange={handleClose}>
      <DialogContent className='sm:max-w-[550px]'>
        <DialogHeader>
          <DialogTitle>{t('cluster.addNode')}</DialogTitle>
          <DialogDescription>{t('cluster.addNodeDescription')}</DialogDescription>
        </DialogHeader>

        <div className='space-y-4 py-4'>
          {/* Host Selection / 主机选择 */}
          <div className='space-y-2'>
            <Label htmlFor='host'>
              {t('cluster.selectHost')} <span className='text-destructive'>*</span>
            </Label>
            {loadingHosts ? (
              <div className='flex items-center gap-2 text-muted-foreground'>
                <Loader2 className='h-4 w-4 animate-spin' />
                {t('common.loading')}
              </div>
            ) : availableHosts.length === 0 ? (
              <div className='text-sm text-muted-foreground p-4 border rounded-md'>
                {t('cluster.noAvailableHosts')}
              </div>
            ) : (
              <div className='flex gap-2'>
                <Select value={hostId} onValueChange={(value) => {
                  setHostId(value);
                  setDiscoveredProcesses([]);
                  setSelectedProcess('');
                }}>
                  <SelectTrigger className='flex-1'>
                    <SelectValue placeholder={t('cluster.selectHostPlaceholder')} />
                  </SelectTrigger>
                  <SelectContent>
                    {availableHosts.map((host) => (
                      <SelectItem key={host.id} value={host.id.toString()}>
                        <div className='flex items-center gap-2'>
                          <Server className='h-4 w-4' />
                          <span>{host.name}</span>
                          {host.ip_address && (
                            <span className='text-muted-foreground'>({host.ip_address})</span>
                          )}
                        </div>
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                <Button
                  variant='outline'
                  size='icon'
                  onClick={handleDiscoverProcesses}
                  disabled={!hostId || discoveryLoading}
                  title={t('discovery.discoverProcesses')}
                >
                  {discoveryLoading ? (
                    <Loader2 className='h-4 w-4 animate-spin' />
                  ) : (
                    <Search className='h-4 w-4' />
                  )}
                </Button>
              </div>
            )}
            <p className='text-xs text-muted-foreground'>
              {t('cluster.onlyAgentInstalledHosts')}
            </p>
          </div>

          {/* Discovered Processes / 发现的进程 */}
          {discoveredProcesses.length > 0 && (
            <div className='space-y-2'>
              <Label>{t('discovery.discoveredProcesses')}</Label>
              <Select value={selectedProcess} onValueChange={handleProcessSelect}>
                <SelectTrigger>
                  <SelectValue placeholder={t('discovery.selectProcess')} />
                </SelectTrigger>
                <SelectContent>
                  {discoveredProcesses.map((proc) => (
                    <SelectItem 
                      key={`${proc.pid}-${proc.install_dir}`} 
                      value={`${proc.pid}-${proc.install_dir}`}
                    >
                      <div className='flex items-center gap-2'>
                        <Cpu className='h-4 w-4' />
                        <span>PID: {proc.pid}</span>
                        <span className='text-muted-foreground'>|</span>
                        <span className='capitalize'>{proc.role}</span>
                        <span className='text-muted-foreground'>|</span>
                        <span className='text-xs text-muted-foreground truncate max-w-[200px]'>
                          {proc.install_dir}
                        </span>
                      </div>
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <p className='text-xs text-muted-foreground'>
                {t('discovery.selectProcessHint')}
              </p>
            </div>
          )}

          {/* Node Role / 节点角色 */}
          <div className='space-y-2'>
            <Label htmlFor='role'>
              {t('cluster.nodeRole')} <span className='text-destructive'>*</span>
            </Label>
            <Select value={role} onValueChange={(value) => setRole(value as NodeRole)}>
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value={NodeRole.MASTER}>
                  {t('cluster.roles.master')}
                </SelectItem>
                <SelectItem value={NodeRole.WORKER}>
                  {t('cluster.roles.worker')}
                </SelectItem>
              </SelectContent>
            </Select>
            <p className='text-xs text-muted-foreground'>
              {role === NodeRole.MASTER
                ? t('cluster.masterRoleDescription')
                : t('cluster.workerRoleDescription')}
            </p>
          </div>

          {/* Installation Directory / 安装目录 */}
          <div className='space-y-2'>
            <Label htmlFor='installDir'>
              {t('cluster.installDir')} <span className='text-destructive'>*</span>
            </Label>
            <Input
              id='installDir'
              value={installDir}
              onChange={(e) => setInstallDir(e.target.value)}
              placeholder={t('cluster.installDirPlaceholder')}
            />
            <p className='text-xs text-muted-foreground'>
              {t('cluster.nodeInstallDirDescription')}
            </p>
          </div>

          {/* Port Configuration / 端口配置 */}
          <div className='space-y-3'>
            <Label>{t('cluster.portConfig')}</Label>
            
            <div className='grid grid-cols-2 gap-4'>
              <div className='space-y-1'>
                <Label htmlFor='hazelcastPort' className='text-xs'>
                  {t('cluster.hazelcastPort')} <span className='text-destructive'>*</span>
                </Label>
                <Input
                  id='hazelcastPort'
                  type='number'
                  value={hazelcastPort}
                  onChange={(e) => setHazelcastPort(parseInt(e.target.value, 10) || 0)}
                  placeholder={role === NodeRole.MASTER ? '5801' : '5802'}
                  required
                />
              </div>

              {role === NodeRole.MASTER && (
                <div className='space-y-1'>
                  <Label htmlFor='apiPort' className='text-xs'>
                    {t('cluster.apiPort')} <span className='text-muted-foreground'>({t('common.optional')})</span>
                  </Label>
                  <Input
                    id='apiPort'
                    type='number'
                    value={apiPort || ''}
                    onChange={(e) => setApiPort(parseInt(e.target.value, 10) || 0)}
                    placeholder='8080'
                  />
                </div>
              )}

              {deploymentMode === DeploymentMode.HYBRID && role === NodeRole.MASTER && (
                <div className='space-y-1'>
                  <Label htmlFor='workerPort' className='text-xs'>
                    {t('cluster.workerPort')}
                  </Label>
                  <Input
                    id='workerPort'
                    type='number'
                    value={workerPort}
                    onChange={(e) => setWorkerPort(parseInt(e.target.value, 10) || 0)}
                    placeholder='5802'
                  />
                </div>
              )}
            </div>
            <p className='text-xs text-muted-foreground'>
              {t('cluster.portConfigDescription')}
            </p>
          </div>

          {/* Precheck Results / 预检查结果 */}
          {precheckResult && (
            <div className='space-y-2 p-3 border rounded-md bg-muted/50'>
              <div className='flex items-center gap-2'>
                {precheckResult.success ? (
                  <CheckCircle2 className='h-5 w-5 text-green-500' />
                ) : (
                  <XCircle className='h-5 w-5 text-red-500' />
                )}
                <span className='font-medium text-sm'>
                  {precheckResult.success ? t('cluster.precheckPassed') : t('cluster.precheckFailed')}
                </span>
              </div>
              <div className='space-y-1'>
                {precheckResult.checks.map((check: PrecheckCheckItem, index: number) => (
                  <div key={index} className='flex items-start gap-2 text-xs'>
                    {getStatusIcon(check.status)}
                    <div>
                      <span className='font-medium'>{t(`cluster.precheckItems.${check.name}`, {defaultValue: check.name})}: </span>
                      <span className='text-muted-foreground'>{check.message}</span>
                    </div>
                  </div>
                ))}
              </div>
            </div>
          )}
        </div>

        <DialogFooter className='gap-2 sm:gap-0'>
          <Button variant='outline' onClick={() => handleClose(false)} disabled={loading || precheckLoading}>
            {t('common.cancel')}
          </Button>
          <Button
            variant='secondary'
            onClick={handlePrecheck}
            disabled={loading || precheckLoading || !hostId || !hazelcastPort}
          >
            {precheckLoading && <Loader2 className='h-4 w-4 mr-2 animate-spin' />}
            {t('cluster.precheck')}
          </Button>
          <Button
            onClick={handleSubmit}
            disabled={loading || precheckLoading || !hostId || !hazelcastPort || availableHosts.length === 0}
          >
            {loading && <Loader2 className='h-4 w-4 mr-2 animate-spin' />}
            {t('cluster.addNode')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
