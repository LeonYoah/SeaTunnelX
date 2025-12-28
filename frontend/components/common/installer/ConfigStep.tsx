/**
 * Configuration Step Component
 * 配置步骤组件
 *
 * Allows users to configure installation options.
 * 允许用户配置安装选项。
 */

'use client';

import { useTranslations } from 'next-intl';
import { usePackages } from '@/hooks/use-installer';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Label } from '@/components/ui/label';
import { Input } from '@/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Separator } from '@/components/ui/separator';
import { ScrollArea } from '@/components/ui/scroll-area';
import { Badge } from '@/components/ui/badge';
import { cn } from '@/lib/utils';
import {
  Cloud,
  HardDrive,
  Server,
  Layers,
  Settings,
  Database,
  Loader2,
} from 'lucide-react';
import type {
  InstallMode,
  MirrorSource,
  DeploymentMode,
  NodeRole,
  JVMConfig,
  CheckpointConfig,
  CheckpointStorageType,
  ConnectorConfig,
} from '@/lib/services/installer/types';

interface InstallWizardConfig {
  version: string;
  installMode: InstallMode;
  mirror: MirrorSource;
  packagePath: string;
  deploymentMode: DeploymentMode;
  nodeRole: NodeRole;
  clusterId: string;
  jvm: JVMConfig;
  checkpoint: CheckpointConfig;
  connector: ConnectorConfig;
}

interface ConfigStepProps {
  /** Current configuration / 当前配置 */
  config: InstallWizardConfig;
  /** Callback when configuration changes / 配置变化时的回调 */
  onConfigChange: (updates: Partial<InstallWizardConfig>) => void;
}

