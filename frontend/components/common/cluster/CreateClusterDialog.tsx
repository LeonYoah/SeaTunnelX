'use client';

/**
 * Create Cluster Dialog Component
 * 创建集群对话框组件
 *
 * Dialog for creating a new SeaTunnel cluster.
 * 用于创建新 SeaTunnel 集群的对话框。
 */

import {useState} from 'react';
import {useTranslations} from 'next-intl';
import {Button} from '@/components/ui/button';
import {Input} from '@/components/ui/input';
import {Label} from '@/components/ui/label';
import {Textarea} from '@/components/ui/textarea';
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
import {Loader2} from 'lucide-react';
import {toast} from 'sonner';
import services from '@/lib/services';
import {DeploymentMode, CreateClusterRequest} from '@/lib/services/cluster/types';

interface CreateClusterDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onSuccess: () => void;
}

/**
 * Create Cluster Dialog Component
 * 创建集群对话框组件
 */
export function CreateClusterDialog({
  open,
  onOpenChange,
  onSuccess,
}: CreateClusterDialogProps) {
  const t = useTranslations();
  const [loading, setLoading] = useState(false);

  // Form state / 表单状态
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [deploymentMode, setDeploymentMode] = useState<DeploymentMode>(DeploymentMode.HYBRID);
  const [version, setVersion] = useState('');

  /**
   * Reset form
   * 重置表单
   */
  const resetForm = () => {
    setName('');
    setDescription('');
    setDeploymentMode(DeploymentMode.HYBRID);
    setVersion('');
  };

  /**
   * Handle submit
   * 处理提交
   */
  const handleSubmit = async () => {
    // Validate required fields / 验证必填字段
    if (!name.trim()) {
      toast.error(t('cluster.nameRequired'));
      return;
    }

    setLoading(true);
    try {
      const data: CreateClusterRequest = {
        name: name.trim(),
        description: description.trim() || undefined,
        deployment_mode: deploymentMode,
        version: version.trim() || undefined,
      };

      const result = await services.cluster.createClusterSafe(data);

      if (result.success) {
        toast.success(t('cluster.createSuccess'));
        resetForm();
        onSuccess();
      } else {
        toast.error(result.error || t('cluster.createError'));
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
          <DialogTitle>{t('cluster.createCluster')}</DialogTitle>
          <DialogDescription>{t('cluster.createClusterDescription')}</DialogDescription>
        </DialogHeader>

        <div className='space-y-4 py-4'>
          <div className='space-y-2'>
            <Label htmlFor='name'>
              {t('cluster.name')} <span className='text-destructive'>*</span>
            </Label>
            <Input
              id='name'
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder={t('cluster.namePlaceholder')}
            />
          </div>

          <div className='space-y-2'>
            <Label htmlFor='description'>{t('cluster.descriptionLabel')}</Label>
            <Textarea
              id='description'
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder={t('cluster.descriptionPlaceholder')}
              rows={2}
            />
          </div>

          <div className='space-y-2'>
            <Label htmlFor='deploymentMode'>
              {t('cluster.deploymentMode')} <span className='text-destructive'>*</span>
            </Label>
            <Select
              value={deploymentMode}
              onValueChange={(value) => setDeploymentMode(value as DeploymentMode)}
            >
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value={DeploymentMode.HYBRID}>
                  {t('cluster.modes.hybrid')}
                </SelectItem>
                <SelectItem value={DeploymentMode.SEPARATED}>
                  {t('cluster.modes.separated')}
                </SelectItem>
              </SelectContent>
            </Select>
            <p className='text-xs text-muted-foreground'>
              {deploymentMode === DeploymentMode.HYBRID
                ? t('cluster.hybridDescription')
                : t('cluster.separatedDescription')}
            </p>
          </div>

          <div className='space-y-2'>
            <Label htmlFor='version'>{t('cluster.version')}</Label>
            <Input
              id='version'
              value={version}
              onChange={(e) => setVersion(e.target.value)}
              placeholder={t('cluster.versionPlaceholder')}
            />
          </div>
        </div>

        <DialogFooter>
          <Button variant='outline' onClick={() => handleClose(false)} disabled={loading}>
            {t('common.cancel')}
          </Button>
          <Button onClick={handleSubmit} disabled={loading}>
            {loading && <Loader2 className='h-4 w-4 mr-2 animate-spin' />}
            {t('common.create')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
