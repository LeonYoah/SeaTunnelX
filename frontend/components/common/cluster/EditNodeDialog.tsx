'use client';

/**
 * Edit Node Dialog Component
 * 编辑节点对话框组件
 *
 * Dialog for editing an existing node's configuration.
 * 用于编辑现有节点配置的对话框。
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
import {Loader2} from 'lucide-react';
import {toast} from 'sonner';
import services from '@/lib/services';
import {NodeInfo, NodeRole, DeploymentMode, DefaultPorts} from '@/lib/services/cluster/types';

interface EditNodeDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  node: NodeInfo | null;
  deploymentMode: DeploymentMode;
  onSuccess: () => void;
}

/**
 * Edit Node Dialog Component
 * 编辑节点对话框组件
 */
export function EditNodeDialog({
  open,
  onOpenChange,
  node,
  deploymentMode,
  onSuccess,
}: EditNodeDialogProps) {
  const t = useTranslations();
  const [loading, setLoading] = useState(false);

  // Form state / 表单状态
  const [installDir, setInstallDir] = useState('');
  const [hazelcastPort, setHazelcastPort] = useState<number>(0);
  const [apiPort, setApiPort] = useState<number>(0);
  const [workerPort, setWorkerPort] = useState<number>(0);

  // Initialize form when node changes / 节点变化时初始化表单
  useEffect(() => {
    if (node) {
      setInstallDir(node.install_dir || '/opt/seatunnel');
      setHazelcastPort(node.hazelcast_port || (node.role === NodeRole.MASTER ? DefaultPorts.MASTER_HAZELCAST : DefaultPorts.WORKER_HAZELCAST));
      setApiPort(node.api_port || 0);
      setWorkerPort(node.worker_port || DefaultPorts.WORKER_HAZELCAST);
    }
  }, [node]);

  /**
   * Handle submit
   * 处理提交
   */
  const handleSubmit = async () => {
    if (!node) return;

    if (!installDir.trim()) {
      toast.error(t('cluster.installDirRequired'));
      return;
    }

    if (!hazelcastPort || hazelcastPort <= 0) {
      toast.error(t('cluster.hazelcastPortRequired'));
      return;
    }

    setLoading(true);
    try {
      const data = {
        install_dir: installDir.trim(),
        hazelcast_port: hazelcastPort,
        api_port: node.role === NodeRole.MASTER && apiPort > 0 ? apiPort : undefined,
        worker_port: deploymentMode === DeploymentMode.HYBRID && node.role === NodeRole.MASTER ? workerPort : undefined,
      };

      const result = await services.cluster.updateNodeSafe(node.cluster_id, node.id, data);

      if (result.success) {
        onSuccess();
        onOpenChange(false);
      } else {
        toast.error(result.error || t('cluster.updateNodeError'));
      }
    } finally {
      setLoading(false);
    }
  };

  if (!node) return null;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='sm:max-w-[500px]'>
        <DialogHeader>
          <DialogTitle>{t('cluster.editNode')}</DialogTitle>
          <DialogDescription>
            {t('cluster.editNodeDescription', {host: node.host_name, ip: node.host_ip})}
          </DialogDescription>
        </DialogHeader>

        <div className='space-y-4 py-4'>
          {/* Node Info (Read-only) / 节点信息（只读） */}
          <div className='grid grid-cols-2 gap-4 p-3 bg-muted rounded-md'>
            <div>
              <Label className='text-xs text-muted-foreground'>{t('cluster.hostName')}</Label>
              <p className='text-sm font-medium'>{node.host_name}</p>
            </div>
            <div>
              <Label className='text-xs text-muted-foreground'>{t('cluster.hostIP')}</Label>
              <p className='text-sm font-medium'>{node.host_ip}</p>
            </div>
            <div>
              <Label className='text-xs text-muted-foreground'>{t('cluster.nodeRole')}</Label>
              <p className='text-sm font-medium'>{t(`cluster.roles.${node.role}`)}</p>
            </div>
            <div>
              <Label className='text-xs text-muted-foreground'>{t('cluster.nodeStatus')}</Label>
              <p className='text-sm font-medium'>{t(`cluster.statuses.${node.status}`)}</p>
            </div>
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
          </div>

          {/* Port Configuration / 端口配置 */}
          <div className='space-y-3'>
            <Label>{t('cluster.portConfig')}</Label>
            
            <div className='grid grid-cols-2 gap-4'>
              {/* Hazelcast Port / Hazelcast 端口 */}
              <div className='space-y-1'>
                <Label htmlFor='hazelcastPort' className='text-xs'>
                  {t('cluster.hazelcastPort')} <span className='text-destructive'>*</span>
                </Label>
                <Input
                  id='hazelcastPort'
                  type='number'
                  value={hazelcastPort}
                  onChange={(e) => setHazelcastPort(parseInt(e.target.value, 10) || 0)}
                  placeholder={node.role === NodeRole.MASTER ? '5801' : '5802'}
                  required
                />
              </div>

              {/* API Port (Master only, optional) / API 端口（仅 Master，可选） */}
              {node.role === NodeRole.MASTER && (
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

              {/* Worker Port (Hybrid mode Master only) / Worker 端口（仅混合模式 Master） */}
              {deploymentMode === DeploymentMode.HYBRID && node.role === NodeRole.MASTER && (
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
        </div>

        <DialogFooter>
          <Button variant='outline' onClick={() => onOpenChange(false)} disabled={loading}>
            {t('common.cancel')}
          </Button>
          <Button onClick={handleSubmit} disabled={loading}>
            {loading && <Loader2 className='h-4 w-4 mr-2 animate-spin' />}
            {t('common.save')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
