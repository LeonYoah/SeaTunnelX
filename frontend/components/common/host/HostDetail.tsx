'use client';

/**
 * Host Detail Component
 * 主机详情组件
 *
 * Displays detailed information about a host including basic info,
 * Agent status, resource usage, and install command.
 * 显示主机详细信息，包括基本信息、Agent 状态、资源使用率和安装命令。
 */

import {useState, useEffect} from 'react';
import {useTranslations} from 'next-intl';
import {Button} from '@/components/ui/button';
import {Badge} from '@/components/ui/badge';
import {Progress} from '@/components/ui/progress';
import {Separator} from '@/components/ui/separator';
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetDescription,
} from '@/components/ui/sheet';
import {toast} from 'sonner';
import {
  Copy,
  Pencil,
  Server,
  Container,
  Cloud,
  Cpu,
  HardDrive,
  MemoryStick,
  Clock,
  Terminal,
  Download,
} from 'lucide-react';
import services from '@/lib/services';
import {HostInfo, HostType, HostStatus, AgentStatus} from '@/lib/services/host/types';
import {InstallationProgress} from '@/components/common/installer';
import {useInstallation} from '@/hooks/use-installer';

interface HostDetailProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  host: HostInfo;
  onEdit: () => void;
  /** Callback to open install wizard / 打开安装向导的回调 */
  onInstall?: () => void;
}

/**
 * Format bytes to human readable string
 * 格式化字节为人类可读字符串
 */
function formatBytes(bytes: number | undefined): string {
  if (!bytes) {
    return '-';
  }
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  let unitIndex = 0;
  let value = bytes;
  while (value >= 1024 && unitIndex < units.length - 1) {
    value /= 1024;
    unitIndex++;
  }
  return `${value.toFixed(1)} ${units[unitIndex]}`;
}


/**
 * Format date time
 * 格式化日期时间
 */
function formatDateTime(dateStr: string | null | undefined): string {
  if (!dateStr) {
    return '-';
  }
  return new Date(dateStr).toLocaleString();
}

/**
 * Get host type icon
 * 获取主机类型图标
 */
function getHostTypeIcon(hostType: HostType) {
  switch (hostType) {
    case HostType.BARE_METAL:
      return <Server className='h-5 w-5' />;
    case HostType.DOCKER:
      return <Container className='h-5 w-5' />;
    case HostType.KUBERNETES:
      return <Cloud className='h-5 w-5' />;
    default:
      return <Server className='h-5 w-5' />;
  }
}

/**
 * Get status badge variant
 * 获取状态徽章变体
 */
function getStatusBadgeVariant(
  status: HostStatus,
): 'default' | 'secondary' | 'destructive' | 'outline' {
  switch (status) {
    case HostStatus.CONNECTED:
      return 'default';
    case HostStatus.PENDING:
      return 'secondary';
    case HostStatus.OFFLINE:
      return 'outline';
    case HostStatus.ERROR:
      return 'destructive';
    default:
      return 'secondary';
  }
}

/**
 * Host Detail Component
 * 主机详情组件
 */
