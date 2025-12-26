/**
 * Cluster Deploy Wizard Component
 * 集群部署向导组件
 *
 * Multi-step wizard for deploying a new SeaTunnel cluster.
 * Includes host selection, role assignment, installation config, and deployment.
 * 多步骤向导，用于部署新的 SeaTunnel 集群。
 * 包括主机选择、角色分配、安装配置和部署。
 */

'use client';

import { useState, useCallback, useEffect, useMemo } from 'react';
import { useTranslations } from 'next-intl';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';
import { Progress } from '@/components/ui/progress';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Textarea } from '@/components/ui/textarea';
import { Badge } from '@/components/ui/badge';
import { Checkbox } from '@/components/ui/checkbox';
import { ScrollArea } from '@/components/ui/scroll-area';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { cn } from '@/lib/utils';
import {
  CheckCircle2,
  ChevronLeft,
  ChevronRight,
  X,
  Loader2,
  Server,
  Settings,
  Package,
  PlayCircle,
  PartyPopper,
  AlertTriangle,
  Crown,
  Wrench,
  Search,
  Download,
} from 'lucide-react';
import { toast } from 'sonner';
import services from '@/lib/services';
import { usePackages } from '@/hooks/use-installer';
import { PluginService } from '@/lib/services/plugin';
import { HostInfo, HostType, AgentStatus } from '@/lib/services/host/types';
import { DeploymentMode, NodeRole } from '@/lib/services/cluster/types';
import type { LocalPlugin } from '@/lib/services/plugin/types';
import type {
  InstallMode,
  MirrorSource,
  JVMConfig,
  CheckpointConfig,
  CheckpointStorageType,
  PrecheckResult,
  CheckStatus,
} from '@/lib/services/installer/types';

// Wizard step types / 向导步骤类型
export type DeployWizardStep =
  | 'basic'
  | 'hosts'
  | 'config'
  | 'plugins'
  | 'precheck'
  | 'deploy'
  | 'complete';

// Step configuration / 步骤配置
interface StepConfig {
  id: DeployWizardStep;
  titleKey: string;
  descKey: string;
  icon: React.ComponentType<{ className?: string }>;
}

const WIZARD_STEPS: StepConfig[] = [
  { id: 'basic', titleKey: 'cluster.wizard.basic', descKey: 'cluster.wizard.basicDesc', icon: Settings },
  { id: 'hosts', titleKey: 'cluster.wizard.hosts', descKey: 'cluster.wizard.hostsDesc', icon: Server },
  { id: 'precheck', titleKey: 'cluster.wizard.precheck', descKey: 'cluster.wizard.precheckDesc', icon: CheckCircle2 },
  { id: 'config', titleKey: 'cluster.wizard.config', descKey: 'cluster.wizard.configDesc', icon: Settings },
  { id: 'plugins', titleKey: 'cluster.wizard.plugins', descKey: 'cluster.wizard.pluginsDesc', icon: Package },
  { id: 'deploy', titleKey: 'cluster.wizard.deploy', descKey: 'cluster.wizard.deployDesc', icon: PlayCircle },
  { id: 'complete', titleKey: 'cluster.wizard.complete', descKey: 'cluster.wizard.completeDesc', icon: PartyPopper },
];

// Host with role assignment / 带角色分配的主机
interface HostWithRole {
  host: HostInfo;
  selected: boolean;
  role: NodeRole;
}

// Host precheck result / 主机预检查结果
interface HostPrecheckResult {
  hostId: number;
  hostName: string;
  loading: boolean;
  result: PrecheckResult | null;
  error: string | null;
}

// Cluster deploy config / 集群部署配置
interface ClusterDeployConfig {
  // Basic info / 基本信息
  name: string;
  description: string;
  deploymentMode: DeploymentMode;
  // Install config / 安装配置
  version: string;
  installMode: InstallMode;
  mirror: MirrorSource;
  jvm: JVMConfig;
  checkpoint: CheckpointConfig;
  // Plugins / 插件
  selectedPlugins: string[];
}

const defaultConfig: ClusterDeployConfig = {
  name: '',
  description: '',
  deploymentMode: DeploymentMode.HYBRID,
  version: '2.3.12',
  installMode: 'online',
  mirror: 'aliyun',
  jvm: {
    hybrid_heap_size: 3096,
    master_heap_size: 2048,
    worker_heap_size: 2048,
  },
  checkpoint: {
    storage_type: 'LOCAL_FILE',
    namespace: '/tmp/seatunnel/checkpoint',
  },
  selectedPlugins: [],
};

interface ClusterDeployWizardProps {
  /** Whether the dialog is open / 对话框是否打开 */
  open: boolean;
  /** Callback when dialog open state changes / 对话框打开状态变化时的回调 */
  onOpenChange: (open: boolean) => void;
  /** Callback when deployment completes / 部署完成时的回调 */
  onComplete?: (clusterId: number) => void;
}

