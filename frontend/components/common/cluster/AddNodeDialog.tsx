/*
 * Licensed to the Apache Software Foundation (ASF) under one or more
 * contributor license agreements.  See the NOTICE file distributed with
 * this work for additional information regarding copyright ownership.
 * The ASF licenses this file to You under the Apache License, Version 2.0
 * (the "License"); you may not use this file except in compliance with
 * the License.  You may obtain a copy of the License at
 *
 *    http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

'use client';

import {useEffect, useMemo, useState} from 'react';
import {useTranslations} from 'next-intl';
import {toast} from 'sonner';
import {Cpu, Loader2, Search, Server} from 'lucide-react';

import services from '@/lib/services';
import {Button} from '@/components/ui/button';
import {Checkbox} from '@/components/ui/checkbox';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import {Input} from '@/components/ui/input';
import {Label} from '@/components/ui/label';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import {Switch} from '@/components/ui/switch';
import {AgentStatus, HostInfo} from '@/lib/services/host/types';
import {DiscoveredProcess} from '@/lib/services/discovery/discovery.service';
import {
  AddNodeEntryRequest,
  ClusterConfig,
  DefaultPorts,
  DeploymentMode,
  NodeInfo,
  NodeRole,
  PrecheckCheckItem,
  PrecheckResult,
  buildNodeJVMOverride,
  getClusterPortDefaultsForRole,
  getClusterJVMConfig,
  getClusterJVMValueForRole,
} from '@/lib/services/cluster/types';
import {buildResolvedClusterConfigFromConfigs} from './nodeConfigDefaults';

interface AddNodeDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  clusterId: number;
  deploymentMode: DeploymentMode;
  clusterConfig?: ClusterConfig;
  clusterInstallDir?: string;
  existingNodes?: NodeInfo[];
  onSuccess: () => void;
}

interface RoleFormState {
  hazelcastPort: number;
  apiPort: number;
  workerPort: number;
  jvmOverrideEnabled: boolean;
  jvmHeapSize: number;
}

interface RolePrecheckResult {
  role: NodeRole;
  result: PrecheckResult;
}

const FALLBACK_JVM_HEAP: Record<NodeRole, number> = {
  [NodeRole.MASTER]: 2,
  [NodeRole.WORKER]: 2,
  [NodeRole.MASTER_WORKER]: 2,
};

function getRoleTranslationKey(role: NodeRole): 'master' | 'worker' | 'masterWorker' {
  if (role === NodeRole.MASTER) {
    return 'master';
  }
  if (role === NodeRole.WORKER) {
    return 'worker';
  }
  return 'masterWorker';
}

function getRoleConfigTitleKey(
  role: NodeRole,
): 'masterNodeConfig' | 'workerNodeConfig' | 'masterWorkerNodeConfig' {
  if (role === NodeRole.MASTER) {
    return 'masterNodeConfig';
  }
  if (role === NodeRole.WORKER) {
    return 'workerNodeConfig';
  }
  return 'masterWorkerNodeConfig';
}

function getPrimaryPortLabelKey(
  role: NodeRole,
): 'hazelcastMasterPort' | 'hazelcastWorkerPort' {
  return role === NodeRole.WORKER ? 'hazelcastWorkerPort' : 'hazelcastMasterPort';
}

function getPrimaryPortRequiredMessageKey(
  role: NodeRole,
): 'cluster.hazelcastPortRequired' | 'cluster.workerPortRequired' {
  return role === NodeRole.WORKER ? 'cluster.workerPortRequired' : 'cluster.hazelcastPortRequired';
}

function resolveDefaultInstallDir(
  clusterInstallDir?: string,
  existingNodes?: NodeInfo[],
): string {
  if (clusterInstallDir?.trim()) {
    return clusterInstallDir.trim();
  }
  const existingInstallDir = existingNodes?.find((node) => node.install_dir?.trim())?.install_dir;
  if (existingInstallDir?.trim()) {
    return existingInstallDir.trim();
  }
  return '/opt/seatunnel';
}

function getValidPortOrUndefined(value?: number): number | undefined {
  return value && value > 0 ? value : undefined;
}

function buildDefaultRoleForm(
  role: NodeRole,
  clusterConfig?: ClusterConfig,
  existingNodes?: NodeInfo[],
): RoleFormState {
  const clusterPortDefaults = getClusterPortDefaultsForRole(role, clusterConfig);
  const existingRoleNode = existingNodes?.find((node) => node.role === role);

  return {
    hazelcastPort:
      clusterPortDefaults.hazelcastPort ??
      getValidPortOrUndefined(existingRoleNode?.hazelcast_port) ??
      (role === NodeRole.WORKER
        ? DefaultPorts.WORKER_HAZELCAST
        : DefaultPorts.MASTER_HAZELCAST),
    apiPort:
      role === NodeRole.WORKER
        ? 0
        : clusterPortDefaults.apiPort ??
          getValidPortOrUndefined(existingRoleNode?.api_port) ??
          DefaultPorts.MASTER_API,
    workerPort:
      role === NodeRole.MASTER_WORKER
        ? clusterPortDefaults.workerPort ??
          getValidPortOrUndefined(existingRoleNode?.worker_port) ??
          DefaultPorts.WORKER_HAZELCAST
        : 0,
    jvmOverrideEnabled: false,
    jvmHeapSize:
      getClusterJVMValueForRole(role, clusterConfig) ?? FALLBACK_JVM_HEAP[role],
  };
}

export function AddNodeDialog({
  open,
  onOpenChange,
  clusterId,
  deploymentMode,
  clusterConfig,
  clusterInstallDir,
  existingNodes = [],
  onSuccess,
}: AddNodeDialogProps) {
  const t = useTranslations();
  const isHybrid = deploymentMode === DeploymentMode.HYBRID;
  const [resolvedClusterConfig, setResolvedClusterConfig] = useState<ClusterConfig | undefined>(
    clusterConfig,
  );
  const defaultInstallDir = useMemo(
    () => resolveDefaultInstallDir(clusterInstallDir, existingNodes),
    [clusterInstallDir, existingNodes],
  );

  const [loading, setLoading] = useState(false);
  const [loadingHosts, setLoadingHosts] = useState(false);
  const [precheckLoading, setPrecheckLoading] = useState(false);
  const [discoveryLoading, setDiscoveryLoading] = useState(false);

  const [hostId, setHostId] = useState('');
  const [installDir, setInstallDir] = useState(defaultInstallDir);
  const [selectedMaster, setSelectedMaster] = useState(false);
  const [selectedWorker, setSelectedWorker] = useState(false);
  const [availableHosts, setAvailableHosts] = useState<HostInfo[]>([]);
  const [discoveredProcesses, setDiscoveredProcesses] = useState<DiscoveredProcess[]>([]);
  const [selectedProcess, setSelectedProcess] = useState('');
  const [precheckResults, setPrecheckResults] = useState<RolePrecheckResult[]>([]);

  const [hybridForm, setHybridForm] = useState<RoleFormState>(
    buildDefaultRoleForm(NodeRole.MASTER_WORKER, clusterConfig, existingNodes),
  );
  const [masterForm, setMasterForm] = useState<RoleFormState>(
    buildDefaultRoleForm(NodeRole.MASTER, clusterConfig, existingNodes),
  );
  const [workerForm, setWorkerForm] = useState<RoleFormState>(
    buildDefaultRoleForm(NodeRole.WORKER, clusterConfig, existingNodes),
  );

  const selectedRoles = useMemo(() => {
    if (isHybrid) {
      return [NodeRole.MASTER_WORKER];
    }
    return [
      ...(selectedMaster ? [NodeRole.MASTER] : []),
      ...(selectedWorker ? [NodeRole.WORKER] : []),
    ];
  }, [isHybrid, selectedMaster, selectedWorker]);

  const resetForm = (effectiveClusterConfig?: ClusterConfig) => {
    setHostId('');
    setInstallDir(defaultInstallDir);
    setSelectedMaster(false);
    setSelectedWorker(false);
    setPrecheckResults([]);
    setDiscoveredProcesses([]);
    setSelectedProcess('');
    setHybridForm(
      buildDefaultRoleForm(NodeRole.MASTER_WORKER, effectiveClusterConfig, existingNodes),
    );
    setMasterForm(buildDefaultRoleForm(NodeRole.MASTER, effectiveClusterConfig, existingNodes));
    setWorkerForm(buildDefaultRoleForm(NodeRole.WORKER, effectiveClusterConfig, existingNodes));
  };

  useEffect(() => {
    if (!open) {
      return;
    }
    let cancelled = false;

    const initialize = async () => {
      let effectiveClusterConfig = clusterConfig;
      if (!getClusterJVMConfig(clusterConfig)) {
        const configsResult = await services.config.getClusterConfigsSafe(clusterId);
        if (configsResult.success && configsResult.data) {
          effectiveClusterConfig = buildResolvedClusterConfigFromConfigs(
            clusterConfig,
            deploymentMode,
            configsResult.data,
          );
        }
      }

      if (cancelled) {
        return;
      }

      setResolvedClusterConfig(effectiveClusterConfig);
      resetForm(effectiveClusterConfig);
      await loadAvailableHosts();
    };

    void initialize();

    return () => {
      cancelled = true;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, clusterId, clusterConfig, deploymentMode, defaultInstallDir, existingNodes]);

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

  const getRoleForm = (role: NodeRole) => {
    switch (role) {
      case NodeRole.MASTER:
        return masterForm;
      case NodeRole.WORKER:
        return workerForm;
      default:
        return hybridForm;
    }
  };

  const updateRoleForm = (role: NodeRole, patch: Partial<RoleFormState>) => {
    if (role === NodeRole.MASTER) {
      setMasterForm((prev) => ({...prev, ...patch}));
      return;
    }
    if (role === NodeRole.WORKER) {
      setWorkerForm((prev) => ({...prev, ...patch}));
      return;
    }
    setHybridForm((prev) => ({...prev, ...patch}));
  };

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

  const handleProcessSelect = (processKey: string) => {
    setSelectedProcess(processKey);
    const process = discoveredProcesses.find(
      (item) => `${item.pid}-${item.install_dir}` === processKey,
    );
    if (!process) {
      return;
    }

    setInstallDir(process.install_dir);
    if (!isHybrid) {
      if (process.role === 'master') {
        setSelectedMaster(true);
      }
      if (process.role === 'worker') {
        setSelectedWorker(true);
      }
    }
    toast.success(t('discovery.processSelected'));
  };

  const validateBeforeSubmit = () => {
    if (!hostId) {
      toast.error(t('cluster.hostRequired'));
      return false;
    }
    if (!installDir.trim()) {
      toast.error(t('cluster.installDirRequired'));
      return false;
    }
    if (selectedRoles.length === 0) {
      toast.error(t('cluster.selectAtLeastOneRole'));
      return false;
    }

    for (const role of selectedRoles) {
      const form = getRoleForm(role);
      if (!form.hazelcastPort || form.hazelcastPort <= 0) {
        toast.error(t(getPrimaryPortRequiredMessageKey(role)));
        return false;
      }
      if (form.jvmOverrideEnabled && (!form.jvmHeapSize || form.jvmHeapSize <= 0)) {
        toast.error(t('cluster.jvmHeapSizeRequired'));
        return false;
      }
    }
    return true;
  };

  const buildEntries = (): AddNodeEntryRequest[] => {
    return selectedRoles.map((role) => {
      const form = getRoleForm(role);
      return {
        role,
        hazelcast_port: form.hazelcastPort,
        api_port: role === NodeRole.WORKER ? undefined : form.apiPort || undefined,
        overrides: form.jvmOverrideEnabled
          ? buildNodeJVMOverride(role, form.jvmHeapSize)
          : undefined,
      };
    });
  };

  const handlePrecheck = async () => {
    if (!validateBeforeSubmit()) {
      return;
    }

    const parsedHostId = parseInt(hostId, 10);
    const entries = buildEntries();

    setPrecheckLoading(true);
    setPrecheckResults([]);
    try {
      const results = await Promise.all(
        entries.map(async (entry) => {
          const response = await services.cluster.precheckNodeSafe(clusterId, {
            host_id: parsedHostId,
            role: entry.role,
            install_dir: installDir.trim(),
            hazelcast_port: entry.hazelcast_port || 0,
            api_port: entry.api_port,
            worker_port: entry.worker_port,
          });
          if (!response.success || !response.data) {
            throw new Error(response.error || t('cluster.precheckError'));
          }
          return {role: entry.role, result: response.data};
        }),
      );

      setPrecheckResults(results);
      if (results.every((item) => item.result.success)) {
        toast.success(t('cluster.precheckPassed'));
      } else {
        toast.warning(t('cluster.precheckFailed'));
      }
    } catch (error) {
      toast.error(
        error instanceof Error ? error.message : t('cluster.precheckError'),
      );
    } finally {
      setPrecheckLoading(false);
    }
  };

  const handleSubmit = async () => {
    if (!validateBeforeSubmit()) {
      return;
    }

    setLoading(true);
    try {
      const result = await services.cluster.addNodesSafe(clusterId, {
        host_id: parseInt(hostId, 10),
        install_dir: installDir.trim(),
        entries: buildEntries(),
      });

      if (!result.success) {
        toast.error(result.error || t('cluster.addNodeError'));
        return;
      }

      resetForm();
      onSuccess();
    } finally {
      setLoading(false);
    }
  };

  const handleClose = (nextOpen: boolean) => {
    if (!nextOpen) {
      resetForm();
    }
    onOpenChange(nextOpen);
  };

  const renderStatusIcon = (status: string) => {
    switch (status) {
      case 'passed':
        return <span className='text-green-600'>●</span>;
      case 'failed':
        return <span className='text-red-600'>●</span>;
      default:
        return <span className='text-amber-500'>●</span>;
    }
  };

  const renderRoleConfig = (role: NodeRole) => {
    const form = getRoleForm(role);
    const roleLabel = t(`cluster.roles.${getRoleTranslationKey(role)}`);
    const defaultHeap = getClusterJVMValueForRole(role, resolvedClusterConfig);

    return (
      <div key={role} className='rounded-lg border p-4 space-y-4'>
        <div className='space-y-1'>
          <h4 className='font-medium'>{t(`cluster.${getRoleConfigTitleKey(role)}`)}</h4>
          <p className='text-xs text-muted-foreground'>
            {role === NodeRole.MASTER_WORKER
              ? t('cluster.hybridRoleFixedDescription')
              : t('cluster.roleConfigDescription', {role: roleLabel})}
          </p>
        </div>

        <div className='grid gap-4 md:grid-cols-3'>
          <div className='space-y-1'>
            <Label className='text-xs'>
              {t(`cluster.${getPrimaryPortLabelKey(role)}`)}{' '}
              <span className='text-destructive'>*</span>
            </Label>
            <Input
              type='number'
              value={form.hazelcastPort}
              onChange={(e) =>
                updateRoleForm(role, {
                  hazelcastPort: parseInt(e.target.value, 10) || 0,
                })
              }
            />
          </div>

          {role !== NodeRole.WORKER && (
            <div className='space-y-1'>
              <Label className='text-xs'>
                {t('cluster.apiPort')} <span className='text-muted-foreground'>({t('common.optional')})</span>
              </Label>
              <Input
                type='number'
                value={form.apiPort || ''}
                onChange={(e) =>
                  updateRoleForm(role, {
                    apiPort: parseInt(e.target.value, 10) || 0,
                  })
                }
              />
            </div>
          )}
        </div>

        <div className='space-y-3 rounded-md bg-muted/40 p-3'>
          <div className='flex items-center justify-between gap-4'>
            <div className='space-y-1'>
              <Label>{t('cluster.jvmOverrideTitle')}</Label>
              <p className='text-xs text-muted-foreground'>
                {form.jvmOverrideEnabled
                  ? t('cluster.jvmOverrideEnabledHint')
                  : defaultHeap
                    ? t('cluster.jvmInheritDefault', {value: defaultHeap})
                    : t('cluster.jvmNoClusterDefault')}
              </p>
            </div>
            <Switch
              checked={form.jvmOverrideEnabled}
              onCheckedChange={(checked) => {
                updateRoleForm(role, {
                  jvmOverrideEnabled: checked,
                  jvmHeapSize:
                    checked && form.jvmHeapSize <= 0
                      ? defaultHeap ?? FALLBACK_JVM_HEAP[role]
                      : form.jvmHeapSize,
                });
              }}
            />
          </div>

          {form.jvmOverrideEnabled && (
            <div className='grid gap-2 md:max-w-xs'>
              <Label className='text-xs'>
                {t('cluster.jvmHeapSize')} <span className='text-destructive'>*</span>
              </Label>
              <Input
                type='number'
                min={1}
                value={form.jvmHeapSize || ''}
                onChange={(e) =>
                  updateRoleForm(role, {
                    jvmHeapSize: parseInt(e.target.value, 10) || 0,
                  })
                }
              />
              <p className='text-xs text-muted-foreground'>
                {t('cluster.jvmHeapSizeHint')}
              </p>
            </div>
          )}
        </div>
      </div>
    );
  };

  return (
    <Dialog open={open} onOpenChange={handleClose}>
      <DialogContent className='sm:max-w-[980px]'>
        <DialogHeader>
          <DialogTitle>{t('cluster.addNode')}</DialogTitle>
          <DialogDescription>{t('cluster.addNodeDescription')}</DialogDescription>
        </DialogHeader>

        <div className='max-h-[72vh] overflow-y-auto pr-1'>
          <div className='space-y-5 py-1'>
            <div className='grid gap-4 md:grid-cols-[1.1fr_0.9fr]'>
              <div className='space-y-2'>
                <Label>
                  {t('cluster.selectHost')} <span className='text-destructive'>*</span>
                </Label>
                {loadingHosts ? (
                  <div className='flex items-center gap-2 text-muted-foreground'>
                    <Loader2 className='h-4 w-4 animate-spin' />
                    {t('common.loading')}
                  </div>
                ) : availableHosts.length === 0 ? (
                  <div className='rounded-md border p-4 text-sm text-muted-foreground'>
                    {t('cluster.noAvailableHosts')}
                  </div>
                ) : (
                  <div className='flex gap-2'>
                    <Select
                      value={hostId}
                      onValueChange={(value) => {
                        setHostId(value);
                        setDiscoveredProcesses([]);
                        setSelectedProcess('');
                        setPrecheckResults([]);
                      }}
                    >
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

              <div className='space-y-2'>
                <Label>
                  {t('cluster.installDir')} <span className='text-destructive'>*</span>
                </Label>
                <Input
                  value={installDir}
                  onChange={(e) => setInstallDir(e.target.value)}
                  placeholder={t('cluster.installDirPlaceholder')}
                />
                <p className='text-xs text-muted-foreground'>
                  {t('cluster.nodeInstallDirDescription')}
                </p>
              </div>
            </div>

            {discoveredProcesses.length > 0 && (
              <div className='space-y-2 rounded-lg border p-4'>
                <Label>{t('discovery.discoveredProcesses')}</Label>
                <Select value={selectedProcess} onValueChange={handleProcessSelect}>
                  <SelectTrigger>
                    <SelectValue placeholder={t('discovery.selectProcess')} />
                  </SelectTrigger>
                  <SelectContent>
                    {discoveredProcesses.map((process) => (
                      <SelectItem
                        key={`${process.pid}-${process.install_dir}`}
                        value={`${process.pid}-${process.install_dir}`}
                      >
                        <div className='flex items-center gap-2'>
                          <Cpu className='h-4 w-4' />
                          <span>PID: {process.pid}</span>
                          <span className='text-muted-foreground'>|</span>
                          <span className='capitalize'>{process.role}</span>
                          <span className='text-muted-foreground'>|</span>
                          <span className='max-w-[260px] truncate text-xs text-muted-foreground'>
                            {process.install_dir}
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

            <div className='space-y-3 rounded-lg border p-4'>
              <div className='space-y-1'>
                <Label>{t('cluster.nodeRole')}</Label>
                <p className='text-xs text-muted-foreground'>
                  {isHybrid
                    ? t('cluster.hybridRoleFixedDescription')
                    : t('cluster.separatedRoleSelectionDescription')}
                </p>
              </div>

              {isHybrid ? (
                <div className='rounded-md bg-muted/50 px-3 py-2 text-sm font-medium'>
                  {t('cluster.roles.masterWorker')}
                </div>
              ) : (
                <div className='space-y-3'>
                  <div className='flex flex-wrap gap-6'>
                    <label className='flex items-center gap-2 text-sm'>
                      <Checkbox
                        checked={selectedMaster}
                        onCheckedChange={(checked) => {
                          setSelectedMaster(checked === true);
                          setPrecheckResults([]);
                        }}
                      />
                      <span>{t('cluster.roles.master')}</span>
                    </label>
                    <label className='flex items-center gap-2 text-sm'>
                      <Checkbox
                        checked={selectedWorker}
                        onCheckedChange={(checked) => {
                          setSelectedWorker(checked === true);
                          setPrecheckResults([]);
                        }}
                      />
                      <span>{t('cluster.roles.worker')}</span>
                    </label>
                  </div>
                  {selectedMaster && selectedWorker && (
                    <p className='text-xs text-muted-foreground'>
                      {t('cluster.sameHostCreatesTwoNodes')}
                    </p>
                  )}
                </div>
              )}
            </div>

            <div className='space-y-4'>
              {selectedRoles.map((role) => renderRoleConfig(role))}
            </div>

            {precheckResults.length > 0 && (
              <div className='space-y-3 rounded-lg border p-4'>
                <div className='space-y-1'>
                  <h4 className='font-medium'>{t('cluster.precheck')}</h4>
                  <p className='text-xs text-muted-foreground'>
                    {precheckResults.every((item) => item.result.success)
                      ? t('cluster.precheckPassed')
                      : t('cluster.precheckFailed')}
                  </p>
                </div>
                {precheckResults.map(({role, result}) => (
                  <div key={role} className='rounded-md bg-muted/40 p-3 space-y-2'>
                    <div className='font-medium text-sm'>
                      {t('cluster.precheckResultForRole', {
                        role: t(`cluster.roles.${getRoleTranslationKey(role)}`),
                      })}
                    </div>
                    <div className='space-y-1'>
                      {result.checks.map((check: PrecheckCheckItem, index: number) => (
                        <div key={`${role}-${index}`} className='flex items-start gap-2 text-xs'>
                          {renderStatusIcon(check.status)}
                          <div>
                            <span className='font-medium'>
                              {t(`cluster.precheckItems.${check.name}`, {
                                defaultValue: check.name,
                              })}
                              :{' '}
                            </span>
                            <span className='text-muted-foreground'>{check.message}</span>
                          </div>
                        </div>
                      ))}
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>
        </div>

        <DialogFooter className='gap-2 sm:gap-0'>
          <Button
            variant='outline'
            onClick={() => handleClose(false)}
            disabled={loading || precheckLoading}
          >
            {t('common.cancel')}
          </Button>
          <Button
            variant='secondary'
            onClick={handlePrecheck}
            disabled={
              loading ||
              precheckLoading ||
              !hostId ||
              selectedRoles.length === 0 ||
              availableHosts.length === 0
            }
          >
            {precheckLoading && <Loader2 className='mr-2 h-4 w-4 animate-spin' />}
            {t('cluster.precheck')}
          </Button>
          <Button
            onClick={handleSubmit}
            disabled={
              loading ||
              precheckLoading ||
              !hostId ||
              selectedRoles.length === 0 ||
              availableHosts.length === 0
            }
          >
            {loading && <Loader2 className='mr-2 h-4 w-4 animate-spin' />}
            {t('cluster.addNode')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