export function ConfigStep({ config, onConfigChange }: ConfigStepProps) {
  const t = useTranslations();
  const { packages, loading: packagesLoading } = usePackages();

  // Get available versions / 获取可用版本
  const availableVersions = packages?.versions || [];
  const localPackages = packages?.local_packages || [];
  const recommendedVersion = packages?.recommended_version;

  // Handle JVM config change / 处理 JVM 配置变化
  const handleJvmChange = (key: keyof JVMConfig, value: number) => {
    onConfigChange({
      jvm: { ...config.jvm, [key]: value },
    });
  };

  // Handle checkpoint config change / 处理检查点配置变化
  const handleCheckpointChange = (key: keyof CheckpointConfig, value: string | number) => {
    onConfigChange({
      checkpoint: { ...config.checkpoint, [key]: value },
    });
  };

  return (
    <ScrollArea className="h-[500px] pr-4">
      <div className="space-y-6">
        {/* Install Mode / 安装模式 */}
        <Card>
          <CardHeader className="pb-3">
            <CardTitle className="text-base flex items-center gap-2">
              <Cloud className="h-4 w-4" />
              {t('installer.installMode')}
            </CardTitle>
            <CardDescription>
              {t('installer.installModeDesc') || 'Choose how to install SeaTunnel'}
            </CardDescription>
          </CardHeader>
          <CardContent>
            <div className="grid grid-cols-2 gap-4">
              {/* Online mode / 在线模式 */}
              <div
                className={cn(
                  'relative flex flex-col p-4 rounded-lg border-2 cursor-pointer transition-colors',
                  config.installMode === 'online'
                    ? 'border-primary bg-primary/5'
                    : 'border-muted hover:border-muted-foreground/50'
                )}
                onClick={() => onConfigChange({ installMode: 'online' })}
              >
                <div className="flex items-center gap-2 mb-2">
                  <Cloud className="h-5 w-5 text-primary" />
                  <span className="font-medium">{t('installer.online')}</span>
                </div>
                <p className="text-sm text-muted-foreground">
                  {t('installer.onlineDesc')}
                </p>
              </div>

              {/* Offline mode / 离线模式 */}
              <div
                className={cn(
                  'relative flex flex-col p-4 rounded-lg border-2 cursor-pointer transition-colors',
                  config.installMode === 'offline'
                    ? 'border-primary bg-primary/5'
                    : 'border-muted hover:border-muted-foreground/50'
                )}
                onClick={() => onConfigChange({ installMode: 'offline' })}
              >
                <div className="flex items-center gap-2 mb-2">
                  <HardDrive className="h-5 w-5 text-primary" />
                  <span className="font-medium">{t('installer.offline')}</span>
                </div>
                <p className="text-sm text-muted-foreground">
                  {t('installer.offlineDesc')}
                </p>
              </div>
            </div>

            {/* Online mode options / 在线模式选项 */}
            {config.installMode === 'online' && (
              <div className="mt-4 space-y-4">
                {/* Mirror source / 镜像源 */}
                <div className="space-y-2">
                  <Label>{t('installer.mirrorSource')}</Label>
                  <Select
                    value={config.mirror}
                    onValueChange={(value: MirrorSource) => onConfigChange({ mirror: value })}
                  >
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="aliyun">{t('installer.mirrors.aliyun')}</SelectItem>
                      <SelectItem value="huaweicloud">{t('installer.mirrors.huaweicloud')}</SelectItem>
                      <SelectItem value="apache">{t('installer.mirrors.apache')}</SelectItem>
                    </SelectContent>
                  </Select>
                </div>

                {/* Version selection / 版本选择 */}
                <div className="space-y-2">
                  <Label>{t('installer.version')}</Label>
                  <Select
                    value={config.version}
                    onValueChange={(value) => onConfigChange({ version: value })}
                    disabled={packagesLoading}
                  >
                    <SelectTrigger>
                      {packagesLoading ? (
                        <div className="flex items-center gap-2">
                          <Loader2 className="h-4 w-4 animate-spin" />
                          <span>{t('common.loading')}</span>
                        </div>
                      ) : (
                        <SelectValue placeholder={t('installer.selectVersion') || 'Select version'} />
                      )}
                    </SelectTrigger>
                    <SelectContent>
                      {availableVersions.map((version) => (
                        <SelectItem key={version} value={version}>
                          <div className="flex items-center gap-2">
                            {version}
                            {version === recommendedVersion && (
                              <Badge variant="secondary" className="text-xs">
                                {t('installer.recommended')}
                              </Badge>
                            )}
                          </div>
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
              </div>
            )}

            {/* Offline mode options / 离线模式选项 */}
            {config.installMode === 'offline' && (
              <div className="mt-4 space-y-4">
                {/* Local package selection / 本地安装包选择 */}
                <div className="space-y-2">
                  <Label>{t('installer.selectPackage')}</Label>
                  {localPackages.length > 0 ? (
                    <Select
                      value={config.version}
                      onValueChange={(value) => {
                        const pkg = localPackages.find(p => p.version === value);
                        onConfigChange({
                          version: value,
                          packagePath: pkg?.local_path || '',
                        });
                      }}
                    >
                      <SelectTrigger>
                        <SelectValue placeholder={t('installer.selectPackage')} />
                      </SelectTrigger>
                      <SelectContent>
                        {localPackages.map((pkg) => (
                          <SelectItem key={pkg.version} value={pkg.version}>
                            <div className="flex items-center gap-2">
                              {pkg.version}
                              <span className="text-xs text-muted-foreground">
                                ({(pkg.file_size / 1024 / 1024).toFixed(1)} MB)
                              </span>
                            </div>
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  ) : (
                    <p className="text-sm text-muted-foreground">
                      {t('installer.noLocalPackages')}
                    </p>
                  )}
                </div>
              </div>
            )}
          </CardContent>
        </Card>

        {/* Deployment Mode / 部署模式 */}
        <Card>
          <CardHeader className="pb-3">
            <CardTitle className="text-base flex items-center gap-2">
              <Layers className="h-4 w-4" />
              {t('installer.deploymentMode')}
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="grid grid-cols-2 gap-4">
              {/* Hybrid mode / 混合模式 */}
              <div
                className={cn(
                  'relative flex flex-col p-4 rounded-lg border-2 cursor-pointer transition-colors',
                  config.deploymentMode === 'hybrid'
                    ? 'border-primary bg-primary/5'
                    : 'border-muted hover:border-muted-foreground/50'
                )}
                onClick={() => onConfigChange({ deploymentMode: 'hybrid' })}
              >
                <div className="flex items-center gap-2 mb-2">
                  <Server className="h-5 w-5 text-primary" />
                  <span className="font-medium">{t('installer.hybrid')}</span>
                </div>
                <p className="text-sm text-muted-foreground">
                  {t('installer.hybridDesc')}
                </p>
              </div>

              {/* Separated mode / 分离模式 */}
              <div
                className={cn(
                  'relative flex flex-col p-4 rounded-lg border-2 cursor-pointer transition-colors',
                  config.deploymentMode === 'separated'
                    ? 'border-primary bg-primary/5'
                    : 'border-muted hover:border-muted-foreground/50'
                )}
                onClick={() => onConfigChange({ deploymentMode: 'separated' })}
              >
                <div className="flex items-center gap-2 mb-2">
                  <Layers className="h-5 w-5 text-primary" />
                  <span className="font-medium">{t('installer.separated')}</span>
                </div>
                <p className="text-sm text-muted-foreground">
                  {t('installer.separatedDesc')}
                </p>
              </div>
            </div>

            {/* Node role (for separated mode) / 节点角色（分离模式） */}
            {config.deploymentMode === 'separated' && (
              <div className="mt-4 space-y-2">
                <Label>{t('installer.nodeRole')}</Label>
                <Select
                  value={config.nodeRole}
                  onValueChange={(value: NodeRole) => onConfigChange({ nodeRole: value })}
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="master">{t('installer.master')}</SelectItem>
                    <SelectItem value="worker">{t('installer.worker')}</SelectItem>
                  </SelectContent>
                </Select>
              </div>
            )}
          </CardContent>
        </Card>

        {/* JVM Configuration / JVM 配置 */}
        <Card>
          <CardHeader className="pb-3">
            <CardTitle className="text-base flex items-center gap-2">
              <Settings className="h-4 w-4" />
              {t('installer.jvmConfig')}
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
              {config.deploymentMode === 'hybrid' ? (
                <div className="space-y-2">
                  <Label>{t('installer.hybridHeapSize')} (GB)</Label>
                  <Input
                    type="number"
                    value={config.jvm.hybrid_heap_size}
                    onChange={(e) => handleJvmChange('hybrid_heap_size', parseInt(e.target.value) || 0)}
                    min={1}
                    max={64}
                    step={1}
                  />
                </div>
              ) : (
                <>
                  <div className="space-y-2">
                    <Label>{t('installer.masterHeapSize')} (GB)</Label>
                    <Input
                      type="number"
                      value={config.jvm.master_heap_size}
                      onChange={(e) => handleJvmChange('master_heap_size', parseInt(e.target.value) || 0)}
                      min={1}
                      max={64}
                      step={1}
                    />
                  </div>
                  <div className="space-y-2">
                    <Label>{t('installer.workerHeapSize')} (GB)</Label>
                    <Input
                      type="number"
                      value={config.jvm.worker_heap_size}
                      onChange={(e) => handleJvmChange('worker_heap_size', parseInt(e.target.value) || 0)}
                      min={1}
                      max={64}
                      step={1}
                    />
                  </div>
                </>
              )}
            </div>
          </CardContent>
        </Card>

        {/* Checkpoint Configuration / 检查点配置 */}
        <Card>
          <CardHeader className="pb-3">
            <CardTitle className="text-base flex items-center gap-2">
              <Database className="h-4 w-4" />
              {t('installer.checkpointConfig')}
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            {/* Storage type / 存储类型 */}
            <div className="space-y-2">
              <Label>{t('installer.storageType')}</Label>
              <Select
                value={config.checkpoint.storage_type}
                onValueChange={(value: CheckpointStorageType) =>
                  handleCheckpointChange('storage_type', value)
                }
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="LOCAL_FILE">Local File</SelectItem>
                  <SelectItem value="HDFS">HDFS</SelectItem>
                  <SelectItem value="OSS">Aliyun OSS</SelectItem>
                  <SelectItem value="S3">AWS S3</SelectItem>
                </SelectContent>
              </Select>
            </div>

            {/* Namespace / 命名空间 */}
            <div className="space-y-2">
              <Label>{t('installer.namespace')}</Label>
              <Input
                value={config.checkpoint.namespace}
                onChange={(e) => handleCheckpointChange('namespace', e.target.value)}
                placeholder="/tmp/seatunnel/checkpoint/"
              />
            </div>

            {/* HDFS config / HDFS 配置 */}
            {config.checkpoint.storage_type === 'HDFS' && (
              <div className="grid grid-cols-2 gap-4">
                <div className="space-y-2">
                  <Label>{t('installer.hdfsNameNodeHost')}</Label>
                  <Input
                    value={config.checkpoint.hdfs_namenode_host || ''}
                    onChange={(e) => handleCheckpointChange('hdfs_namenode_host', e.target.value)}
                    placeholder="namenode.example.com"
                  />
                </div>
                <div className="space-y-2">
                  <Label>{t('installer.hdfsNameNodePort')}</Label>
                  <Input
                    type="number"
                    value={config.checkpoint.hdfs_namenode_port || ''}
                    onChange={(e) => handleCheckpointChange('hdfs_namenode_port', parseInt(e.target.value) || 0)}
                    placeholder="8020"
                  />
                </div>
              </div>
            )}

            {/* OSS/S3 config / OSS/S3 配置 */}
            {(config.checkpoint.storage_type === 'OSS' || config.checkpoint.storage_type === 'S3') && (
              <div className="space-y-4">
                <div className="space-y-2">
                  <Label>{t('installer.endpoint')}</Label>
                  <Input
                    value={config.checkpoint.storage_endpoint || ''}
                    onChange={(e) => handleCheckpointChange('storage_endpoint', e.target.value)}
                    placeholder={config.checkpoint.storage_type === 'OSS' ? 'oss-cn-hangzhou.aliyuncs.com' : 's3.amazonaws.com'}
                  />
                </div>
                <div className="grid grid-cols-2 gap-4">
                  <div className="space-y-2">
                    <Label>{t('installer.accessKey')}</Label>
                    <Input
                      type="password"
                      value={config.checkpoint.storage_access_key || ''}
                      onChange={(e) => handleCheckpointChange('storage_access_key', e.target.value)}
                    />
                  </div>
                  <div className="space-y-2">
                    <Label>{t('installer.secretKey')}</Label>
                    <Input
                      type="password"
                      value={config.checkpoint.storage_secret_key || ''}
                      onChange={(e) => handleCheckpointChange('storage_secret_key', e.target.value)}
                    />
                  </div>
                </div>
                <div className="space-y-2">
                  <Label>{t('installer.bucket')}</Label>
                  <Input
                    value={config.checkpoint.storage_bucket || ''}
                    onChange={(e) => handleCheckpointChange('storage_bucket', e.target.value)}
                    placeholder="my-checkpoint-bucket"
                  />
                </div>
              </div>
            )}
          </CardContent>
        </Card>
      </div>
    </ScrollArea>
  );
}

export default ConfigStep;