export function ClusterDeployWizard({
  open,
  onOpenChange,
  onComplete,
}: ClusterDeployWizardProps) {
  const t = useTranslations();
  const [currentStepIndex, setCurrentStepIndex] = useState(0);
  const [config, setConfig] = useState<ClusterDeployConfig>(defaultConfig);
  const [hostsWithRole, setHostsWithRole] = useState<HostWithRole[]>([]);
  const [loadingHosts, setLoadingHosts] = useState(false);
  const [deploying, setDeploying] = useState(false);
  const [deployProgress, setDeployProgress] = useState(0);
  const [deployStatus, setDeployStatus] = useState<'idle' | 'running' | 'success' | 'failed'>('idle');
  const [deployError, setDeployError] = useState<string | null>(null);
  const [createdClusterId, setCreatedClusterId] = useState<number | null>(null);

  // Packages hook / 安装包 hook
  const { packages, loading: packagesLoading } = usePackages();

  // Plugin filter state / 插件筛选状态
  const [pluginSearch, setPluginSearch] = useState('');
  const [localPlugins, setLocalPlugins] = useState<LocalPlugin[]>([]);
  const [localPluginsLoading, setLocalPluginsLoading] = useState(false);

  // Precheck state / 预检查状态
  const [precheckResults, setPrecheckResults] = useState<HostPrecheckResult[]>([]);
  const [precheckRunning, setPrecheckRunning] = useState(false);

  // Load local plugins / 加载本地插件
  const loadLocalPlugins = useCallback(async () => {
    setLocalPluginsLoading(true);
    try {
      const data = await PluginService.listLocalPlugins();
      setLocalPlugins(data || []);
    } catch (err) {
      console.error('Failed to load local plugins:', err);
      setLocalPlugins([]);
    } finally {
      setLocalPluginsLoading(false);
    }
  }, []);

  // Load local plugins when entering plugins step / 进入插件步骤时加载本地插件
  useEffect(() => {
    if (open && currentStepIndex === 3) {
      loadLocalPlugins();
    }
  }, [open, currentStepIndex, loadLocalPlugins]);

  // Load available hosts / 加载可用主机
  const loadHosts = useCallback(async () => {
    setLoadingHosts(true);
    try {
      const result = await services.host.getHostsSafe({ current: 1, size: 100 });
      if (result.success && result.data) {
        // Filter hosts with Agent installed / 过滤已安装 Agent 的主机
        const availableHosts = (result.data.hosts || []).filter(
          (h) => h.host_type === HostType.BARE_METAL && h.agent_status === AgentStatus.INSTALLED
        );
        setHostsWithRole(
          availableHosts.map((host) => ({
            host,
            selected: false,
            // Hybrid mode uses WORKER role for all nodes / 混合模式所有节点使用 WORKER 角色
            role: config.deploymentMode === DeploymentMode.HYBRID ? NodeRole.WORKER : NodeRole.WORKER,
          }))
        );
      }
    } catch (err) {
      console.error('Failed to load hosts:', err);
    } finally {
      setLoadingHosts(false);
    }
  }, [config.deploymentMode]);

  // Load hosts when dialog opens / 对话框打开时加载主机
  useEffect(() => {
    if (open) {
      loadHosts();
    }
  }, [open, loadHosts]);

  // Current step / 当前步骤
  const currentStep = WIZARD_STEPS[currentStepIndex];

  // Calculate progress / 计算进度
  const progress = ((currentStepIndex + 1) / WIZARD_STEPS.length) * 100;

  // Selected hosts / 已选择的主机
  const selectedHosts = useMemo(
    () => hostsWithRole.filter((h) => h.selected),
    [hostsWithRole]
  );

  // Local packages for offline mode / 离线模式的本地安装包
  const localPackages = useMemo(
    () => packages?.local_packages || [],
    [packages?.local_packages]
  );

  // Filter local plugins by version and search / 按版本和搜索过滤本地插件
  const filteredLocalPlugins = useMemo(() => {
    let result = localPlugins.filter((lp) => lp.version === config.version);
    
    // Filter by search keyword / 按搜索关键词过滤
    if (pluginSearch.trim()) {
      const keyword = pluginSearch.toLowerCase();
      result = result.filter((p) => p.name.toLowerCase().includes(keyword));
    }
    
    return result;
  }, [localPlugins, config.version, pluginSearch]);

  // Count local plugins for current version / 统计当前版本的本地插件数量
  const localPluginsForVersion = useMemo(
    () => localPlugins.filter((lp) => lp.version === config.version),
    [localPlugins, config.version]
  );

  // Run precheck when entering precheck step / 进入预检查步骤时运行预检查
  const runPrecheck = useCallback(async () => {
    if (selectedHosts.length === 0) {
      return;
    }

    setPrecheckRunning(true);
    
    // Initialize results for all selected hosts / 初始化所有选中主机的结果
    const initialResults: HostPrecheckResult[] = selectedHosts.map((h) => ({
      hostId: h.host.id,
      hostName: h.host.name,
      loading: true,
      result: null,
      error: null,
    }));
    setPrecheckResults(initialResults);

    // Run precheck for each host in parallel / 并行运行每个主机的预检查
    const promises = selectedHosts.map(async (hostWithRole) => {
      try {
        const result = await services.installer.runPrecheck(hostWithRole.host.id);
        return {
          hostId: hostWithRole.host.id,
          hostName: hostWithRole.host.name,
          loading: false,
          result,
          error: null,
        };
      } catch (err) {
        return {
          hostId: hostWithRole.host.id,
          hostName: hostWithRole.host.name,
          loading: false,
          result: null,
          error: err instanceof Error ? err.message : 'Precheck failed',
        };
      }
    });

    const results = await Promise.all(promises);
    setPrecheckResults(results);
    setPrecheckRunning(false);
  }, [selectedHosts]);

  // Check if all prechecks passed / 检查是否所有预检查都通过
  const allPrechecksPassed = useMemo(() => {
    if (precheckResults.length === 0) {
      return false;
    }
    if (precheckRunning) {
      return false;
    }
    return precheckResults.every(
      (r) =>
        r.result &&
        (r.result.overall_status === 'passed' || r.result.overall_status === 'warning')
    );
  }, [precheckResults, precheckRunning]);

  // Check if precheck has been run / 检查是否已运行过预检查
  const precheckHasRun = useMemo(() => {
    return precheckResults.length > 0 && !precheckRunning;
  }, [precheckResults, precheckRunning]);

  // Update config / 更新配置
  const updateConfig = useCallback((updates: Partial<ClusterDeployConfig>) => {
    setConfig((prev) => {
      // If deployment mode changes, reset precheck results / 如果部署模式变化，重置预检查结果
      if (updates.deploymentMode !== undefined && updates.deploymentMode !== prev.deploymentMode) {
        setPrecheckResults([]);
        setPrecheckRunning(false);
      }
      return { ...prev, ...updates };
    });
  }, []);

  // Toggle host selection / 切换主机选择
  const toggleHostSelection = useCallback((hostId: number) => {
    setHostsWithRole((prev) =>
      prev.map((h) =>
        h.host.id === hostId ? { ...h, selected: !h.selected } : h
      )
    );
    // Reset precheck results when host selection changes / 主机选择变化时重置预检查结果
    setPrecheckResults([]);
    setPrecheckRunning(false);
  }, []);

  // Update host role / 更新主机角色
  const updateHostRole = useCallback((hostId: number, role: NodeRole) => {
    setHostsWithRole((prev) =>
      prev.map((h) => (h.host.id === hostId ? { ...h, role } : h))
    );
  }, []);

  // Check if can proceed to next step / 检查是否可以进入下一步
  const canProceed = useCallback(() => {
    switch (currentStep.id) {
      case 'basic':
        return config.name.trim().length > 0;
      case 'hosts':
        if (selectedHosts.length === 0) {
          return false;
        }
        // For separated mode, need at least one master / 分离模式需要至少一个 master
        if (config.deploymentMode === DeploymentMode.SEPARATED) {
          const hasMaster = selectedHosts.some((h) => h.role === NodeRole.MASTER);
          const hasWorker = selectedHosts.some((h) => h.role === NodeRole.WORKER);
          return hasMaster && hasWorker;
        }
        return true;
      case 'precheck':
        return allPrechecksPassed; // Must pass precheck / 必须通过预检查
      case 'config':
        // For offline mode, must have local packages available / 离线模式必须有本地安装包
        if (config.installMode === 'offline') {
          const hasLocalPackage = localPackages.some((pkg) => pkg.version === config.version);
          return hasLocalPackage;
        }
        return config.version.length > 0;
      case 'plugins':
        return true; // Optional / 可选
      case 'deploy':
        return deployStatus === 'success';
      case 'complete':
        return true;
      default:
        return false;
    }
  }, [currentStep.id, config, selectedHosts, deployStatus, localPackages, allPrechecksPassed]);

  // Handle deploy / 处理部署
  const handleDeploy = useCallback(async () => {
    setDeploying(true);
    setDeployStatus('running');
    setDeployProgress(0);
    setDeployError(null);

    try {
      let clusterId = createdClusterId;

      // Step 1: Create cluster (skip if already created) / 步骤1：创建集群（如果已创建则跳过）
      if (!clusterId) {
        setDeployProgress(10);
        const clusterResult = await services.cluster.createClusterSafe({
          name: config.name,
          description: config.description || undefined,
          deployment_mode: config.deploymentMode,
          version: config.version,
        });

        if (!clusterResult.success || !clusterResult.data) {
          throw new Error(clusterResult.error || 'Failed to create cluster');
        }

        clusterId = clusterResult.data.id;
        setCreatedClusterId(clusterId);
      }
      setDeployProgress(20);

      // Step 2: Add nodes to cluster / 步骤2：添加节点到集群
      for (let i = 0; i < selectedHosts.length; i++) {
        const hostWithRole = selectedHosts[i];
        // Use addNodeSafe which handles duplicates gracefully
        // 使用 addNodeSafe，它会优雅地处理重复添加
        await services.cluster.addNodeSafe(clusterId, {
          host_id: hostWithRole.host.id,
          role: hostWithRole.role,
        });
        setDeployProgress(20 + ((i + 1) / selectedHosts.length) * 30);
      }

      // Step 3: Install SeaTunnel on each host / 步骤3：在每台主机上安装 SeaTunnel
      for (let i = 0; i < selectedHosts.length; i++) {
        const hostWithRole = selectedHosts[i];
        
        // Start installation / 开始安装
        const installResult = await services.installer.startInstallation(hostWithRole.host.id, {
          cluster_id: String(clusterId),
          version: config.version,
          install_mode: config.installMode,
          mirror: config.installMode === 'online' ? config.mirror : undefined,
          deployment_mode: config.deploymentMode,
          node_role: hostWithRole.role,
          jvm: config.jvm,
          checkpoint: config.checkpoint,
          connector: config.selectedPlugins.length > 0 ? {
            install_connectors: true,
            connectors: [],
            selected_plugins: config.selectedPlugins,
          } : undefined,
        });

        // Poll for completion / 轮询等待完成
        let status = installResult;
        while (status.status === 'running') {
          await new Promise((resolve) => setTimeout(resolve, 2000));
          status = await services.installer.getInstallationStatus(hostWithRole.host.id);
        }

        if (status.status === 'failed') {
          throw new Error(`Installation failed on host ${hostWithRole.host.name}: ${status.error}`);
        }

        setDeployProgress(50 + ((i + 1) / selectedHosts.length) * 40);
      }

      // Step 4: Complete / 步骤4：完成
      setDeployProgress(100);
      setDeployStatus('success');
      toast.success(t('cluster.wizard.deploySuccess'));
      
      // Auto advance to complete step / 自动跳转到完成步骤
      setCurrentStepIndex(6); // complete step index
    } catch (err) {
      setDeployStatus('failed');
      setDeployError(err instanceof Error ? err.message : 'Deployment failed');
      toast.error(t('cluster.wizard.deployFailed'));
    } finally {
      setDeploying(false);
    }
  }, [config, selectedHosts, t, createdClusterId]);

  // Handle next step / 处理下一步
  const handleNext = useCallback(() => {
    if (currentStep.id === 'plugins') {
      // Start deployment when moving from plugins to deploy
      // 从插件步骤进入部署步骤时开始部署
      setCurrentStepIndex(currentStepIndex + 1);
      handleDeploy();
    } else if (currentStepIndex < WIZARD_STEPS.length - 1) {
      setCurrentStepIndex(currentStepIndex + 1);
    }
  }, [currentStep.id, currentStepIndex, handleDeploy]);

  // Handle previous step / 处理上一步
  const handlePrevious = useCallback(() => {
    if (currentStepIndex > 0 && currentStep.id !== 'deploy' && currentStep.id !== 'complete') {
      setCurrentStepIndex(currentStepIndex - 1);
    }
  }, [currentStepIndex, currentStep.id]);

  // Handle close / 处理关闭
  const handleClose = useCallback(() => {
    if (deploying) {
      if (!confirm(t('cluster.wizard.confirmCancel'))) {
        return;
      }
    }
    // Reset state / 重置状态
    setCurrentStepIndex(0);
    setConfig(defaultConfig);
    setHostsWithRole([]);
    setDeployStatus('idle');
    setDeployProgress(0);
    setDeployError(null);
    setCreatedClusterId(null);
    setPrecheckResults([]);
    setPrecheckRunning(false);
    onOpenChange(false);
  }, [deploying, t, onOpenChange]);

  // Handle complete / 处理完成
  const handleComplete = useCallback(() => {
    if (createdClusterId) {
      onComplete?.(createdClusterId);
    }
    handleClose();
  }, [createdClusterId, onComplete, handleClose]);

  // Render basic info step / 渲染基本信息步骤
  const renderBasicStep = () => (
    <div className="space-y-4">
      <div className="space-y-2">
        <Label htmlFor="name">
          {t('cluster.name')} <span className="text-destructive">*</span>
        </Label>
        <Input
          id="name"
          value={config.name}
          onChange={(e) => updateConfig({ name: e.target.value })}
          placeholder={t('cluster.namePlaceholder')}
        />
      </div>

      <div className="space-y-2">
        <Label htmlFor="description">{t('cluster.descriptionLabel')}</Label>
        <Textarea
          id="description"
          value={config.description}
          onChange={(e) => updateConfig({ description: e.target.value })}
          placeholder={t('cluster.descriptionPlaceholder')}
          rows={2}
        />
      </div>

      <div className="space-y-2">
        <Label>{t('cluster.deploymentMode')}</Label>
        <div className="grid grid-cols-2 gap-4">
          <Card
            className={cn(
              'cursor-pointer transition-colors',
              config.deploymentMode === DeploymentMode.HYBRID
                ? 'border-primary bg-primary/5'
                : 'hover:border-muted-foreground/50'
            )}
            onClick={() => updateConfig({ deploymentMode: DeploymentMode.HYBRID })}
          >
            <CardHeader className="pb-2">
              <CardTitle className="text-sm">{t('cluster.modes.hybrid')}</CardTitle>
            </CardHeader>
            <CardContent>
              <p className="text-xs text-muted-foreground">{t('cluster.hybridDescription')}</p>
            </CardContent>
          </Card>
          <Card
            className={cn(
              'cursor-pointer transition-colors',
              config.deploymentMode === DeploymentMode.SEPARATED
                ? 'border-primary bg-primary/5'
                : 'hover:border-muted-foreground/50'
            )}
            onClick={() => updateConfig({ deploymentMode: DeploymentMode.SEPARATED })}
          >
            <CardHeader className="pb-2">
              <CardTitle className="text-sm">{t('cluster.modes.separated')}</CardTitle>
            </CardHeader>
            <CardContent>
              <p className="text-xs text-muted-foreground">{t('cluster.separatedDescription')}</p>
            </CardContent>
          </Card>
        </div>
      </div>
    </div>
  );

  // Render hosts selection step / 渲染主机选择步骤
  const renderHostsStep = () => (
    <div className="space-y-4 h-full flex flex-col">
      <div className="flex items-center justify-between">
        <p className="text-sm text-muted-foreground">
          {t('cluster.wizard.selectHostsDesc')}
        </p>
        <Badge variant="outline">
          {selectedHosts.length} {t('cluster.wizard.hostsSelected')}
        </Badge>
      </div>

      {config.deploymentMode === DeploymentMode.SEPARATED && (
        <div className="flex gap-4 text-sm">
          <div className="flex items-center gap-2">
            <Crown className="h-4 w-4 text-yellow-500" />
            <span>Master: {selectedHosts.filter((h) => h.role === NodeRole.MASTER).length}</span>
          </div>
          <div className="flex items-center gap-2">
            <Wrench className="h-4 w-4 text-blue-500" />
            <span>Worker: {selectedHosts.filter((h) => h.role === NodeRole.WORKER).length}</span>
          </div>
        </div>
      )}

      <ScrollArea className="flex-1 pr-4">
        {loadingHosts ? (
          <div className="flex items-center justify-center py-12">
            <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
          </div>
        ) : hostsWithRole.length === 0 ? (
          <div className="text-center py-12 text-muted-foreground">
            <Server className="h-12 w-12 mx-auto mb-4 opacity-50" />
            <p>{t('cluster.wizard.noAvailableHosts')}</p>
            <p className="text-xs mt-2">{t('cluster.wizard.noAvailableHostsDesc')}</p>
          </div>
        ) : (
          <div className="space-y-2">
            {hostsWithRole.map((hostWithRole) => (
              <Card
                key={hostWithRole.host.id}
                className={cn(
                  'cursor-pointer transition-colors',
                  hostWithRole.selected && 'border-primary bg-primary/5'
                )}
              >
                <CardContent className="p-4">
                  <div className="flex items-center gap-4">
                    <Checkbox
                      checked={hostWithRole.selected}
                      onCheckedChange={() => toggleHostSelection(hostWithRole.host.id)}
                    />
                    <div className="flex-1 min-w-0">
                      <div className="flex items-center gap-2">
                        <span className="font-medium">{hostWithRole.host.name}</span>
                        <Badge variant="outline" className="text-xs">
                          {hostWithRole.host.ip_address}
                        </Badge>
                      </div>
                      <p className="text-xs text-muted-foreground mt-1">
                        CPU: {hostWithRole.host.cpu_cores} cores | 
                        Memory: {((hostWithRole.host.total_memory || 0) / 1024 / 1024 / 1024).toFixed(1)} GB
                      </p>
                    </div>
                    {hostWithRole.selected && config.deploymentMode === DeploymentMode.SEPARATED && (
                      <Select
                        value={hostWithRole.role}
                        onValueChange={(value) => updateHostRole(hostWithRole.host.id, value as NodeRole)}
                      >
                        <SelectTrigger className="w-[120px]">
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value={NodeRole.MASTER}>
                            <div className="flex items-center gap-2">
                              <Crown className="h-3 w-3 text-yellow-500" />
                              Master
                            </div>
                          </SelectItem>
                          <SelectItem value={NodeRole.WORKER}>
                            <div className="flex items-center gap-2">
                              <Wrench className="h-3 w-3 text-blue-500" />
                              Worker
                            </div>
                          </SelectItem>
                        </SelectContent>
                      </Select>
                    )}
                  </div>
                </CardContent>
              </Card>
            ))}
          </div>
        )}
      </ScrollArea>
    </div>
  );

  // Render config step / 渲染配置步骤
  const renderConfigStep = () => (
    <div className="h-full flex flex-col">
      <ScrollArea className="flex-1 pr-4">
        <div className="space-y-4">
        {/* Install Mode / 安装模式 */}
        <Card>
          <CardHeader className="pb-3">
            <CardTitle className="text-base">{t('installer.installMode')}</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="grid grid-cols-2 gap-4">
              <div
                className={cn(
                  'p-4 rounded-lg border-2 cursor-pointer transition-colors',
                  config.installMode === 'online'
                    ? 'border-primary bg-primary/5'
                    : 'border-muted hover:border-muted-foreground/50'
                )}
                onClick={() => updateConfig({ installMode: 'online' })}
              >
                <span className="font-medium">{t('installer.online')}</span>
                <p className="text-xs text-muted-foreground mt-1">{t('installer.onlineDesc')}</p>
              </div>
              <div
                className={cn(
                  'p-4 rounded-lg border-2 cursor-pointer transition-colors',
                  config.installMode === 'offline'
                    ? 'border-primary bg-primary/5'
                    : 'border-muted hover:border-muted-foreground/50'
                )}
                onClick={() => updateConfig({ installMode: 'offline' })}
              >
                <span className="font-medium">{t('installer.offline')}</span>
                <p className="text-xs text-muted-foreground mt-1">{t('installer.offlineDesc')}</p>
              </div>
            </div>

            {/* Online mode: select mirror and version / 在线模式：选择镜像源和版本 */}
            {config.installMode === 'online' && (
              <div className="mt-4 space-y-4">
                <div className="grid grid-cols-2 gap-4">
                  <div className="space-y-2">
                    <Label>{t('installer.mirrorSource')}</Label>
                    <Select
                      value={config.mirror}
                      onValueChange={(value: MirrorSource) => updateConfig({ mirror: value })}
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
                  <div className="space-y-2">
                    <Label>{t('installer.version')}</Label>
                    <Select
                      value={config.version}
                      onValueChange={(value) => updateConfig({ version: value })}
                      disabled={packagesLoading}
                    >
                      <SelectTrigger>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        {(packages?.versions || ['2.3.12']).map((version) => {
                          const isLocal = localPackages.some((pkg) => pkg.version === version);
                          return (
                            <SelectItem key={version} value={version}>
                              <div className="flex items-center gap-2">
                                {version}
                                {isLocal && (
                                  <Download className="h-3 w-3 text-green-500" />
                                )}
                                {version === packages?.recommended_version && (
                                  <Badge variant="secondary" className="text-xs">
                                    {t('installer.recommended')}
                                  </Badge>
                                )}
                              </div>
                            </SelectItem>
                          );
                        })}
                      </SelectContent>
                    </Select>
                  </div>
                </div>
                
                {/* Local package status hint / 本地安装包状态提示 */}
                {(() => {
                  const isVersionLocal = localPackages.some((pkg) => pkg.version === config.version);
                  if (isVersionLocal) {
                    return (
                      <div className="p-3 bg-green-50 dark:bg-green-900/20 rounded-lg">
                        <div className="flex items-center gap-2 text-green-700 dark:text-green-300">
                          <CheckCircle2 className="h-4 w-4" />
                          <span className="text-sm">{t('cluster.wizard.packageAlreadyDownloaded')}</span>
                        </div>
                      </div>
                    );
                  } else {
                    return (
                      <div className="p-3 bg-blue-50 dark:bg-blue-900/20 rounded-lg">
                        <div className="flex items-start gap-2">
                          <Download className="h-4 w-4 text-blue-600 mt-0.5" />
                          <div className="flex-1">
                            <p className="text-sm text-blue-700 dark:text-blue-300">
                              {t('cluster.wizard.packageWillDownload')}
                            </p>
                            <p className="text-xs text-blue-600 dark:text-blue-400 mt-1">
                              {t('cluster.wizard.packageWillDownloadDesc')}
                            </p>
                          </div>
                        </div>
                      </div>
                    );
                  }
                })()}
              </div>
            )}

            {/* Offline mode: select from local packages / 离线模式：从本地安装包选择 */}
            {config.installMode === 'offline' && (
              <div className="mt-4 space-y-4">
                {packagesLoading ? (
                  <div className="flex items-center justify-center py-8">
                    <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
                  </div>
                ) : localPackages.length === 0 ? (
                  /* No local packages available / 没有本地安装包 */
                  <div className="p-4 bg-yellow-50 dark:bg-yellow-900/20 rounded-lg">
                    <div className="flex items-start gap-3">
                      <AlertTriangle className="h-5 w-5 text-yellow-600 mt-0.5" />
                      <div className="flex-1">
                        <p className="text-sm font-medium text-yellow-700 dark:text-yellow-300">
                          {t('cluster.wizard.noLocalPackages')}
                        </p>
                        <p className="text-xs text-yellow-600 dark:text-yellow-400 mt-1">
                          {t('cluster.wizard.noLocalPackagesDesc')}
                        </p>
                        <Button
                          variant="outline"
                          size="sm"
                          className="mt-3"
                          onClick={() => window.open('/packages', '_blank')}
                        >
                          <Package className="h-4 w-4 mr-2" />
                          {t('cluster.wizard.goToPackageManagement')}
                        </Button>
                      </div>
                    </div>
                  </div>
                ) : (
                  /* Local packages list / 本地安装包列表 */
                  <div className="space-y-2">
                    <Label>{t('installer.selectPackage')}</Label>
                    <div className="space-y-2">
                      {localPackages.map((pkg) => (
                        <div
                          key={pkg.version}
                          className={cn(
                            'p-3 rounded-lg border-2 cursor-pointer transition-colors flex items-center justify-between',
                            config.version === pkg.version
                              ? 'border-primary bg-primary/5'
                              : 'border-muted hover:border-muted-foreground/50'
                          )}
                          onClick={() => updateConfig({ version: pkg.version })}
                        >
                          <div className="flex items-center gap-3">
                            <Package className="h-5 w-5 text-muted-foreground" />
                            <div>
                              <span className="font-medium">{pkg.version}</span>
                              <p className="text-xs text-muted-foreground">
                                {pkg.file_name} ({(pkg.file_size / 1024 / 1024).toFixed(1)} MB)
                              </p>
                            </div>
                          </div>
                          {config.version === pkg.version && (
                            <CheckCircle2 className="h-5 w-5 text-primary" />
                          )}
                        </div>
                      ))}
                    </div>
                  </div>
                )}
              </div>
            )}
          </CardContent>
        </Card>

        {/* JVM Config & Checkpoint Config / JVM 配置和检查点配置 */}
        <div className="grid grid-cols-2 gap-4">
          {/* JVM Config / JVM 配置 */}
          <Card>
            <CardHeader className="pb-2">
              <CardTitle className="text-base">{t('installer.jvmConfig')}</CardTitle>
            </CardHeader>
            <CardContent>
              {config.deploymentMode === DeploymentMode.HYBRID ? (
                <div className="space-y-2">
                  <Label>{t('installer.hybridHeapSize')}</Label>
                  <Input
                    type="number"
                    value={config.jvm.hybrid_heap_size}
                    onChange={(e) =>
                      updateConfig({
                        jvm: { ...config.jvm, hybrid_heap_size: parseInt(e.target.value) || 0 },
                      })
                    }
                    min={512}
                    step={256}
                  />
                </div>
              ) : (
                <div className="space-y-3">
                  <div className="space-y-2">
                    <Label>{t('installer.masterHeapSize')}</Label>
                    <Input
                      type="number"
                      value={config.jvm.master_heap_size}
                      onChange={(e) =>
                        updateConfig({
                          jvm: { ...config.jvm, master_heap_size: parseInt(e.target.value) || 0 },
                        })
                      }
                      min={512}
                      step={256}
                    />
                  </div>
                  <div className="space-y-2">
                    <Label>{t('installer.workerHeapSize')}</Label>
                    <Input
                      type="number"
                      value={config.jvm.worker_heap_size}
                      onChange={(e) =>
                        updateConfig({
                          jvm: { ...config.jvm, worker_heap_size: parseInt(e.target.value) || 0 },
                        })
                      }
                      min={512}
                      step={256}
                    />
                  </div>
                </div>
              )}
            </CardContent>
          </Card>

          {/* Checkpoint Config / 检查点配置 */}
          <Card>
            <CardHeader className="pb-2">
              <CardTitle className="text-base">{t('installer.checkpointConfig')}</CardTitle>
            </CardHeader>
            <CardContent className="space-y-3">
              <div className="space-y-2">
                <Label>{t('installer.storageType')}</Label>
                <Select
                  value={config.checkpoint.storage_type}
                  onValueChange={(value: CheckpointStorageType) =>
                    updateConfig({
                      checkpoint: { ...config.checkpoint, storage_type: value },
                    })
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
              <div className="space-y-2">
                <Label>{t('installer.namespace')}</Label>
                <Input
                  value={config.checkpoint.namespace}
                  onChange={(e) =>
                    updateConfig({
                      checkpoint: { ...config.checkpoint, namespace: e.target.value },
                    })
                  }
                  placeholder="/tmp/seatunnel/checkpoint"
                />
              </div>
            </CardContent>
          </Card>
        </div>
      </div>
    </ScrollArea>
    </div>
  );

  // Render plugins step / 渲染插件步骤
  const renderPluginsStep = () => {
    // Select all filtered plugins / 全选过滤后的插件
    const handleSelectAll = () => {
      const allNames = filteredLocalPlugins.map((p) => p.name);
      const newSelected = [...new Set([...config.selectedPlugins, ...allNames])];
      updateConfig({ selectedPlugins: newSelected });
    };

    // Deselect all filtered plugins / 取消全选过滤后的插件
    const handleDeselectAll = () => {
      const filteredNames = new Set(filteredLocalPlugins.map((p) => p.name));
      const newSelected = config.selectedPlugins.filter((name) => !filteredNames.has(name));
      updateConfig({ selectedPlugins: newSelected });
    };

    return (
      <div className="space-y-4 h-full flex flex-col">
        {/* Header with count / 标题和计数 */}
        <div className="flex items-center justify-between">
          <div>
            <p className="text-sm text-muted-foreground">
              {t('cluster.wizard.selectLocalPluginsDesc')}
            </p>
            <p className="text-xs text-muted-foreground mt-1">
              {t('cluster.wizard.localPluginsCount', { count: localPluginsForVersion.length, version: config.version })}
            </p>
          </div>
          <Badge variant="outline">
            {config.selectedPlugins.length} {t('plugin.selectedCount')}
          </Badge>
        </div>

        {/* No local plugins warning / 没有本地插件警告 */}
        {localPluginsForVersion.length === 0 ? (
          <div className="flex-1 flex items-center justify-center">
            <div className="text-center p-8 max-w-md">
              <Package className="h-16 w-16 mx-auto mb-4 text-muted-foreground/50" />
              <h3 className="text-lg font-medium mb-2">{t('cluster.wizard.noLocalPluginsForVersion')}</h3>
              <p className="text-sm text-muted-foreground mb-4">
                {t('cluster.wizard.noLocalPluginsForVersionDesc', { version: config.version })}
              </p>
              <div className="flex flex-col gap-2">
                <Button
                  variant="default"
                  onClick={() => window.open('/plugins', '_blank')}
                >
                  <Download className="h-4 w-4 mr-2" />
                  {t('cluster.wizard.goToPluginMarket')}
                </Button>
                <p className="text-xs text-muted-foreground">
                  {t('cluster.wizard.pluginsOptional')}
                </p>
              </div>
            </div>
          </div>
        ) : (
          <>
            {/* Filters / 筛选器 */}
            <div className="flex flex-wrap gap-3 items-center">
              {/* Search / 搜索 */}
              <div className="relative flex-1 min-w-[200px]">
                <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
                <Input
                  placeholder={t('plugin.searchPlaceholder')}
                  value={pluginSearch}
                  onChange={(e) => setPluginSearch(e.target.value)}
                  className="pl-9"
                />
              </div>

              {/* Select all / Deselect all / 全选/取消全选 */}
              <div className="flex gap-2">
                <Button
                  variant="outline"
                  size="sm"
                  onClick={handleSelectAll}
                  disabled={filteredLocalPlugins.length === 0}
                >
                  {t('common.selectAll')}
                </Button>
                <Button
                  variant="outline"
                  size="sm"
                  onClick={handleDeselectAll}
                  disabled={config.selectedPlugins.length === 0}
                >
                  {t('common.deselectAll')}
                </Button>
              </div>
            </div>

            {/* Plugin list / 插件列表 */}
            <ScrollArea className="flex-1 pr-4">
              {localPluginsLoading ? (
                <div className="flex items-center justify-center py-12">
                  <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
                </div>
              ) : filteredLocalPlugins.length === 0 ? (
                <div className="text-center py-12 text-muted-foreground">
                  <Package className="h-12 w-12 mx-auto mb-4 opacity-50" />
                  <p>{t('plugin.noPluginsFound')}</p>
                </div>
              ) : (
                <div className="grid grid-cols-4 gap-2">
                  {filteredLocalPlugins.map((plugin) => {
                    const isSelected = config.selectedPlugins.includes(plugin.name);
                    
                    return (
                      <Card
                        key={plugin.name}
                        className={cn(
                          'cursor-pointer transition-colors hover:border-muted-foreground/50',
                          isSelected && 'border-primary bg-primary/5'
                        )}
                        onClick={() => {
                          if (isSelected) {
                            updateConfig({
                              selectedPlugins: config.selectedPlugins.filter((p) => p !== plugin.name),
                            });
                          } else {
                            updateConfig({
                              selectedPlugins: [...config.selectedPlugins, plugin.name],
                            });
                          }
                        }}
                      >
                        <CardContent className="p-3">
                          <div className="flex items-start gap-2">
                            <Checkbox 
                              checked={isSelected} 
                              className="mt-0.5"
                              onClick={(e) => e.stopPropagation()}
                              onCheckedChange={(checked) => {
                                if (checked) {
                                  updateConfig({
                                    selectedPlugins: [...config.selectedPlugins, plugin.name],
                                  });
                                } else {
                                  updateConfig({
                                    selectedPlugins: config.selectedPlugins.filter((p) => p !== plugin.name),
                                  });
                                }
                              }}
                            />
                            <div className="flex-1 min-w-0">
                              <span className="text-sm font-medium truncate block">
                                {plugin.name}
                              </span>
                              <div className="flex items-center gap-1 mt-1">
                                <Badge variant="outline" className="text-xs">
                                  {plugin.category}
                                </Badge>
                                <span className="text-xs text-muted-foreground">
                                  {(plugin.size / 1024 / 1024).toFixed(1)}MB
                                </span>
                              </div>
                            </div>
                          </div>
                        </CardContent>
                      </Card>
                    );
                  })}
                </div>
              )}
            </ScrollArea>

            {/* Footer hint / 底部提示 */}
            <div className="flex items-center justify-between text-xs text-muted-foreground pt-2 border-t">
              <p>{t('cluster.wizard.pluginsOptional')}</p>
              <Button
                variant="link"
                size="sm"
                className="h-auto p-0 text-xs"
                onClick={() => window.open('/plugins', '_blank')}
              >
                {t('cluster.wizard.goToPluginMarket')}
              </Button>
            </div>
          </>
        )}
      </div>
    );
  };

  // Get status icon for precheck item / 获取预检查项的状态图标
  const getPrecheckStatusIcon = (status: CheckStatus) => {
    switch (status) {
      case 'passed':
        return <CheckCircle2 className="h-4 w-4 text-green-500" />;
      case 'failed':
        return <X className="h-4 w-4 text-red-500" />;
      case 'warning':
        return <AlertTriangle className="h-4 w-4 text-yellow-500" />;
      default:
        return <Loader2 className="h-4 w-4 animate-spin text-muted-foreground" />;
    }
  };

  // Render precheck step / 渲染预检查步骤
  const renderPrecheckStep = () => (
    <div className="space-y-4 h-full flex flex-col overflow-hidden">
      {/* Summary header / 摘要头部 */}
      <div className="flex items-center justify-between">
        <div>
          <h3 className="text-sm font-medium">{t('cluster.wizard.precheckTitle')}</h3>
          <p className="text-xs text-muted-foreground">{t('cluster.wizard.precheckDesc')}</p>
        </div>
        <div className="flex items-center gap-2">
          {precheckRunning ? (
            <Badge variant="outline" className="gap-1">
              <Loader2 className="h-3 w-3 animate-spin" />
              {t('cluster.wizard.precheckRunning')}
            </Badge>
          ) : precheckHasRun ? (
            allPrechecksPassed ? (
              <Badge variant="default" className="gap-1 bg-green-500">
                <CheckCircle2 className="h-3 w-3" />
                {t('cluster.wizard.precheckPassed')}
              </Badge>
            ) : (
              <Badge variant="destructive" className="gap-1">
                <X className="h-3 w-3" />
                {t('cluster.wizard.precheckFailed')}
              </Badge>
            )
          ) : null}
        </div>
      </div>

      {/* Initial state - show start button / 初始状态 - 显示开始按钮 */}
      {!precheckHasRun && !precheckRunning ? (
        <div className="flex-1 flex items-center justify-center">
          <div className="text-center p-8 max-w-md">
            <CheckCircle2 className="h-16 w-16 mx-auto mb-4 text-muted-foreground/50" />
            <h3 className="text-lg font-medium mb-2">{t('cluster.wizard.readyToPrecheck')}</h3>
            <p className="text-sm text-muted-foreground mb-6">
              {t('cluster.wizard.readyToPrecheckDesc', { count: selectedHosts.length })}
            </p>
            <Button onClick={runPrecheck} size="lg">
              <PlayCircle className="h-5 w-5 mr-2" />
              {t('cluster.wizard.startPrecheck')}
            </Button>
          </div>
        </div>
      ) : (
        <>
          {/* Precheck results / 预检查结果 */}
          <ScrollArea className="flex-1 min-h-0 pr-4">
            <div className="space-y-4">
              {precheckResults.map((hostResult) => (
                <Card key={hostResult.hostId}>
                  <CardHeader className="pb-2">
                    <div className="flex items-center justify-between">
                      <div className="flex items-center gap-2">
                        <Server className="h-4 w-4" />
                        <CardTitle className="text-sm">{hostResult.hostName}</CardTitle>
                      </div>
                      {hostResult.loading ? (
                        <Badge variant="outline" className="gap-1">
                          <Loader2 className="h-3 w-3 animate-spin" />
                          {t('cluster.wizard.checking')}
                        </Badge>
                      ) : hostResult.error ? (
                        <Badge variant="destructive">{t('cluster.wizard.checkError')}</Badge>
                      ) : hostResult.result?.overall_status === 'passed' ? (
                        <Badge variant="default" className="bg-green-500">
                          {t('cluster.wizard.passed')}
                        </Badge>
                      ) : hostResult.result?.overall_status === 'warning' ? (
                        <Badge variant="secondary" className="bg-yellow-500 text-white">
                          {t('cluster.wizard.warning')}
                        </Badge>
                      ) : (
                        <Badge variant="destructive">{t('cluster.wizard.failed')}</Badge>
                      )}
                    </div>
                  </CardHeader>
                  <CardContent>
                    {hostResult.loading ? (
                      <div className="flex items-center justify-center py-4">
                        <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
                      </div>
                    ) : hostResult.error ? (
                      <div className="p-3 bg-red-50 dark:bg-red-900/20 rounded-lg">
                        <p className="text-sm text-red-600 dark:text-red-400">{hostResult.error}</p>
                      </div>
                    ) : hostResult.result ? (
                      <div className="space-y-2">
                        {hostResult.result.items.map((item, index) => (
                          <div
                            key={index}
                            className={cn(
                              'flex items-start gap-3 p-2 rounded-md',
                              item.status === 'passed' && 'bg-green-50 dark:bg-green-900/20',
                              item.status === 'failed' && 'bg-red-50 dark:bg-red-900/20',
                              item.status === 'warning' && 'bg-yellow-50 dark:bg-yellow-900/20'
                            )}
                          >
                            {getPrecheckStatusIcon(item.status)}
                            <div className="flex-1 min-w-0">
                              <div className="flex items-center gap-2">
                                <span className="text-sm font-medium capitalize">{item.name}</span>
                              </div>
                              <p className="text-xs text-muted-foreground mt-0.5">{item.message}</p>
                              {/* Show details if available / 如果有详情则显示 */}
                              {item.details && Object.keys(item.details).length > 0 && (
                                <div className="mt-1 text-xs text-muted-foreground/80">
                                  {Object.entries(item.details).map(([key, value]) => (
                                    <div key={key} className="flex gap-1">
                                      <span className="font-medium">{key}:</span>
                                      <span>{JSON.stringify(value)}</span>
                                    </div>
                                  ))}
                                </div>
                              )}
                            </div>
                          </div>
                        ))}
                        {hostResult.result.summary && (
                          <p className="text-xs text-muted-foreground pt-2 border-t">
                            {hostResult.result.summary}
                          </p>
                        )}
                      </div>
                    ) : null}
                  </CardContent>
                </Card>
              ))}
            </div>
          </ScrollArea>

          {/* Rerun button / 重新运行按钮 */}
          {precheckHasRun && (
            <div className="flex justify-center pt-2">
              <Button variant="outline" onClick={runPrecheck} disabled={precheckRunning}>
                {precheckRunning ? (
                  <Loader2 className="h-4 w-4 animate-spin mr-2" />
                ) : (
                  <PlayCircle className="h-4 w-4 mr-2" />
                )}
                {t('cluster.wizard.rerunPrecheck')}
              </Button>
            </div>
          )}

          {/* Warning/Success message / 警告/成功消息 */}
          {precheckHasRun && !allPrechecksPassed && (
            <div className="flex items-center gap-2 p-4 bg-red-50 dark:bg-red-900/20 rounded-lg">
              <AlertTriangle className="h-5 w-5 text-red-600" />
              <p className="text-sm text-red-700 dark:text-red-300">
                {t('cluster.wizard.precheckFailedWarning')}
              </p>
            </div>
          )}

          {precheckHasRun && allPrechecksPassed && (
            <div className="flex items-center gap-2 p-4 bg-green-50 dark:bg-green-900/20 rounded-lg">
              <CheckCircle2 className="h-5 w-5 text-green-600" />
              <p className="text-sm text-green-700 dark:text-green-300">
                {t('cluster.wizard.precheckPassedInfo')}
              </p>
            </div>
          )}
        </>
      )}
    </div>
  );

  // Render deploy step / 渲染部署步骤
  const renderDeployStep = () => (
    <div className="space-y-6">
      <Card>
        <CardContent className="pt-6">
          <div className="text-center mb-6">
            {deployStatus === 'running' && (
              <>
                <Loader2 className="h-12 w-12 animate-spin mx-auto text-primary mb-4" />
                <h3 className="text-lg font-medium">{t('cluster.wizard.deploying')}</h3>
                <p className="text-sm text-muted-foreground mt-1">
                  {t('cluster.wizard.deployingDesc')}
                </p>
              </>
            )}
            {deployStatus === 'success' && (
              <>
                <CheckCircle2 className="h-12 w-12 mx-auto text-green-500 mb-4" />
                <h3 className="text-lg font-medium text-green-600">{t('cluster.wizard.deploySuccess')}</h3>
              </>
            )}
            {deployStatus === 'failed' && (
              <>
                <AlertTriangle className="h-12 w-12 mx-auto text-red-500 mb-4" />
                <h3 className="text-lg font-medium text-red-600">{t('cluster.wizard.deployFailed')}</h3>
                {deployError && (
                  <p className="text-sm text-red-500 mt-2">{deployError}</p>
                )}
                <div className="flex gap-2 mt-4 justify-center">
                  <Button
                    variant="outline"
                    onClick={() => {
                      // Go back to plugins step to adjust config
                      // 返回插件步骤调整配置
                      setDeployStatus('idle');
                      setDeployProgress(0);
                      setDeployError(null);
                      setCurrentStepIndex(4); // plugins step index
                    }}
                    disabled={deploying}
                  >
                    <ChevronLeft className="h-4 w-4 mr-2" />
                    {t('common.previous')}
                  </Button>
                  <Button
                    variant="default"
                    onClick={handleDeploy}
                    disabled={deploying}
                  >
                    <PlayCircle className="h-4 w-4 mr-2" />
                    {t('common.retry')}
                  </Button>
                </div>
              </>
            )}
          </div>

          <div className="space-y-2">
            <div className="flex items-center justify-between text-sm">
              <span>{t('cluster.wizard.progress')}</span>
              <span>{deployProgress}%</span>
            </div>
            <Progress value={deployProgress} className="h-3" />
          </div>
        </CardContent>
      </Card>
    </div>
  );

  // Render complete step / 渲染完成步骤
  const renderCompleteStep = () => (
    <div className="space-y-6">
      <Card className="border-green-500/50">
        <CardContent className="pt-8 pb-6">
          <div className="text-center">
            <PartyPopper className="h-16 w-16 mx-auto text-green-500 mb-4" />
            <h2 className="text-2xl font-bold text-green-600 mb-2">
              {t('cluster.wizard.deployComplete')}
            </h2>
            <p className="text-muted-foreground">
              {t('cluster.wizard.deployCompleteDesc')}
            </p>
          </div>
        </CardContent>
      </Card>

      <div className="flex justify-center gap-4">
        <Button variant="outline" onClick={handleClose}>
          {t('common.close')}
        </Button>
        <Button onClick={handleComplete}>
          {t('cluster.wizard.viewCluster')}
        </Button>
      </div>
    </div>
  );

  // Render step content / 渲染步骤内容
  const renderStepContent = () => {
    switch (currentStep.id) {
      case 'basic':
        return renderBasicStep();
      case 'hosts':
        return renderHostsStep();
      case 'config':
        return renderConfigStep();
      case 'plugins':
        return renderPluginsStep();
      case 'precheck':
        return renderPrecheckStep();
      case 'deploy':
        return renderDeployStep();
      case 'complete':
        return renderCompleteStep();
      default:
        return null;
    }
  };

  return (
    <Dialog open={open} onOpenChange={handleClose}>
      <DialogContent className="!max-w-[95vw] w-[95vw] max-h-[95vh] h-[90vh] overflow-hidden flex flex-col">
        <DialogHeader>
          <DialogTitle>{t('cluster.wizard.title')}</DialogTitle>
          <DialogDescription>{t(currentStep.descKey)}</DialogDescription>
        </DialogHeader>

        {/* Step indicator / 步骤指示器 */}
        <div className="py-4">
          <div className="flex items-center justify-between mb-2">
            {WIZARD_STEPS.map((step, index) => {
              const StepIcon = step.icon;
              return (
                <div
                  key={step.id}
                  className={cn(
                    'flex items-center',
                    index < WIZARD_STEPS.length - 1 && 'flex-1'
                  )}
                >
                  <div
                    className={cn(
                      'flex items-center justify-center w-8 h-8 rounded-full border-2 transition-colors',
                      index < currentStepIndex && 'bg-primary border-primary text-primary-foreground',
                      index === currentStepIndex && 'border-primary text-primary',
                      index > currentStepIndex && 'border-muted-foreground/30 text-muted-foreground/50'
                    )}
                  >
                    {index < currentStepIndex ? (
                      <CheckCircle2 className="h-5 w-5" />
                    ) : (
                      <StepIcon className="h-4 w-4" />
                    )}
                  </div>
                  {index < WIZARD_STEPS.length - 1 && (
                    <div
                      className={cn(
                        'flex-1 h-0.5 mx-2',
                        index < currentStepIndex ? 'bg-primary' : 'bg-muted-foreground/30'
                      )}
                    />
                  )}
                </div>
              );
            })}
          </div>
          <Progress value={progress} className="h-1" />
        </div>

        {/* Step content / 步骤内容 */}
        <div className="flex-1 overflow-hidden min-h-0 py-4">
          {renderStepContent()}
        </div>

        {/* Footer buttons / 底部按钮 */}
        {currentStep.id !== 'complete' && (
          <div className="flex items-center justify-between pt-4 border-t">
            <Button
              variant="outline"
              onClick={handlePrevious}
              disabled={currentStepIndex === 0 || currentStep.id === 'deploy'}
            >
              <ChevronLeft className="h-4 w-4 mr-1" />
              {t('common.previous')}
            </Button>

            <div className="flex items-center gap-2">
              <Button variant="ghost" onClick={handleClose} disabled={deploying}>
                <X className="h-4 w-4 mr-1" />
                {t('common.cancel')}
              </Button>

              {currentStep.id !== 'deploy' && (
                <Button
                  onClick={handleNext}
                  disabled={!canProceed() || deploying}
                >
                  {currentStep.id === 'plugins' ? (
                    <>
                      {t('cluster.wizard.startDeploy')}
                      <PlayCircle className="h-4 w-4 ml-1" />
                    </>
                  ) : (
                    <>
                      {t('common.next')}
                      <ChevronRight className="h-4 w-4 ml-1" />
                    </>
                  )}
                </Button>
              )}
            </div>
          </div>
        )}
      </DialogContent>
    </Dialog>
  );
}

export default ClusterDeployWizard;
