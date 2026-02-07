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
import { HostInfo, HostType, HostStatus } from '@/lib/services/host/types';
import { DeploymentMode, NodeRole } from '@/lib/services/cluster/types';
import type { LocalPlugin } from '@/lib/services/plugin/types';
import type {
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
  { id: 'config', titleKey: 'cluster.wizard.config', descKey: 'cluster.wizard.configDesc', icon: Settings },
  { id: 'precheck', titleKey: 'cluster.wizard.precheck', descKey: 'cluster.wizard.precheckDesc', icon: CheckCircle2 },
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
  installDir: string; // Installation directory / 安装目录
  mirror: MirrorSource;
  // Port config / 端口配置
  clusterPort: number; // Hazelcast cluster port (default 5801) / Hazelcast 集群端口（默认 5801）
  httpPort: number; // HTTP API port (default 8080) / HTTP API 端口（默认 8080）
  workerPort: number; // Worker port for separated mode (default 5802) / 分离模式 Worker 端口（默认 5802）
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
  installDir: '/opt/seatunnel-2.3.12', // Default install dir with version / 默认安装目录带版本号
  mirror: 'aliyun',
  clusterPort: 5801, // Default Hazelcast cluster port / 默认 Hazelcast 集群端口
  httpPort: 8080, // Default HTTP API port / 默认 HTTP API 端口
  workerPort: 5802, // Default worker port for separated mode / 分离模式默认 Worker 端口
  jvm: {
    hybrid_heap_size: 3, // GB
    master_heap_size: 2, // GB
    worker_heap_size: 2, // GB
  },
  checkpoint: {
    storage_type: 'LOCAL_FILE',
    namespace: '/tmp/seatunnel/checkpoint/',
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
  // Detailed deploy steps state / 详细部署步骤状态
  const [deploySteps, setDeploySteps] = useState<{
    step: string;
    status: 'pending' | 'running' | 'success' | 'failed';
    message: string;
    hostName?: string;
    progress?: number;
  }[]>([]);

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
    if (open && currentStepIndex === 4) {
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
          (h) => h.host_type === HostType.BARE_METAL && h.status === HostStatus.CONNECTED
        );
        setHostsWithRole(
          availableHosts.map((host) => ({
            host,
            selected: false,
            // Hybrid mode uses MASTER_WORKER role for all nodes / 混合模式所有节点使用 MASTER_WORKER 角色
            role: config.deploymentMode === DeploymentMode.HYBRID ? NodeRole.MASTER_WORKER : NodeRole.WORKER,
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

    // Build ports list for precheck based on deployment mode
    // 根据部署模式构建预检查的端口列表
    const portsToCheck = config.deploymentMode === DeploymentMode.SEPARATED
      ? [config.clusterPort, config.workerPort, config.httpPort]
      : [config.clusterPort, config.httpPort];

    // Run precheck for each host in parallel / 并行运行每个主机的预检查
    // Pass install_dir and ports so precheck can verify the installation path and port availability
    // 传递 install_dir 和 ports 以便预检查可以验证安装路径和端口可用性
    const promises = selectedHosts.map(async (hostWithRole) => {
      try {
        const result = await services.installer.runPrecheck(hostWithRole.host.id, {
          install_dir: config.installDir,
          ports: portsToCheck,
        });
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
  }, [selectedHosts, config.installDir, config.clusterPort, config.httpPort, config.workerPort, config.deploymentMode]);

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

      // If version changes, update install dir with new version / 如果版本变化，更新安装目录中的版本号
      const newUpdates = { ...updates };
      if (updates.version !== undefined && updates.version !== prev.version) {
        // Replace version in install dir / 替换安装目录中的版本号
        const newInstallDir = prev.installDir.replace(prev.version, updates.version);
        // Only update if the path contains the old version / 只有当路径包含旧版本时才更新
        if (newInstallDir !== prev.installDir) {
          newUpdates.installDir = newInstallDir;
        } else {
          // If path doesn't contain version, use default pattern / 如果路径不包含版本，使用默认模式
          newUpdates.installDir = `/opt/seatunnel-${updates.version}`;
        }
      }

      return { ...prev, ...newUpdates };
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
        // Just need a valid version selected / 只需要选择有效版本
        // Package will be downloaded automatically if not available locally
        // 如果本地没有安装包会自动下载
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
  }, [currentStep.id, config, selectedHosts, deployStatus, allPrechecksPassed]);

  // Handle deploy / 处理部署
  const handleDeploy = useCallback(async () => {
    setDeploying(true);
    setDeployStatus('running');
    setDeployProgress(0);
    setDeployError(null);
    setDeploySteps([]);

    // Helper to update step status / 更新步骤状态的辅助函数
    const updateStep = (step: string, status: 'pending' | 'running' | 'success' | 'failed', message: string, hostName?: string, progress?: number) => {
      setDeploySteps(prev => {
        const existing = prev.find(s => s.step === step && s.hostName === hostName);
        if (existing) {
          return prev.map(s => s.step === step && s.hostName === hostName ? { ...s, status, message, progress } : s);
        }
        return [...prev, { step, status, message, hostName, progress }];
      });
    };

    try {
      let clusterId = createdClusterId;

      // Step 1: Create cluster (skip if already created) / 步骤1：创建集群（如果已创建则跳过）
      if (!clusterId) {
        updateStep('create_cluster', 'running', t('cluster.wizard.steps.creatingCluster'));
        setDeployProgress(10);
        const clusterResult = await services.cluster.createClusterSafe({
          name: config.name,
          description: config.description || undefined,
          deployment_mode: config.deploymentMode,
          version: config.version,
        });

        if (!clusterResult.success || !clusterResult.data) {
          updateStep('create_cluster', 'failed', clusterResult.error || 'Failed to create cluster');
          throw new Error(clusterResult.error || 'Failed to create cluster');
        }

        clusterId = clusterResult.data.id;
        setCreatedClusterId(clusterId);
        updateStep('create_cluster', 'success', t('cluster.wizard.steps.clusterCreated'));
      }
      setDeployProgress(20);

      // Step 2: Add nodes to cluster / 步骤2：添加节点到集群
      updateStep('add_nodes', 'running', t('cluster.wizard.steps.addingNodes'));
      for (let i = 0; i < selectedHosts.length; i++) {
        const hostWithRole = selectedHosts[i];
        updateStep('add_node', 'running', t('cluster.wizard.steps.addingNode'), hostWithRole.host.name);
        // Use addNodeSafe which handles duplicates gracefully
        // 使用 addNodeSafe，它会优雅地处理重复添加
        // Pass install_dir and port configuration / 传递安装目录和端口配置
        await services.cluster.addNodeSafe(clusterId, {
          host_id: hostWithRole.host.id,
          role: hostWithRole.role,
          install_dir: config.installDir,
          hazelcast_port: hostWithRole.role === 'worker' && config.deploymentMode === DeploymentMode.SEPARATED
            ? config.workerPort
            : config.clusterPort,
          api_port: hostWithRole.role === 'master' ? config.httpPort : undefined,
        });
        updateStep('add_node', 'success', t('cluster.wizard.steps.nodeAdded'), hostWithRole.host.name);
        setDeployProgress(20 + ((i + 1) / selectedHosts.length) * 30);
      }
      updateStep('add_nodes', 'success', t('cluster.wizard.steps.nodesAdded'));

      // Collect master addresses for cluster configuration
      // 收集 master 节点地址用于集群配置
      // For hybrid mode, all nodes act as both master and worker, so collect all addresses
      // 对于混合模式，所有节点同时作为 master 和 worker，所以收集所有地址
      const masterAddresses = config.deploymentMode === DeploymentMode.HYBRID
        ? selectedHosts.map(h => h.host.ip_address || '').filter(ip => ip !== '')
        : selectedHosts
            .filter(h => h.role === 'master')
            .map(h => h.host.ip_address || '')
            .filter(ip => ip !== '');

      // Collect worker addresses for separated mode
      // 收集分离模式的 worker 地址
      const workerAddresses = config.deploymentMode === DeploymentMode.SEPARATED
        ? selectedHosts
            .filter(h => h.role === 'worker')
            .map(h => h.host.ip_address || '')
            .filter(ip => ip !== '')
        : [];

      // Step 3: Install SeaTunnel on each host / 步骤3：在每台主机上安装 SeaTunnel
      for (let i = 0; i < selectedHosts.length; i++) {
        const hostWithRole = selectedHosts[i];

        // Start installation / 开始安装
        updateStep('install', 'running', t('cluster.wizard.steps.startingInstall'), hostWithRole.host.name, 0);
        const installResult = await services.installer.startInstallation(hostWithRole.host.id, {
          cluster_id: String(clusterId),
          version: config.version,
          install_dir: config.installDir,
          install_mode: 'online', // Always use online mode, backend handles local package check / 始终使用在线模式，后端会检查本地安装包
          mirror: config.mirror,
          deployment_mode: config.deploymentMode,
          node_role: hostWithRole.role,
          master_addresses: masterAddresses, // Pass all master addresses for hazelcast.yaml configuration / 传递所有 master 地址用于 hazelcast.yaml 配置
          worker_addresses: workerAddresses, // Pass worker addresses for separated mode / 传递分离模式的 worker 地址
          cluster_port: config.clusterPort, // Use configured cluster port / 使用配置的集群端口
          http_port: config.httpPort, // Use configured HTTP port / 使用配置的 HTTP 端口
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

        // Helper to update steps from backend response / 从后端响应更新步骤的辅助函数
        const updateStepsFromStatus = () => {
          if (status.steps && status.steps.length > 0) {
            // Show each step from backend with host name prefix
            // 显示后端返回的每个步骤，带主机名前缀
            for (const step of status.steps) {
              const stepKey = `${step.step}_${hostWithRole.host.id}`;
              const stepStatus = step.status === 'success' ? 'success'
                : step.status === 'failed' ? 'failed'
                : step.status === 'running' ? 'running'
                : 'pending';
              const stepMessage = step.message || step.name || step.step;
              updateStep(stepKey, stepStatus, stepMessage, hostWithRole.host.name, step.progress || 0);
            }
          }
        };

        while (status.status === 'running') {
          await new Promise((resolve) => setTimeout(resolve, 1000));
          status = await services.installer.getInstallationStatus(hostWithRole.host.id);

          // Update detailed steps from backend / 从后端更新详细步骤
          updateStepsFromStatus();

          if (!status.steps || status.steps.length === 0) {
            // Fallback to simple message / 回退到简单消息
            const stepMessage = status.message || status.current_step || t('cluster.wizard.steps.installing');
            updateStep('install', 'running', stepMessage, hostWithRole.host.name, status.progress || 0);
          }
        }

        // Update steps one final time after loop exits (handles immediate success case)
        // 循环退出后最后更新一次步骤（处理立即成功的情况）
        updateStepsFromStatus();

        if (status.status === 'failed') {
          updateStep('install', 'failed', status.error || t('cluster.wizard.steps.installFailed'), hostWithRole.host.name);
          throw new Error(`Installation failed on host ${hostWithRole.host.name}: ${status.error}`);
        }

        updateStep('install', 'success', t('cluster.wizard.steps.installComplete'), hostWithRole.host.name, 100);
        setDeployProgress(50 + ((i + 1) / selectedHosts.length) * 40);
      }

      // Step 4: Complete / 步骤4：完成
      setDeployProgress(100);
      setDeployStatus('success');
      toast.success(t('cluster.wizard.deploySuccess'));

      // Wait 1 second before auto advancing to let user see the final status
      // 等待 1 秒后再自动跳转，让用户看到最终状态
      await new Promise((resolve) => setTimeout(resolve, 1000));

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
    <div className="h-full flex flex-col overflow-hidden">
      <ScrollArea className="flex-1 min-h-0 pr-4">
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

      {/* Version and Install Directory / 版本和安装目录 */}
      <div className="grid grid-cols-2 gap-4">
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
        <div className="space-y-2">
          <Label htmlFor="installDir">{t('installer.installDirLabel')}</Label>
          <Input
            id="installDir"
            value={config.installDir}
            onChange={(e) => updateConfig({ installDir: e.target.value })}
            placeholder="/opt/seatunnel-2.3.12"
          />
        </div>
      </div>
      <p className="text-xs text-muted-foreground">
        {t('installer.installDirDesc')}
      </p>
        </div>
      </ScrollArea>
    </div>
  );

  // Render hosts selection step / 渲染主机选择步骤
  const renderHostsStep = () => (
    <div className="h-full flex flex-col overflow-hidden">
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

      <ScrollArea className="flex-1 min-h-0 pr-4 mt-4">
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
  const renderConfigStep = () => {
    // Check if current version package is available locally
    // 检查当前版本的安装包是否在本地可用
    const currentPackage = localPackages.find((pkg) => pkg.version === config.version);
    const isPackageLocal = !!currentPackage;

    return (
    <div className="h-full flex flex-col overflow-hidden">
      <ScrollArea className="flex-1 min-h-0 pr-4">
        <div className="space-y-4">
        {/* Package Status / 安装包状态 */}
        <Card>
          <CardHeader className="pb-3">
            <CardTitle className="text-base">{t('installer.packageStatus')}</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            {/* Package status display / 安装包状态显示 */}
            {packagesLoading ? (
              <div className="flex items-center justify-center py-4">
                <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
              </div>
            ) : isPackageLocal ? (
              /* Package is available locally / 安装包已在本地 */
              <div className="p-4 bg-green-50 dark:bg-green-900/20 rounded-lg">
                <div className="flex items-center gap-3">
                  <CheckCircle2 className="h-5 w-5 text-green-600" />
                  <div className="flex-1">
                    <p className="text-sm font-medium text-green-700 dark:text-green-300">
                      {t('cluster.wizard.packageReady')}
                    </p>
                    <p className="text-xs text-green-600 dark:text-green-400 mt-1">
                      {currentPackage.file_name} ({(currentPackage.file_size / 1024 / 1024).toFixed(1)} MB)
                    </p>
                  </div>
                </div>
              </div>
            ) : (
              /* Package not available locally / 安装包不在本地 */
              <div className="space-y-4">
                <div className="p-4 bg-yellow-50 dark:bg-yellow-900/20 rounded-lg">
                  <div className="flex items-start gap-3">
                    <AlertTriangle className="h-5 w-5 text-yellow-600 mt-0.5" />
                    <div className="flex-1">
                      <p className="text-sm font-medium text-yellow-700 dark:text-yellow-300">
                        {t('cluster.wizard.packageNotFound')}
                      </p>
                      <p className="text-xs text-yellow-600 dark:text-yellow-400 mt-1">
                        {t('cluster.wizard.packageNotFoundDesc', { version: config.version })}
                      </p>
                    </div>
                  </div>
                </div>

                {/* Mirror source selection for download / 下载镜像源选择 */}
                <div className="space-y-2">
                  <Label>{t('installer.mirrorSource')}</Label>
                  <div className="flex items-center gap-2">
                    <Select
                      value={config.mirror}
                      onValueChange={(value: MirrorSource) => updateConfig({ mirror: value })}
                    >
                      <SelectTrigger className="flex-1">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="aliyun">{t('installer.mirrors.aliyun')}</SelectItem>
                        <SelectItem value="huaweicloud">{t('installer.mirrors.huaweicloud')}</SelectItem>
                        <SelectItem value="apache">{t('installer.mirrors.apache')}</SelectItem>
                      </SelectContent>
                    </Select>
                  </div>
                  <p className="text-xs text-muted-foreground">
                    {t('cluster.wizard.mirrorHint')}
                  </p>
                </div>

                {/* Manual upload option / 手动上传选项 */}
                <div className="flex items-center justify-between p-3 bg-muted/50 rounded-lg">
                  <div>
                    <p className="text-sm">{t('cluster.wizard.manualUpload')}</p>
                    <p className="text-xs text-muted-foreground">{t('cluster.wizard.manualUploadDesc')}</p>
                  </div>
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => window.open('/packages', '_blank')}
                  >
                    <Package className="h-4 w-4 mr-2" />
                    {t('cluster.wizard.goToPackageManagement')}
                  </Button>
                </div>
              </div>
            )}
          </CardContent>
        </Card>

        {/* JVM Config & Checkpoint Config / JVM 配置和检查点配置 */}
        <div className="grid grid-cols-2 gap-4">
          {/* Port Config / 端口配置 */}
          <Card>
            <CardHeader className="pb-2">
              <CardTitle className="text-base">{t('cluster.wizard.portConfig')}</CardTitle>
            </CardHeader>
            <CardContent className="space-y-3">
              <div className="space-y-2">
                <Label>{t('cluster.wizard.clusterPort')}</Label>
                <Input
                  type="number"
                  value={config.clusterPort}
                  onChange={(e) => updateConfig({ clusterPort: parseInt(e.target.value) || 5801 })}
                  min={1024}
                  max={65535}
                  placeholder="5801"
                />
                <p className="text-xs text-muted-foreground">{t('cluster.wizard.clusterPortDesc')}</p>
              </div>
              <div className="space-y-2">
                <Label>{t('cluster.wizard.httpPort')}</Label>
                <Input
                  type="number"
                  value={config.httpPort}
                  onChange={(e) => updateConfig({ httpPort: parseInt(e.target.value) || 8080 })}
                  min={1024}
                  max={65535}
                  placeholder="8080"
                />
                <p className="text-xs text-muted-foreground">{t('cluster.wizard.httpPortDesc')}</p>
              </div>
              {config.deploymentMode === DeploymentMode.SEPARATED && (
                <div className="space-y-2">
                  <Label>{t('cluster.wizard.workerPort')}</Label>
                  <Input
                    type="number"
                    value={config.workerPort}
                    onChange={(e) => updateConfig({ workerPort: parseInt(e.target.value) || 5802 })}
                    min={1024}
                    max={65535}
                    placeholder="5802"
                  />
                  <p className="text-xs text-muted-foreground">{t('cluster.wizard.workerPortDesc')}</p>
                </div>
              )}
            </CardContent>
          </Card>

          {/* JVM Config / JVM 配置 */}
          <Card>
            <CardHeader className="pb-2">
              <CardTitle className="text-base">{t('installer.jvmConfig')}</CardTitle>
            </CardHeader>
            <CardContent>
              {config.deploymentMode === DeploymentMode.HYBRID ? (
                <div className="space-y-2">
                  <Label>{t('installer.hybridHeapSize')} (GB)</Label>
                  <Input
                    type="number"
                    value={config.jvm.hybrid_heap_size}
                    onChange={(e) =>
                      updateConfig({
                        jvm: { ...config.jvm, hybrid_heap_size: parseInt(e.target.value) || 0 },
                      })
                    }
                    min={1}
                    max={64}
                    step={1}
                  />
                </div>
              ) : (
                <div className="space-y-3">
                  <div className="space-y-2">
                    <Label>{t('installer.masterHeapSize')} (GB)</Label>
                    <Input
                      type="number"
                      value={config.jvm.master_heap_size}
                      onChange={(e) =>
                        updateConfig({
                          jvm: { ...config.jvm, master_heap_size: parseInt(e.target.value) || 0 },
                        })
                      }
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
                      onChange={(e) =>
                        updateConfig({
                          jvm: { ...config.jvm, worker_heap_size: parseInt(e.target.value) || 0 },
                        })
                      }
                      min={1}
                      max={64}
                      step={1}
                    />
                  </div>
                </div>
              )}
            </CardContent>
          </Card>
        </div>

        {/* Checkpoint Config / 检查点配置 */}
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-base">{t('installer.checkpointConfig')}</CardTitle>
          </CardHeader>
          <CardContent className="space-y-3">
            <div className="grid grid-cols-2 gap-4">
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
                <Label>{t('installer.storagePath')}</Label>
                <Input
                  value={config.checkpoint.namespace}
                  onChange={(e) =>
                    updateConfig({
                      checkpoint: { ...config.checkpoint, namespace: e.target.value },
                    })
                  }
                  placeholder={
                    config.checkpoint.storage_type === 'LOCAL_FILE'
                      ? '/tmp/seatunnel/checkpoint/'
                      : '/seatunnel/checkpoint/'
                  }
                />
                <p className="text-xs text-muted-foreground">{t('installer.storagePathHint')}</p>
              </div>
            </div>

            {/* HDFS Config / HDFS 配置 */}
            {config.checkpoint.storage_type === 'HDFS' && (
              <div className="space-y-4">
                {/* HA Mode Toggle / HA 模式开关 */}
                <div className="flex items-center space-x-2">
                  <Checkbox
                    id="hdfs-ha-enabled"
                    checked={config.checkpoint.hdfs_ha_enabled || false}
                    onCheckedChange={(checked) =>
                      updateConfig({
                        checkpoint: { ...config.checkpoint, hdfs_ha_enabled: checked === true },
                      })
                    }
                  />
                  <Label htmlFor="hdfs-ha-enabled">{t('installer.hdfsHAMode')}</Label>
                </div>

                {/* Standard HDFS Config / 标准 HDFS 配置 */}
                {!config.checkpoint.hdfs_ha_enabled && (
                  <div className="grid grid-cols-2 gap-4">
                    <div className="space-y-2">
                      <Label>{t('installer.hdfsNameNodeHost')}</Label>
                      <Input
                        value={config.checkpoint.hdfs_namenode_host || ''}
                        onChange={(e) =>
                          updateConfig({
                            checkpoint: { ...config.checkpoint, hdfs_namenode_host: e.target.value },
                          })
                        }
                        placeholder="namenode.example.com"
                      />
                    </div>
                    <div className="space-y-2">
                      <Label>{t('installer.hdfsNameNodePort')}</Label>
                      <Input
                        type="number"
                        value={config.checkpoint.hdfs_namenode_port || ''}
                        onChange={(e) =>
                          updateConfig({
                            checkpoint: { ...config.checkpoint, hdfs_namenode_port: parseInt(e.target.value) || 0 },
                          })
                        }
                        placeholder="8020"
                      />
                    </div>
                  </div>
                )}

                {/* HDFS HA Config / HDFS HA 配置 */}
                {config.checkpoint.hdfs_ha_enabled && (
                  <div className="space-y-3 p-3 border rounded-md bg-muted/30">
                    <div className="grid grid-cols-2 gap-4">
                      <div className="space-y-2">
                        <Label>{t('installer.hdfsNameServices')}</Label>
                        <Input
                          value={config.checkpoint.hdfs_name_services || ''}
                          onChange={(e) =>
                            updateConfig({
                              checkpoint: { ...config.checkpoint, hdfs_name_services: e.target.value },
                            })
                          }
                          placeholder="mycluster"
                        />
                      </div>
                      <div className="space-y-2">
                        <Label>{t('installer.hdfsHANamenodes')}</Label>
                        <Input
                          value={config.checkpoint.hdfs_ha_namenodes || ''}
                          onChange={(e) =>
                            updateConfig({
                              checkpoint: { ...config.checkpoint, hdfs_ha_namenodes: e.target.value },
                            })
                          }
                          placeholder="nn1,nn2"
                        />
                      </div>
                    </div>
                    <div className="grid grid-cols-2 gap-4">
                      <div className="space-y-2">
                        <Label>{t('installer.hdfsNamenodeRPCAddress1')}</Label>
                        <Input
                          value={config.checkpoint.hdfs_namenode_rpc_address_1 || ''}
                          onChange={(e) =>
                            updateConfig({
                              checkpoint: { ...config.checkpoint, hdfs_namenode_rpc_address_1: e.target.value },
                            })
                          }
                          placeholder="nn1-host:8020"
                        />
                      </div>
                      <div className="space-y-2">
                        <Label>{t('installer.hdfsNamenodeRPCAddress2')}</Label>
                        <Input
                          value={config.checkpoint.hdfs_namenode_rpc_address_2 || ''}
                          onChange={(e) =>
                            updateConfig({
                              checkpoint: { ...config.checkpoint, hdfs_namenode_rpc_address_2: e.target.value },
                            })
                          }
                          placeholder="nn2-host:8020"
                        />
                      </div>
                    </div>
                  </div>
                )}

                {/* Kerberos Config / Kerberos 配置 */}
                <div className="flex items-center space-x-2">
                  <Checkbox
                    id="hdfs-kerberos"
                    checked={config.checkpoint.kerberos_principal !== undefined}
                    onCheckedChange={(checked) => {
                      if (checked) {
                        // Enable Kerberos - set empty strings to show the form
                        updateConfig({
                          checkpoint: {
                            ...config.checkpoint,
                            kerberos_principal: '',
                            kerberos_keytab_file_path: '',
                          },
                        });
                      } else {
                        // Disable Kerberos - remove the fields
                        updateConfig({
                          checkpoint: {
                            ...config.checkpoint,
                            kerberos_principal: undefined,
                            kerberos_keytab_file_path: undefined,
                          },
                        });
                      }
                    }}
                  />
                  <Label htmlFor="hdfs-kerberos">{t('installer.hdfsKerberos')}</Label>
                </div>

                {config.checkpoint.kerberos_principal !== undefined && (
                  <div className="grid grid-cols-2 gap-4 p-3 border rounded-md bg-muted/30">
                    <div className="space-y-2">
                      <Label>{t('installer.kerberosPrincipal')}</Label>
                      <Input
                        value={config.checkpoint.kerberos_principal || ''}
                        onChange={(e) =>
                          updateConfig({
                            checkpoint: { ...config.checkpoint, kerberos_principal: e.target.value },
                          })
                        }
                        placeholder="hdfs/namenode@EXAMPLE.COM"
                      />
                    </div>
                    <div className="space-y-2">
                      <Label>{t('installer.kerberosKeytabPath')}</Label>
                      <Input
                        value={config.checkpoint.kerberos_keytab_file_path || ''}
                        onChange={(e) =>
                          updateConfig({
                            checkpoint: { ...config.checkpoint, kerberos_keytab_file_path: e.target.value },
                          })
                        }
                        placeholder="/etc/security/keytabs/hdfs.keytab"
                      />
                    </div>
                  </div>
                )}
              </div>
            )}

            {/* OSS/S3 Config / OSS/S3 配置 */}
            {(config.checkpoint.storage_type === 'OSS' || config.checkpoint.storage_type === 'S3') && (
              <div className="space-y-3">
                <div className="grid grid-cols-2 gap-4">
                  <div className="space-y-2">
                    <Label>{t('installer.endpoint')}</Label>
                    <Input
                      value={config.checkpoint.storage_endpoint || ''}
                      onChange={(e) =>
                        updateConfig({
                          checkpoint: { ...config.checkpoint, storage_endpoint: e.target.value },
                        })
                      }
                      placeholder={config.checkpoint.storage_type === 'OSS' ? 'oss-cn-hangzhou.aliyuncs.com' : 's3.amazonaws.com'}
                    />
                  </div>
                  <div className="space-y-2">
                    <Label>{t('installer.bucket')}</Label>
                    <Input
                      value={config.checkpoint.storage_bucket || ''}
                      onChange={(e) =>
                        updateConfig({
                          checkpoint: { ...config.checkpoint, storage_bucket: e.target.value },
                        })
                      }
                      placeholder="my-checkpoint-bucket"
                    />
                  </div>
                </div>
                <div className="grid grid-cols-2 gap-4">
                  <div className="space-y-2">
                    <Label>{t('installer.accessKey')}</Label>
                    <Input
                      type="password"
                      value={config.checkpoint.storage_access_key || ''}
                      onChange={(e) =>
                        updateConfig({
                          checkpoint: { ...config.checkpoint, storage_access_key: e.target.value },
                        })
                      }
                      placeholder="Access Key ID"
                    />
                  </div>
                  <div className="space-y-2">
                    <Label>{t('installer.secretKey')}</Label>
                    <Input
                      type="password"
                      value={config.checkpoint.storage_secret_key || ''}
                      onChange={(e) =>
                        updateConfig({
                          checkpoint: { ...config.checkpoint, storage_secret_key: e.target.value },
                        })
                      }
                      placeholder="Secret Access Key"
                    />
                  </div>
                </div>
              </div>
            )}
          </CardContent>
        </Card>
      </div>
    </ScrollArea>
    </div>
  );
  };

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
      <div className="h-full flex flex-col overflow-hidden">
        {/* Header with count / 标题和计数 */}
        <div className="flex items-center justify-between mb-4">
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
            <ScrollArea className="flex-1 min-h-0 pr-4 mt-4">
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
                                  {Object.entries(item.details).map(([key, value]) => {
                                    // Skip output field as it contains raw JSON / 跳过 output 字段因为它包含原始 JSON
                                    if (key === 'output') {return null;}
                                    // Format the value nicely / 格式化值
                                    let displayValue: string;
                                    if (typeof value === 'string') {
                                      displayValue = value;
                                    } else if (Array.isArray(value)) {
                                      displayValue = value.join(', ');
                                    } else if (typeof value === 'object' && value !== null) {
                                      displayValue = JSON.stringify(value);
                                    } else {
                                      displayValue = String(value);
                                    }
                                    return (
                                      <div key={key} className="flex gap-1">
                                        <span className="font-medium">{key.replace(/_/g, ' ')}:</span>
                                        <span>{displayValue}</span>
                                      </div>
                                    );
                                  })}
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
    <div className="h-full flex flex-col overflow-hidden">
      <ScrollArea className="flex-1 min-h-0 pr-4">
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
                      setDeploySteps([]);
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

          {/* Detailed steps / 详细步骤 */}
          {deploySteps.length > 0 && (
            <div className="mt-6 space-y-2">
              <h4 className="text-sm font-medium mb-3">{t('cluster.wizard.deploySteps')}</h4>
              <div className="max-h-[300px] overflow-y-auto pr-2">
                <div className="space-y-2">
                  {deploySteps.map((step, index) => (
                    <div
                      key={`${step.step}-${step.hostName || index}`}
                      className={cn(
                        'flex items-center gap-3 p-2 rounded-lg text-sm',
                        step.status === 'running' && 'bg-blue-50 dark:bg-blue-950',
                        step.status === 'success' && 'bg-green-50 dark:bg-green-950',
                        step.status === 'failed' && 'bg-red-50 dark:bg-red-950',
                        step.status === 'pending' && 'bg-muted'
                      )}
                    >
                      {step.status === 'running' && (
                        <Loader2 className="h-4 w-4 animate-spin text-blue-500 flex-shrink-0" />
                      )}
                      {step.status === 'success' && (
                        <CheckCircle2 className="h-4 w-4 text-green-500 flex-shrink-0" />
                      )}
                      {step.status === 'failed' && (
                        <X className="h-4 w-4 text-red-500 flex-shrink-0" />
                      )}
                      {step.status === 'pending' && (
                        <div className="h-4 w-4 rounded-full border-2 border-muted-foreground flex-shrink-0" />
                      )}
                      <div className="flex-1 min-w-0">
                        <div className="flex items-center gap-2">
                          {step.hostName && (
                            <Badge variant="outline" className="text-xs">
                              {step.hostName}
                            </Badge>
                          )}
                          <span className="truncate">{step.message}</span>
                        </div>
                        {step.status === 'running' && step.progress !== undefined && (
                          <Progress value={step.progress} className="h-1 mt-1" />
                        )}
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            </div>
          )}
        </CardContent>
      </Card>
        </div>
      </ScrollArea>
    </div>
  );

  // Render complete step / 渲染完成步骤
  const renderCompleteStep = () => (
    <div className="h-full flex flex-col overflow-hidden">
      <ScrollArea className="flex-1 min-h-0 pr-4">
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
      </ScrollArea>
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