export function HostDetail({open, onOpenChange, host, onEdit, onInstall}: HostDetailProps) {
  const t = useTranslations();
  const [installCommand, setInstallCommand] = useState<string>('');
  const [loadingCommand, setLoadingCommand] = useState(false);

  // Check for ongoing installation / 检查是否有正在进行的安装
  const { status: installationStatus, refresh: refreshInstallation } = useInstallation(host.id);

  // Determine if we can show install button / 判断是否可以显示安装按钮
  // Agent must be online and no installation in progress
  // Agent 必须在线且没有正在进行的安装
  const canInstallSeaTunnel =
    host.host_type === HostType.BARE_METAL &&
    host.agent_status === AgentStatus.INSTALLED &&
    !installationStatus &&
    !host.seatunnel_installed;

  // Check if installation is in progress / 检查安装是否正在进行
  const isInstalling = installationStatus?.status === 'running';

  /**
   * Load install command for bare_metal hosts
   * 加载物理机的安装命令
   */
  const loadInstallCommand = async () => {
    setLoadingCommand(true);
    try {
      const result = await services.host.getInstallCommandSafe(host.id);
      if (result.success && result.data) {
        setInstallCommand(result.data.command);
      }
    } catch (err) {
      console.error('Failed to load install command:', err);
    } finally {
      setLoadingCommand(false);
    }
  };

  useEffect(() => {
    if (open && host.host_type === HostType.BARE_METAL) {
      loadInstallCommand();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, host.id, host.host_type]);

  /**
   * Copy install command to clipboard
   * 复制安装命令到剪贴板
   */
  const handleCopyCommand = async () => {
    try {
      await navigator.clipboard.writeText(installCommand);
      toast.success(t('host.commandCopied'));
    } catch {
      toast.error(t('host.copyFailed'));
    }
  };

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className='w-[500px] sm:w-[600px] overflow-y-auto'>
        <SheetHeader>
          <SheetTitle className='flex items-center gap-2'>
            {getHostTypeIcon(host.host_type)}
            {host.name}
          </SheetTitle>
          <SheetDescription>{host.description || t('host.noDescription')}</SheetDescription>
        </SheetHeader>

        <div className='mt-6 space-y-6'>
          {/* Basic Info / 基本信息 */}
          <div>
            <h3 className='text-sm font-medium mb-3'>{t('host.basicInfo')}</h3>
            <div className='grid grid-cols-2 gap-4 text-sm'>
              <div>
                <span className='text-muted-foreground'>{t('host.hostType')}:</span>
                <Badge variant='outline' className='ml-2'>
                  {t(`host.types.${host.host_type === HostType.BARE_METAL ? 'bareMetal' : host.host_type}`)}
                </Badge>
              </div>
              <div>
                <span className='text-muted-foreground'>{t('host.status')}:</span>
                <Badge variant={getStatusBadgeVariant(host.status)} className='ml-2'>
                  {t(`host.statuses.${host.status}`)}
                </Badge>
              </div>
              <div>
                <span className='text-muted-foreground'>{t('host.createdAt')}:</span>
                <span className='ml-2'>{formatDateTime(host.created_at)}</span>
              </div>
              <div>
                <span className='text-muted-foreground'>{t('host.updatedAt')}:</span>
                <span className='ml-2'>{formatDateTime(host.updated_at)}</span>
              </div>
            </div>
          </div>

          <Separator />

          {/* Connection Info / 连接信息 */}
          <div>
            <h3 className='text-sm font-medium mb-3'>{t('host.connectionInfo')}</h3>
            {host.host_type === HostType.BARE_METAL && (
              <div className='space-y-3 text-sm'>
                <div className='flex justify-between'>
                  <span className='text-muted-foreground'>{t('host.ipAddress')}:</span>
                  <span>{host.ip_address || '-'}</span>
                </div>
                <div className='flex justify-between'>
                  <span className='text-muted-foreground'>{t('host.sshPort')}:</span>
                  <span>{host.ssh_port || 22}</span>
                </div>
                <div className='flex justify-between'>
                  <span className='text-muted-foreground'>{t('host.agentStatus')}:</span>
                  <Badge
                    variant={
                      host.agent_status === AgentStatus.INSTALLED
                        ? 'default'
                        : host.agent_status === AgentStatus.OFFLINE
                          ? 'outline'
                          : 'secondary'
                    }
                  >
                    {t(`host.agentStatuses.${host.agent_status === AgentStatus.NOT_INSTALLED ? 'notInstalled' : host.agent_status}`)}
                  </Badge>
                </div>
                {host.agent_version && (
                  <div className='flex justify-between'>
                    <span className='text-muted-foreground'>{t('host.agentVersion')}:</span>
                    <span>{host.agent_version}</span>
                  </div>
                )}
                {host.os_type && (
                  <div className='flex justify-between'>
                    <span className='text-muted-foreground'>{t('host.osType')}:</span>
                    <span>{host.os_type} ({host.arch})</span>
                  </div>
                )}
                {host.last_heartbeat && (
                  <div className='flex justify-between'>
                    <span className='text-muted-foreground'>{t('host.lastHeartbeat')}:</span>
                    <span>{formatDateTime(host.last_heartbeat)}</span>
                  </div>
                )}
              </div>
            )}
            {host.host_type === HostType.DOCKER && (
              <div className='space-y-3 text-sm'>
                <div className='flex justify-between'>
                  <span className='text-muted-foreground'>{t('host.dockerApiUrl')}:</span>
                  <span className='truncate max-w-[250px]'>{host.docker_api_url || '-'}</span>
                </div>
                <div className='flex justify-between'>
                  <span className='text-muted-foreground'>{t('host.tlsEnabled')}:</span>
                  <span>{host.docker_tls_enabled ? t('common.yes') : t('common.no')}</span>
                </div>
                {host.docker_version && (
                  <div className='flex justify-between'>
                    <span className='text-muted-foreground'>{t('host.dockerVersion')}:</span>
                    <span>{host.docker_version}</span>
                  </div>
                )}
              </div>
            )}
            {host.host_type === HostType.KUBERNETES && (
              <div className='space-y-3 text-sm'>
                <div className='flex justify-between'>
                  <span className='text-muted-foreground'>{t('host.k8sApiUrl')}:</span>
                  <span className='truncate max-w-[250px]'>{host.k8s_api_url || '-'}</span>
                </div>
                <div className='flex justify-between'>
                  <span className='text-muted-foreground'>{t('host.namespace')}:</span>
                  <span>{host.k8s_namespace || 'default'}</span>
                </div>
                {host.k8s_version && (
                  <div className='flex justify-between'>
                    <span className='text-muted-foreground'>{t('host.k8sVersion')}:</span>
                    <span>{host.k8s_version}</span>
                  </div>
                )}
              </div>
            )}
          </div>

          <Separator />

          {/* Resource Usage / 资源使用率 */}
          <div>
            <h3 className='text-sm font-medium mb-3'>{t('host.resources')}</h3>
            <div className='space-y-4'>
              <div>
                <div className='flex items-center justify-between mb-1'>
                  <div className='flex items-center gap-2'>
                    <Cpu className='h-4 w-4 text-muted-foreground' />
                    <span className='text-sm'>CPU</span>
                  </div>
                  <span className='text-sm'>{host.cpu_usage?.toFixed(1) || 0}%</span>
                </div>
                <Progress value={host.cpu_usage || 0} className='h-2' />
                {host.cpu_cores && (
                  <span className='text-xs text-muted-foreground'>
                    {host.cpu_cores} {t('host.cores')}
                  </span>
                )}
              </div>

              <div>
                <div className='flex items-center justify-between mb-1'>
                  <div className='flex items-center gap-2'>
                    <MemoryStick className='h-4 w-4 text-muted-foreground' />
                    <span className='text-sm'>{t('host.memory')}</span>
                  </div>
                  <span className='text-sm'>{host.memory_usage?.toFixed(1) || 0}%</span>
                </div>
                <Progress value={host.memory_usage || 0} className='h-2' />
                {host.total_memory && (
                  <span className='text-xs text-muted-foreground'>
                    {formatBytes(host.total_memory)} {t('host.total')}
                  </span>
                )}
              </div>

              <div>
                <div className='flex items-center justify-between mb-1'>
                  <div className='flex items-center gap-2'>
                    <HardDrive className='h-4 w-4 text-muted-foreground' />
                    <span className='text-sm'>{t('host.disk')}</span>
                  </div>
                  <span className='text-sm'>{host.disk_usage?.toFixed(1) || 0}%</span>
                </div>
                <Progress value={host.disk_usage || 0} className='h-2' />
                {host.total_disk && (
                  <span className='text-xs text-muted-foreground'>
                    {formatBytes(host.total_disk)} {t('host.total')}
                  </span>
                )}
              </div>

              {host.last_check && (
                <div className='flex items-center gap-2 text-xs text-muted-foreground'>
                  <Clock className='h-3 w-3' />
                  {t('host.lastCheck')}: {formatDateTime(host.last_check)}
                </div>
              )}
            </div>
          </div>

          {/* SeaTunnel Installation Section / SeaTunnel 安装部分 */}
          {host.host_type === HostType.BARE_METAL && (
            <>
              <Separator />
              <div>
                <h3 className='text-sm font-medium mb-3 flex items-center gap-2'>
                  <Download className='h-4 w-4' />
                  {t('installer.seaTunnelInstallation')}
                </h3>

                {/* Installation in progress / 安装进行中 */}
                {installationStatus && (
                  <InstallationProgress
                    hostId={host.id}
                    compact
                    onComplete={() => {
                      toast.success(t('installer.installationComplete'));
                      refreshInstallation();
                    }}
                    onFailed={() => {
                      toast.error(t('installer.installationFailed'));
                    }}
                  />
                )}

                {/* Install button / 安装按钮 */}
                {canInstallSeaTunnel && onInstall && (
                  <div className='space-y-2'>
                    <p className='text-sm text-muted-foreground'>
                      {t('installer.seaTunnelNotInstalled')}
                    </p>
                    <Button onClick={onInstall} size='sm'>
                      <Download className='h-4 w-4 mr-2' />
                      {t('installer.installSeaTunnel')}
                    </Button>
                  </div>
                )}

                {/* Already installed / 已安装 */}
                {host.seatunnel_installed && !installationStatus && (
                  <div className='flex items-center gap-2'>
                    <Badge variant='default'>{t('installer.installed')}</Badge>
                    {host.seatunnel_version && (
                      <span className='text-sm text-muted-foreground'>
                        v{host.seatunnel_version}
                      </span>
                    )}
                  </div>
                )}

                {/* Agent not ready / Agent 未就绪 */}
                {host.agent_status !== AgentStatus.INSTALLED && !installationStatus && (
                  <p className='text-sm text-muted-foreground'>
                    {t('installer.agentRequiredForInstall')}
                  </p>
                )}
              </div>
            </>
          )}

          {/* Install & Uninstall Commands (for bare_metal) / 安装和卸载命令（物理机） */}
          {host.host_type === HostType.BARE_METAL && (
            <>
              <Separator />
              {/* Install Command / 安装命令 */}
              <div>
                <h3 className='text-sm font-medium mb-3 flex items-center gap-2'>
                  <Terminal className='h-4 w-4' />
                  {t('host.installCommand')}
                </h3>
                {loadingCommand ? (
                  <div className='text-sm text-muted-foreground'>{t('common.loading')}</div>
                ) : installCommand ? (
                  <div className='relative'>
                    <pre className='bg-muted p-3 rounded-md text-xs overflow-x-auto'>
                      {installCommand}
                    </pre>
                    <Button
                      variant='ghost'
                      size='icon'
                      className='absolute top-2 right-2'
                      onClick={handleCopyCommand}
                    >
                      <Copy className='h-4 w-4' />
                    </Button>
                  </div>
                ) : (
                  <div className='text-sm text-muted-foreground'>
                    {t('host.noInstallCommand')}
                  </div>
                )}
              </div>

              {/* Uninstall Command / 卸载命令 */}
              <div>
                <h3 className='text-sm font-medium mb-3 flex items-center gap-2'>
                  <Terminal className='h-4 w-4' />
                  {t('host.uninstallCommand')}
                </h3>
                {installCommand ? (
                  <div className='relative'>
                    <pre className='bg-muted p-3 rounded-md text-xs overflow-x-auto'>
                      {installCommand.replace('/install.sh', '/uninstall.sh')}
                    </pre>
                    <Button
                      variant='ghost'
                      size='icon'
                      className='absolute top-2 right-2'
                      onClick={() => {
                        navigator.clipboard.writeText(installCommand.replace('/install.sh', '/uninstall.sh'));
                        toast.success(t('host.commandCopied'));
                      }}
                    >
                      <Copy className='h-4 w-4' />
                    </Button>
                  </div>
                ) : (
                  <div className='text-sm text-muted-foreground'>
                    {t('host.noInstallCommand')}
                  </div>
                )}
                <p className='text-xs text-muted-foreground mt-2'>
                  {t('host.uninstallCommandTip')}
                </p>
              </div>
            </>
          )}

          {/* Actions / 操作 */}
          <div className='flex justify-end gap-2 pt-4'>
            <Button variant='outline' onClick={() => onOpenChange(false)}>
              {t('common.close')}
            </Button>
            <Button onClick={onEdit}>
              <Pencil className='h-4 w-4 mr-2' />
              {t('common.edit')}
            </Button>
          </div>
        </div>
      </SheetContent>
    </Sheet>
  );
}
