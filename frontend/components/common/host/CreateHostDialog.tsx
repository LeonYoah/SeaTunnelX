'use client';

/**
 * Create Host Dialog Component
 * 创建主机对话框组件
 *
 * Dialog for creating a new host with support for different host types.
 * 用于创建新主机的对话框，支持不同的主机类型。
 */

import {useState} from 'react';
import {useTranslations} from 'next-intl';
import {Button} from '@/components/ui/button';
import {Input} from '@/components/ui/input';
import {Label} from '@/components/ui/label';
import {Textarea} from '@/components/ui/textarea';
import {Switch} from '@/components/ui/switch';
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from '@/components/ui/dialog';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import {toast} from 'sonner';
import services from '@/lib/services';
import {HostType, CreateHostRequest} from '@/lib/services/host/types';

interface CreateHostDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onSuccess: () => void;
}

/**
 * Create Host Dialog Component
 * 创建主机对话框组件
 */
export function CreateHostDialog({
  open,
  onOpenChange,
  onSuccess,
}: CreateHostDialogProps) {
  const t = useTranslations();
  const [loading, setLoading] = useState(false);
  const [formData, setFormData] = useState<CreateHostRequest>({
    name: '',
    host_type: HostType.BARE_METAL,
    description: '',
    ip_address: '',
    ssh_port: 22,
    docker_api_url: '',
    docker_tls_enabled: false,
    docker_cert_path: '',
    k8s_api_url: '',
    k8s_namespace: 'default',
    k8s_kubeconfig: '',
    k8s_token: '',
  });


  /**
   * Reset form data
   * 重置表单数据
   */
  const resetForm = () => {
    setFormData({
      name: '',
      host_type: HostType.BARE_METAL,
      description: '',
      ip_address: '',
      ssh_port: 22,
      docker_api_url: '',
      docker_tls_enabled: false,
      docker_cert_path: '',
      k8s_api_url: '',
      k8s_namespace: 'default',
      k8s_kubeconfig: '',
      k8s_token: '',
    });
  };

  /**
   * Handle dialog close
   * 处理对话框关闭
   */
  const handleClose = () => {
    resetForm();
    onOpenChange(false);
  };

  /**
   * Validate form data
   * 验证表单数据
   */
  const validateForm = (): boolean => {
    if (!formData.name || formData.name.trim().length === 0) {
      toast.error(t('host.errors.nameRequired'));
      return false;
    }

    if (formData.host_type === HostType.BARE_METAL) {
      if (!formData.ip_address || formData.ip_address.trim().length === 0) {
        toast.error(t('host.errors.ipRequired'));
        return false;
      }
      // Basic IP validation / 基本 IP 验证
      const ipRegex = /^(\d{1,3}\.){3}\d{1,3}$|^([a-fA-F0-9:]+)$/;
      if (!ipRegex.test(formData.ip_address)) {
        toast.error(t('host.errors.ipInvalid'));
        return false;
      }
    }

    if (formData.host_type === HostType.DOCKER) {
      if (!formData.docker_api_url || formData.docker_api_url.trim().length === 0) {
        toast.error(t('host.errors.dockerUrlRequired'));
        return false;
      }
      // Validate Docker API URL format / 验证 Docker API URL 格式
      if (
        !formData.docker_api_url.startsWith('tcp://') &&
        !formData.docker_api_url.startsWith('unix://')
      ) {
        toast.error(t('host.errors.dockerUrlInvalid'));
        return false;
      }
    }

    if (formData.host_type === HostType.KUBERNETES) {
      if (!formData.k8s_api_url || formData.k8s_api_url.trim().length === 0) {
        toast.error(t('host.errors.k8sUrlRequired'));
        return false;
      }
      // Validate K8s API URL format / 验证 K8s API URL 格式
      if (
        !formData.k8s_api_url.startsWith('https://') &&
        !formData.k8s_api_url.startsWith('http://')
      ) {
        toast.error(t('host.errors.k8sUrlInvalid'));
        return false;
      }
      // K8s requires either kubeconfig or token / K8s 需要 kubeconfig 或 token
      if (!formData.k8s_kubeconfig && !formData.k8s_token) {
        toast.error(t('host.errors.k8sCredentialsRequired'));
        return false;
      }
    }

    return true;
  };

  /**
   * Handle form submit
   * 处理表单提交
   */
  const handleSubmit = async () => {
    if (!validateForm()) return;

    setLoading(true);
    try {
      const result = await services.host.createHostSafe(formData);
      if (result.success) {
        resetForm();
        onSuccess();
      } else {
        toast.error(result.error || t('host.createError'));
      }
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('host.createError'));
    } finally {
      setLoading(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={handleClose}>
      <DialogContent className='max-w-[500px] max-h-[90vh] overflow-y-auto'>
        <DialogHeader>
          <DialogTitle>{t('host.createHost')}</DialogTitle>
        </DialogHeader>

        <div className='space-y-4 py-4'>
          {/* Host Name / 主机名称 */}
          <div className='space-y-2'>
            <Label htmlFor='name'>{t('host.name')} *</Label>
            <Input
              id='name'
              value={formData.name}
              onChange={(e) => setFormData({...formData, name: e.target.value})}
              placeholder={t('host.namePlaceholder')}
            />
          </div>

          {/* Host Type / 主机类型 */}
          <div className='space-y-2'>
            <Label htmlFor='host_type'>{t('host.hostType')} *</Label>
            <Select
              value={formData.host_type}
              onValueChange={(value) =>
                setFormData({...formData, host_type: value as HostType})
              }
            >
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
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
          </div>

          {/* Description / 描述 */}
          <div className='space-y-2'>
            <Label htmlFor='description'>{t('host.descriptionLabel')}</Label>
            <Textarea
              id='description'
              value={formData.description}
              onChange={(e) =>
                setFormData({...formData, description: e.target.value})
              }
              placeholder={t('host.descriptionPlaceholder')}
              rows={2}
            />
          </div>

          {/* Bare Metal Fields / 物理机字段 */}
          {formData.host_type === HostType.BARE_METAL && (
            <>
              <div className='space-y-2'>
                <Label htmlFor='ip_address'>{t('host.ipAddress')} *</Label>
                <Input
                  id='ip_address'
                  value={formData.ip_address}
                  onChange={(e) =>
                    setFormData({...formData, ip_address: e.target.value})
                  }
                  placeholder='192.168.1.100'
                />
              </div>
              <div className='space-y-2'>
                <Label htmlFor='ssh_port'>{t('host.sshPort')}</Label>
                <Input
                  id='ssh_port'
                  type='number'
                  value={formData.ssh_port}
                  onChange={(e) =>
                    setFormData({...formData, ssh_port: parseInt(e.target.value) || 22})
                  }
                  placeholder='22'
                />
              </div>
            </>
          )}

          {/* Docker Fields / Docker 字段 */}
          {formData.host_type === HostType.DOCKER && (
            <>
              <div className='space-y-2'>
                <Label htmlFor='docker_api_url'>{t('host.dockerApiUrl')} *</Label>
                <Input
                  id='docker_api_url'
                  value={formData.docker_api_url}
                  onChange={(e) =>
                    setFormData({...formData, docker_api_url: e.target.value})
                  }
                  placeholder='tcp://192.168.1.100:2375'
                />
                <p className='text-xs text-muted-foreground'>
                  {t('host.dockerApiUrlHint')}
                </p>
              </div>
              <div className='flex items-center space-x-2'>
                <Switch
                  id='docker_tls_enabled'
                  checked={formData.docker_tls_enabled}
                  onCheckedChange={(checked) =>
                    setFormData({...formData, docker_tls_enabled: checked})
                  }
                />
                <Label htmlFor='docker_tls_enabled'>{t('host.tlsEnabled')}</Label>
              </div>
              {formData.docker_tls_enabled && (
                <div className='space-y-2'>
                  <Label htmlFor='docker_cert_path'>{t('host.certPath')}</Label>
                  <Input
                    id='docker_cert_path'
                    value={formData.docker_cert_path}
                    onChange={(e) =>
                      setFormData({...formData, docker_cert_path: e.target.value})
                    }
                    placeholder='/path/to/certs'
                  />
                </div>
              )}
            </>
          )}

          {/* Kubernetes Fields / Kubernetes 字段 */}
          {formData.host_type === HostType.KUBERNETES && (
            <>
              <div className='space-y-2'>
                <Label htmlFor='k8s_api_url'>{t('host.k8sApiUrl')} *</Label>
                <Input
                  id='k8s_api_url'
                  value={formData.k8s_api_url}
                  onChange={(e) =>
                    setFormData({...formData, k8s_api_url: e.target.value})
                  }
                  placeholder='https://kubernetes.default.svc:6443'
                />
              </div>
              <div className='space-y-2'>
                <Label htmlFor='k8s_namespace'>{t('host.namespace')}</Label>
                <Input
                  id='k8s_namespace'
                  value={formData.k8s_namespace}
                  onChange={(e) =>
                    setFormData({...formData, k8s_namespace: e.target.value})
                  }
                  placeholder='default'
                />
              </div>
              <div className='space-y-2'>
                <Label htmlFor='k8s_kubeconfig'>{t('host.kubeconfig')}</Label>
                <Textarea
                  id='k8s_kubeconfig'
                  value={formData.k8s_kubeconfig}
                  onChange={(e) =>
                    setFormData({...formData, k8s_kubeconfig: e.target.value})
                  }
                  placeholder={t('host.kubeconfigPlaceholder')}
                  rows={3}
                />
              </div>
              <div className='space-y-2'>
                <Label htmlFor='k8s_token'>{t('host.token')}</Label>
                <Textarea
                  id='k8s_token'
                  value={formData.k8s_token}
                  onChange={(e) =>
                    setFormData({...formData, k8s_token: e.target.value})
                  }
                  placeholder={t('host.tokenPlaceholder')}
                  rows={2}
                />
                <p className='text-xs text-muted-foreground'>
                  {t('host.k8sCredentialsHint')}
                </p>
              </div>
            </>
          )}
        </div>

        <DialogFooter>
          <Button variant='outline' onClick={handleClose} disabled={loading}>
            {t('common.cancel')}
          </Button>
          <Button onClick={handleSubmit} disabled={loading}>
            {loading ? t('common.creating') : t('common.create')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
