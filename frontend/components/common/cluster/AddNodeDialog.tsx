'use client';

/**
 * Add Node Dialog Component
 * 添加节点对话框组件
 *
 * Dialog for adding a node to a cluster with installation directory.
 * 用于向集群添加节点的对话框，包含安装目录配置。
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
import {Loader2, Server} from 'lucide-react';
import {toast} from 'sonner';
import services from '@/lib/services';
import {NodeRole, AddNodeRequest} from '@/lib/services/cluster/types';
import {HostInfo, AgentStatus} from '@/lib/services/host/types';

interface AddNodeDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  clusterId: number;
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
  onSuccess,
}: AddNodeDialogProps) {
  const t = useTranslations();
  const [loading, setLoading] = useState(false);
  const [loadingHosts, setLoadingHosts] = useState(false);

  // Form state / 表单状态
  const [hostId, setHostId] = useState<string>('');
  const [role, setRole] = useState<NodeRole>(NodeRole.WORKER);
  const [installDir, setInstallDir] = useState('/opt/seatunnel');

  // Available hosts / 可用主机
  const [availableHosts, setAvailableHosts] = useState<HostInfo[]>([]);

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
      <DialogContent className='sm:max-w-[500px]'>
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
              <Select value={hostId} onValueChange={setHostId}>
                <SelectTrigger>
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
            )}
            <p className='text-xs text-muted-foreground'>
              {t('cluster.onlyAgentInstalledHosts')}
            </p>
          </div>

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
        </div>

        <DialogFooter>
          <Button variant='outline' onClick={() => handleClose(false)} disabled={loading}>
            {t('common.cancel')}
          </Button>
          <Button
            onClick={handleSubmit}
            disabled={loading || !hostId || availableHosts.length === 0}
          >
            {loading && <Loader2 className='h-4 w-4 mr-2 animate-spin' />}
            {t('cluster.addNode')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
