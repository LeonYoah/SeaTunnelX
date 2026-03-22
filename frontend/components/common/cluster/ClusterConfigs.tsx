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

/**
 * Cluster Configs Component
 * 集群配置组件
 */

import {useState, useEffect, useCallback, useMemo, useRef} from 'react';
import {useTranslations} from 'next-intl';
import {Button} from '@/components/ui/button';
import {Card, CardContent, CardHeader, CardTitle} from '@/components/ui/card';
import {Badge} from '@/components/ui/badge';
import {Select, SelectContent, SelectItem, SelectTrigger, SelectValue} from '@/components/ui/select';
import {Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle} from '@/components/ui/dialog';
import {Textarea} from '@/components/ui/textarea';
import {Input} from '@/components/ui/input';
import {Label} from '@/components/ui/label';
import {Tabs, TabsContent, TabsList, TabsTrigger} from '@/components/ui/tabs';
import {ScrollArea} from '@/components/ui/scroll-area';
import {Tooltip, TooltipContent, TooltipTrigger} from '@/components/ui/tooltip';
import {toast} from 'sonner';
import {Settings, RefreshCw, FileText, Server, Check, AlertTriangle, Edit, History, Upload, Download, Loader2, FolderSync, Eye, GitCompareArrows, WandSparkles, CircleHelp} from 'lucide-react';
import {ConfigService} from '@/lib/services/config';
import type {ConfigInfo, ConfigVersionInfo} from '@/lib/services/config';
import {ConfigType, ConfigTypeNames, getConfigTypesForMode} from '@/lib/services/config';
import services from '@/lib/services';
import type {NodeInfo} from '@/lib/services/cluster/types';

interface ClusterConfigsProps {
  clusterId: number;
  deploymentMode: string;
}

interface DiffRow {
  leftLineNumber: number | null;
  leftText: string;
  rightLineNumber: number | null;
  rightText: string;
  changed: boolean;
}

function buildDiffRows(left: string, right: string): DiffRow[] {
  const leftLines = left.split('\n');
  const rightLines = right.split('\n');
  const maxLen = Math.max(leftLines.length, rightLines.length);

  return Array.from({length: maxLen}, (_, index) => {
    const leftText = leftLines[index] ?? '';
    const rightText = rightLines[index] ?? '';
    return {
      leftLineNumber: index < leftLines.length ? index + 1 : null,
      leftText,
      rightLineNumber: index < rightLines.length ? index + 1 : null,
      rightText,
      changed: leftText !== rightText,
    };
  });
}

function isYamlConfigType(configType?: ConfigType | null): boolean {
  switch (configType) {
    case ConfigType.SEATUNNEL:
    case ConfigType.HAZELCAST:
    case ConfigType.HAZELCAST_CLIENT:
    case ConfigType.HAZELCAST_MASTER:
    case ConfigType.HAZELCAST_WORKER:
      return true;
    default:
      return false;
  }
}

export function ClusterConfigs({clusterId, deploymentMode}: ClusterConfigsProps) {
  const t = useTranslations();
  const [configs, setConfigs] = useState<ConfigInfo[]>([]);
  const [nodes, setNodes] = useState<NodeInfo[]>([]);
  const [loading, setLoading] = useState(true);
  
  // Get available config types based on deployment mode
  const availableConfigTypes = getConfigTypesForMode(deploymentMode);
  const [selectedConfigType, setSelectedConfigType] = useState<ConfigType>(availableConfigTypes[0]);
  const [editDialogOpen, setEditDialogOpen] = useState(false);
  const [editingConfig, setEditingConfig] = useState<ConfigInfo | null>(null);
  const [editContent, setEditContent] = useState('');
  const [editComment, setEditComment] = useState('');
  const [saving, setSaving] = useState(false);
  const [saveMode, setSaveMode] = useState<'save' | 'save-and-sync'>('save');
  const [repairing, setRepairing] = useState(false);
  const [versionDialogOpen, setVersionDialogOpen] = useState(false);
  const [versionConfig, setVersionConfig] = useState<ConfigInfo | null>(null);
  const [versions, setVersions] = useState<ConfigVersionInfo[]>([]);
  const [loadingVersions, setLoadingVersions] = useState(false);
  const [selectedVersionId, setSelectedVersionId] = useState<number | null>(null);
  const [versionViewMode, setVersionViewMode] = useState<'preview' | 'compare'>('preview');
  
  // Init config dialog state
  const [initDialogOpen, setInitDialogOpen] = useState(false);
  const [selectedNodeId, setSelectedNodeId] = useState<string>('');
  const [initLoading, setInitLoading] = useState(false);
  const [syncAllLoading, setSyncAllLoading] = useState(false);
  const editTextareaRef = useRef<HTMLTextAreaElement | null>(null);

  const selectedVersion = useMemo(
    () => versions.find((version) => version.id === selectedVersionId) || null,
    [versions, selectedVersionId]
  );
  const diffRows = useMemo(
    () => buildDiffRows(versionConfig?.content || '', selectedVersion?.content || ''),
    [versionConfig?.content, selectedVersion?.content]
  );

  const loadData = useCallback(async () => {
    setLoading(true);
    try {
      const [configsResult, nodesResult] = await Promise.all([
        ConfigService.getClusterConfigs(clusterId),
        services.cluster.getNodesSafe(clusterId),
      ]);
      setConfigs(configsResult || []);
      if (nodesResult.success && nodesResult.data) {
        setNodes(nodesResult.data);
      }
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t('config.loadError'));
    } finally {
      setLoading(false);
    }
  }, [clusterId, t]);

  useEffect(() => { loadData(); }, [loadData]);

  const templateConfig = configs.find(c => c.config_type === selectedConfigType && c.is_template);
  const nodeConfigs = configs.filter(c => c.config_type === selectedConfigType && !c.is_template);
  const mismatchedNodeCount = nodeConfigs.filter((config) => !config.match_template).length;

  const handleEdit = (config: ConfigInfo) => {
    setEditingConfig(config);
    setEditContent(config.content);
    setEditComment('');
    setEditDialogOpen(true);
  };

  const handleSave = async (syncAfterSave = false) => {
    if (!editingConfig) {return;}
    setSaveMode(syncAfterSave ? 'save-and-sync' : 'save');
    setSaving(true);
    try {
      const result = await ConfigService.updateConfig(editingConfig.id, {content: editContent, comment: editComment || undefined});
      if (syncAfterSave && editingConfig.is_template) {
        const syncResult = await ConfigService.syncTemplateToAllNodes(clusterId, editingConfig.config_type);
        if (syncResult.synced_count > 0) {
          if (syncResult.push_errors && syncResult.push_errors.length > 0) {
            const errorNodes = syncResult.push_errors.map((e) => e.host_ip || `Host ${e.host_id}`).join(', ');
            toast.warning(t('config.saveAndSyncSuccessWithPushErrors', {count: syncResult.synced_count, nodes: errorNodes}));
          } else {
            toast.success(t('config.saveAndSyncSuccess', {count: syncResult.synced_count}));
          }
        } else {
          toast.success(t('config.saveAndSyncSuccessNoChanges'));
        }
      } else if (result.push_error) {
        toast.warning(t('config.saveSuccessWithPushError', {error: result.push_error}));
      } else {
        toast.success(t('config.saveSuccess'));
      }
      setEditDialogOpen(false);
      loadData();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t('config.saveFailed'));
    } finally {
      setSaving(false);
      setSaveMode('save');
    }
  };

  const handleSmartRepair = async () => {
    if (!editingConfig || !isYamlConfigType(editingConfig.config_type)) {
      return;
    }
    setRepairing(true);
    try {
      const normalized = await ConfigService.normalizeConfig({
        config_type: editingConfig.config_type,
        content: editContent,
      });
      setEditContent(normalized);
      toast.success(t('config.smartRepairSuccess'));
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t('config.smartRepairFailed'));
    } finally {
      setRepairing(false);
    }
  };

  const handleViewVersions = async (config: ConfigInfo) => {
    setVersionConfig(config);
    setVersionDialogOpen(true);
    setLoadingVersions(true);
    setVersionViewMode('preview');
    try {
      const result = await ConfigService.getConfigVersions(config.id);
      const versionList = result || [];
      setVersions(versionList);
      setSelectedVersionId(versionList[0]?.id ?? null);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t('config.loadVersionsFailed'));
    } finally {
      setLoadingVersions(false);
    }
  };

  const handleRollback = async (version: number) => {
    if (!versionConfig) {return;}
    try {
      const result = await ConfigService.rollbackConfig(versionConfig.id, {version, comment: `Rollback to v${version}`});
      if (result.push_error) {
        toast.warning(t('config.rollbackSuccessWithPushError', {error: result.push_error}));
      } else {
        toast.success(t('config.rollbackSuccess'));
      }
      setVersionDialogOpen(false);
      loadData();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t('config.rollbackFailed'));
    }
  };

  const handlePreviewVersion = (versionId: number) => {
    setSelectedVersionId(versionId);
    setVersionViewMode('preview');
  };

  const handleCompareVersion = (versionId: number) => {
    setSelectedVersionId(versionId);
    setVersionViewMode('compare');
  };

  const handlePromote = async (config: ConfigInfo) => {
    try {
      await ConfigService.promoteConfig(config.id, {comment: t('config.promoteComment')});
      toast.success(t('config.promoteSuccess'));
      loadData();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t('config.promoteFailed'));
    }
  };

  const handleSyncFromTemplate = async (config: ConfigInfo) => {
    try {
      const result = await ConfigService.syncFromTemplate(config.id, {comment: t('config.syncComment')});
      if (result.push_error) {
        toast.warning(t('config.syncSuccessWithPushError', {error: result.push_error}));
      } else {
        toast.success(t('config.syncSuccess'));
      }
      loadData();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t('config.syncFailed'));
    }
  };

  const handleOpenInitDialog = () => {
    if (nodes.length > 0) {
      setSelectedNodeId(String(nodes[0].id));
    }
    setInitDialogOpen(true);
  };

  const handleInitConfigs = async () => {
    if (!selectedNodeId) {
      toast.error(t('config.selectNodeFirst'));
      return;
    }
    const node = nodes.find(n => n.id === Number(selectedNodeId));
    if (!node) {
      toast.error(t('config.nodeNotFound'));
      return;
    }
    setInitLoading(true);
    try {
      await ConfigService.initClusterConfigs(clusterId, node.host_id, node.install_dir);
      toast.success(t('config.initSuccess'));
      setInitDialogOpen(false);
      loadData();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t('config.initFailed'));
    } finally {
      setInitLoading(false);
    }
  };

  const handleSyncToAllNodes = async () => {
    if (!templateConfig) {return;}
    setSyncAllLoading(true);
    try {
      const result = await ConfigService.syncTemplateToAllNodes(clusterId, selectedConfigType);
      if (result.synced_count > 0) {
        if (result.push_errors && result.push_errors.length > 0) {
          const errorNodes = result.push_errors.map(e => e.host_ip || `Host ${e.host_id}`).join(', ');
          toast.warning(t('config.syncAllSuccessWithPushErrors', {count: result.synced_count, nodes: errorNodes}));
        } else {
          toast.success(t('config.syncAllSuccess', {count: result.synced_count}));
        }
      } else {
        toast.info(t('config.syncAllNoChanges'));
      }
      loadData();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t('config.syncAllFailed'));
    } finally {
      setSyncAllLoading(false);
    }
  };

  return (
    <Card data-testid="cluster-configs-root">
      <CardHeader className="flex flex-row items-center justify-between">
        <CardTitle className="flex items-center gap-2">
          <Settings className="h-5 w-5" />
          {t('config.title')}
        </CardTitle>
        <div className="flex items-center gap-2">
          <Select value={selectedConfigType} onValueChange={(v) => setSelectedConfigType(v as ConfigType)}>
            <SelectTrigger className="w-[200px]" data-testid="cluster-configs-type-select"><SelectValue /></SelectTrigger>
            <SelectContent>
              {availableConfigTypes.map((type) => (
                <SelectItem key={type} value={type}>{ConfigTypeNames[type]}</SelectItem>
              ))}
            </SelectContent>
          </Select>
          <Button variant="outline" size="sm" onClick={handleOpenInitDialog} disabled={loading || nodes.length === 0} data-testid="cluster-configs-init-button">
            <FolderSync className="h-4 w-4 mr-1" />
            {t('config.initFromNode')}
          </Button>
          <Button variant="outline" size="icon" onClick={loadData} disabled={loading}>
            <RefreshCw className={`h-4 w-4 ${loading ? 'animate-spin' : ''}`} />
          </Button>
        </div>
      </CardHeader>
      <CardContent>
        {loading ? (
          <div className="flex items-center justify-center py-8">
            <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
          </div>
        ) : configs.length === 0 ? (
          <div className="text-center py-8 text-muted-foreground">
            <FileText className="h-12 w-12 mx-auto mb-4 opacity-50" />
            <p>{t('config.noConfigs')}</p>
            <p className="text-sm mt-2">{t('config.initHint')}</p>
            {nodes.length > 0 && (
              <Button variant="outline" className="mt-4" onClick={handleOpenInitDialog}>
                <FolderSync className="h-4 w-4 mr-2" />
                {t('config.initFromNode')}
              </Button>
            )}
          </div>
        ) : (
          <Tabs defaultValue="template" className="w-full">
            <TabsList className="grid w-full grid-cols-2">
              <TabsTrigger value="template" className="flex items-center gap-2" data-testid="cluster-configs-tab-template">
                <FileText className="h-4 w-4" />{t('config.clusterTemplate')}
              </TabsTrigger>
              <TabsTrigger value="nodes" className="flex items-center gap-2" data-testid="cluster-configs-tab-nodes">
                <Server className="h-4 w-4" />{t('config.nodeConfigs')}
                {nodeConfigs.length > 0 && <Badge variant="secondary" className="ml-1">{nodeConfigs.length}</Badge>}
              </TabsTrigger>
            </TabsList>
            <TabsContent value="template" className="mt-4">
              <div className="mb-4 rounded-lg border bg-muted/30 p-3 text-sm text-muted-foreground">
                <p className="font-medium text-foreground">{t('config.templateSectionTitle')}</p>
                <p className="mt-1">{t('config.templateSectionDesc')}</p>
              </div>
              {templateConfig && mismatchedNodeCount > 0 && (
                <div className="mb-4 rounded-lg border border-amber-200 bg-amber-50 p-3 text-sm text-amber-900 dark:border-amber-900/40 dark:bg-amber-950/20 dark:text-amber-200" data-testid="cluster-configs-pending-sync">
                  <p className="font-medium">{t('config.pendingSyncTitle', {count: mismatchedNodeCount})}</p>
                  <p className="mt-1">{t('config.pendingSyncDesc')}</p>
                  <div className="mt-3">
                    <Button variant="outline" size="sm" onClick={handleSyncToAllNodes} disabled={syncAllLoading} data-testid="cluster-configs-template-sync-all-banner">
                      {syncAllLoading ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : <Download className="mr-2 h-4 w-4" />}
                      {t('config.syncToAllNodes')}
                    </Button>
                  </div>
                </div>
              )}
              {templateConfig ? (
                <div className="space-y-3">
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-2">
                      <Badge variant="outline">{ConfigTypeNames[selectedConfigType]}</Badge>
                      <Badge variant="secondary">v{templateConfig.version}</Badge>
                      <span className="text-sm text-muted-foreground">{new Date(templateConfig.updated_at).toLocaleString()}</span>
                    </div>
                    <div className="flex gap-1">
                      <Tooltip>
                        <TooltipTrigger asChild>
                          <Button variant="outline" size="sm" onClick={() => handleEdit(templateConfig)} data-testid="cluster-configs-template-edit">
                            <Edit className="h-4 w-4 mr-1" />{t('common.edit')}
                          </Button>
                        </TooltipTrigger>
                        <TooltipContent>{t('config.editTemplateHint')}</TooltipContent>
                      </Tooltip>
                      <Tooltip>
                        <TooltipTrigger asChild>
                          <Button variant="outline" size="sm" onClick={() => handleViewVersions(templateConfig)} data-testid="cluster-configs-template-versions">
                            <History className="h-4 w-4 mr-1" />{t('config.versions')}
                          </Button>
                        </TooltipTrigger>
                        <TooltipContent>{t('config.historyHint')}</TooltipContent>
                      </Tooltip>
                      <Tooltip>
                        <TooltipTrigger asChild>
                          <Button variant="outline" size="sm" onClick={handleSyncToAllNodes} disabled={syncAllLoading || nodeConfigs.length === 0} data-testid="cluster-configs-template-sync-all">
                            {syncAllLoading ? <Loader2 className="h-4 w-4 mr-1 animate-spin" /> : <Download className="h-4 w-4 mr-1" />}
                            {t('config.syncToAllNodes')}
                          </Button>
                        </TooltipTrigger>
                        <TooltipContent>{t('config.syncAllHint')}</TooltipContent>
                      </Tooltip>
                    </div>
                  </div>
                  <pre className="p-3 bg-muted rounded-md text-xs font-mono max-h-[300px] overflow-auto" data-testid="cluster-configs-template-content">{templateConfig.content}</pre>
                </div>
              ) : (
                <p className="text-sm text-muted-foreground py-4 text-center">{t('config.noTemplate')}</p>
              )}
            </TabsContent>
            <TabsContent value="nodes" className="mt-4">
              <div className="mb-4 rounded-lg border bg-muted/30 p-3 text-sm text-muted-foreground">
                <p className="font-medium text-foreground">{t('config.nodeSectionTitle')}</p>
                <p className="mt-1">{t('config.nodeSectionDesc')}</p>
              </div>
              {nodeConfigs.length === 0 ? (
                <p className="text-sm text-muted-foreground py-4 text-center">{t('config.noNodeConfigs')}</p>
              ) : (
                <div className="space-y-3">
                  {nodeConfigs.map((config) => (
                    <div key={config.id} className="border rounded-lg p-3">
                      <div className="flex items-center justify-between">
                        <div className="flex items-center gap-2">
                          <Server className="h-4 w-4 text-muted-foreground" />
                          <span className="font-medium">{config.host_name || `Node ${config.host_id}`}</span>
                          {config.host_ip && <span className="text-sm text-muted-foreground">({config.host_ip})</span>}
                          {config.match_template ? (
                            <Badge variant="outline" className="text-green-600"><Check className="h-3 w-3 mr-1" />{t('config.matchTemplate')}</Badge>
                          ) : (
                            <Badge variant="outline" className="text-yellow-600"><AlertTriangle className="h-3 w-3 mr-1" />{t('config.customized')}</Badge>
                          )}
                          <Badge variant="secondary">v{config.version}</Badge>
                        </div>
                        <div className="flex flex-wrap justify-end gap-2">
                          <Tooltip>
                            <TooltipTrigger asChild>
                                  <Button variant="outline" size="sm" onClick={() => handleEdit(config)} data-testid={`cluster-configs-node-edit-${config.id}`}>
                                    <Edit className="h-4 w-4 mr-1" />
                                    {t('common.edit')}
                                  </Button>
                            </TooltipTrigger>
                            <TooltipContent>{t('config.editNodeHint')}</TooltipContent>
                          </Tooltip>
                          <Tooltip>
                            <TooltipTrigger asChild>
                              <Button variant="outline" size="sm" onClick={() => handleViewVersions(config)} data-testid={`cluster-configs-node-versions-${config.id}`}>
                                <History className="h-4 w-4 mr-1" />
                                {t('config.versions')}
                              </Button>
                            </TooltipTrigger>
                            <TooltipContent>{t('config.historyHint')}</TooltipContent>
                          </Tooltip>
                          {!config.match_template && (
                            <>
                              <Tooltip>
                                <TooltipTrigger asChild>
                                  <Button variant="outline" size="sm" onClick={() => handlePromote(config)} data-testid={`cluster-configs-node-promote-${config.id}`}>
                                    <Upload className="h-4 w-4 mr-1" />
                                    {t('config.promoteToCluster')}
                                  </Button>
                                </TooltipTrigger>
                                <TooltipContent>{t('config.promoteHint')}</TooltipContent>
                              </Tooltip>
                              <Tooltip>
                                <TooltipTrigger asChild>
                                  <Button variant="outline" size="sm" onClick={() => handleSyncFromTemplate(config)} data-testid={`cluster-configs-node-sync-${config.id}`}>
                                    <Download className="h-4 w-4 mr-1" />
                                    {t('config.syncFromTemplate')}
                                  </Button>
                                </TooltipTrigger>
                                <TooltipContent>{t('config.syncFromTemplateHint')}</TooltipContent>
                              </Tooltip>
                            </>
                          )}
                        </div>
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </TabsContent>
          </Tabs>
        )}
      </CardContent>

      {/* Init Config Dialog */}
      <Dialog open={initDialogOpen} onOpenChange={setInitDialogOpen}>
        <DialogContent className="sm:max-w-2xl">
          <DialogHeader>
            <DialogTitle>{t('config.initFromNode')}</DialogTitle>
            <DialogDescription>{t('config.initFromNodeDesc')}</DialogDescription>
          </DialogHeader>
          <div className="space-y-4">
            <div>
              <Label>{t('config.selectNode')}</Label>
              <ScrollArea className="mt-2 max-h-64 rounded-md border">
                <div className="space-y-2 p-2">
                  {nodes.map((node) => {
                    const selected = selectedNodeId === String(node.id);
                    return (
                      <button
                        key={node.id}
                        type="button"
                        onClick={() => setSelectedNodeId(String(node.id))}
                        data-testid={`cluster-configs-init-node-${node.id}`}
                        className={`w-full rounded-md border p-3 text-left transition-colors ${
                          selected ? 'border-primary bg-primary/5' : 'border-border hover:bg-muted/50'
                        }`}
                      >
                        <div className="flex items-center justify-between gap-3">
                          <div className="min-w-0">
                            <div className="truncate font-medium">
                              {node.host_name || `Node ${node.id}`} ({node.host_ip})
                            </div>
                            <div className="mt-1 truncate text-sm text-muted-foreground">
                              {node.install_dir}
                            </div>
                          </div>
                          <Badge variant={selected ? 'default' : 'outline'}>
                            {node.role}
                          </Badge>
                        </div>
                      </button>
                    );
                  })}
                </div>
              </ScrollArea>
            </div>
            {selectedNodeId && (
              <div className="text-sm text-muted-foreground bg-muted p-3 rounded-md">
                <p>{t('config.initWillPull')}</p>
                <ul className="list-disc list-inside mt-2 space-y-1">
                  {availableConfigTypes.map((type) => (
                    <li key={type}>{ConfigTypeNames[type]}</li>
                  ))}
                </ul>
              </div>
            )}
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setInitDialogOpen(false)}>{t('common.cancel')}</Button>
            <Button onClick={handleInitConfigs} disabled={initLoading || !selectedNodeId} data-testid="cluster-configs-init-confirm">
              {initLoading && <Loader2 className="h-4 w-4 mr-2 animate-spin" />}
              {t('config.initConfirm')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Edit Dialog */}
      <Dialog open={editDialogOpen} onOpenChange={setEditDialogOpen}>
        <DialogContent
          className="!max-w-5xl max-h-[90vh]"
          data-testid="cluster-configs-edit-dialog"
          onOpenAutoFocus={(event) => {
            event.preventDefault();
            window.requestAnimationFrame(() => {
              editTextareaRef.current?.focus();
            });
          }}
        >
          <DialogHeader>
            <DialogTitle>{t('config.editConfig')} - {editingConfig?.is_template ? t('config.clusterTemplate') : editingConfig?.host_name}</DialogTitle>
            <DialogDescription>{ConfigTypeNames[editingConfig?.config_type as ConfigType]}</DialogDescription>
          </DialogHeader>
          <div className="space-y-4">
            <div>
              <div className="flex items-center justify-between gap-3">
                <div className="flex items-center gap-2">
                  <Label>{t('config.content')}</Label>
                  {isYamlConfigType(editingConfig?.config_type) && (
                    <Tooltip>
                      <TooltipTrigger asChild>
                        <button type="button" className="text-muted-foreground transition-colors hover:text-foreground">
                          <CircleHelp className="h-4 w-4" />
                        </button>
                      </TooltipTrigger>
                      <TooltipContent className="max-w-xs">
                        {t('config.smartRepairHint')}
                      </TooltipContent>
                    </Tooltip>
                  )}
                </div>
                {isYamlConfigType(editingConfig?.config_type) && (
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    onClick={handleSmartRepair}
                    disabled={repairing || saving}
                    data-testid="cluster-configs-smart-repair"
                  >
                    {repairing ? (
                      <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                    ) : (
                      <WandSparkles className="mr-2 h-4 w-4" />
                    )}
                    {t('config.smartRepair')}
                  </Button>
                )}
              </div>
              <Textarea
                ref={editTextareaRef}
                value={editContent}
                onChange={(e) => setEditContent(e.target.value)}
                className="font-mono text-sm h-[500px]"
                data-testid="cluster-configs-edit-content"
              />
              {isYamlConfigType(editingConfig?.config_type) && (
                <p className="mt-2 text-xs text-muted-foreground">
                  {t('config.yamlValidationHint')}
                </p>
              )}
            </div>
            <div>
              <Label>{t('config.comment')}</Label>
              <Input value={editComment} onChange={(e) => setEditComment(e.target.value)} placeholder={t('config.commentPlaceholder')} />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setEditDialogOpen(false)}>{t('common.cancel')}</Button>
            {editingConfig?.is_template && (
              <Button variant="outline" onClick={() => handleSave(false)} disabled={saving} data-testid="cluster-configs-save-only">
                {saving && saveMode === 'save' && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                {t('config.saveOnly')}
              </Button>
            )}
            <Button onClick={() => handleSave(editingConfig?.is_template)} disabled={saving} data-testid="cluster-configs-save-primary">
              {saving && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
              {editingConfig?.is_template ? t('config.saveAndSync') : t('common.save')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Version History Dialog */}
      <Dialog open={versionDialogOpen} onOpenChange={setVersionDialogOpen}>
        <DialogContent className="w-[96vw] sm:max-w-7xl max-h-[90vh]" data-testid="cluster-configs-versions-dialog">
          <DialogHeader>
            <DialogTitle>{t('config.versionHistory')}</DialogTitle>
            <DialogDescription>{versionConfig?.is_template ? t('config.clusterTemplate') : versionConfig?.host_name}</DialogDescription>
          </DialogHeader>
          <div className="grid gap-4 lg:grid-cols-[320px_minmax(0,1fr)]">
            {loadingVersions ? (
              <div className="flex items-center justify-center py-8"><Loader2 className="h-6 w-6 animate-spin" /></div>
            ) : versions.length === 0 ? (
              <p className="text-center py-8 text-muted-foreground">{t('config.noVersions')}</p>
            ) : (
              <>
                <ScrollArea className="h-[65vh] rounded-md border">
                  <div className="space-y-3 p-3">
                    {versions.map((version) => {
                      const selected = version.id === selectedVersionId;
                      const isCurrent = version.version === versionConfig?.version;
                      return (
                        <div key={version.id} className={`rounded-lg border p-3 ${selected ? 'border-primary bg-primary/5' : ''}`}>
                          <div className="flex items-center gap-2">
                            <Badge variant={selected ? 'default' : 'secondary'}>v{version.version}</Badge>
                            {isCurrent && <Badge variant="outline">{t('config.current')}</Badge>}
                          </div>
                          <p className="mt-2 text-sm text-muted-foreground">
                            {new Date(version.created_at).toLocaleString()}
                          </p>
                          <p className="mt-2 line-clamp-2 text-sm text-muted-foreground">
                            {version.comment || t('config.noComment')}
                          </p>
                          <div className="mt-3 flex flex-wrap gap-2">
                            <Button variant="outline" size="sm" onClick={() => handlePreviewVersion(version.id)} data-testid={`cluster-configs-version-preview-${version.id}`}>
                              <Eye className="h-4 w-4 mr-1" />
                              {t('config.preview')}
                            </Button>
                            <Button variant="outline" size="sm" onClick={() => handleCompareVersion(version.id)} disabled={isCurrent} data-testid={`cluster-configs-version-compare-${version.id}`}>
                              <GitCompareArrows className="h-4 w-4 mr-1" />
                              {t('config.compareWithCurrent')}
                            </Button>
                            {!isCurrent && (
                              <Button variant="outline" size="sm" onClick={() => handleRollback(version.version)} data-testid={`cluster-configs-version-rollback-${version.id}`}>
                                {t('config.rollback')}
                              </Button>
                            )}
                          </div>
                        </div>
                      );
                    })}
                  </div>
                </ScrollArea>
                <div className="min-w-0 rounded-md border">
                  {selectedVersion ? (
                    <div className="flex h-[65vh] flex-col">
                      <div className="border-b px-4 py-3">
                        <div className="flex flex-wrap items-center gap-2">
                          <Badge variant="secondary">v{selectedVersion.version}</Badge>
                          {selectedVersion.version === versionConfig?.version && (
                            <Badge variant="outline">{t('config.current')}</Badge>
                          )}
                          <span className="text-sm text-muted-foreground">
                            {new Date(selectedVersion.created_at).toLocaleString()}
                          </span>
                        </div>
                        <p className="mt-2 text-sm text-muted-foreground">
                          {selectedVersion.comment || t('config.noComment')}
                        </p>
                      </div>
                      <Tabs value={versionViewMode} onValueChange={(value) => setVersionViewMode(value as 'preview' | 'compare')} className="flex min-h-0 flex-1 flex-col">
                        <div className="border-b px-4 py-2">
                          <TabsList>
                            <TabsTrigger value="preview">{t('config.preview')}</TabsTrigger>
                            <TabsTrigger value="compare" disabled={selectedVersion.version === versionConfig?.version}>
                              {t('config.compareWithCurrent')}
                            </TabsTrigger>
                          </TabsList>
                        </div>
                        <TabsContent value="preview" className="mt-0 min-h-0 flex-1">
                          <ScrollArea className="h-full">
                            <pre className="min-h-full whitespace-pre-wrap break-all p-4 text-xs font-mono" data-testid="cluster-configs-version-preview-content">
                              {selectedVersion.content}
                            </pre>
                          </ScrollArea>
                        </TabsContent>
                        <TabsContent value="compare" className="mt-0 min-h-0 flex-1">
                          {selectedVersion.version === versionConfig?.version ? (
                            <div className="flex h-full items-center justify-center text-sm text-muted-foreground">
                              {t('config.currentVersionCompareHint')}
                            </div>
                          ) : (
                            <div className="flex h-full min-h-0 flex-col">
                              <div className="grid grid-cols-2 border-b text-xs text-muted-foreground">
                                <div className="border-r px-4 py-2">
                                  {t('config.currentVersionLabel', {version: versionConfig?.version ?? '-'})}
                                </div>
                                <div className="px-4 py-2">
                                  {t('config.versionLabel', {version: selectedVersion.version})}
                                </div>
                              </div>
                              <ScrollArea className="h-full" data-testid="cluster-configs-version-compare-content">
                                <div className="grid min-w-[920px] grid-cols-2 text-xs font-mono">
                                  <div className="border-r">
                                    {diffRows.map((row, index) => (
                                      <div
                                        key={`left-${index}`}
                                        className={`grid grid-cols-[56px,minmax(0,1fr)] px-2 py-1 ${row.changed ? 'bg-amber-50 dark:bg-amber-950/20' : ''}`}
                                      >
                                        <span className="pr-3 text-right text-muted-foreground">{row.leftLineNumber ?? ''}</span>
                                        <span className="whitespace-pre-wrap break-all">{row.leftText}</span>
                                      </div>
                                    ))}
                                  </div>
                                  <div>
                                    {diffRows.map((row, index) => (
                                      <div
                                        key={`right-${index}`}
                                        className={`grid grid-cols-[56px,minmax(0,1fr)] px-2 py-1 ${row.changed ? 'bg-amber-50 dark:bg-amber-950/20' : ''}`}
                                      >
                                        <span className="pr-3 text-right text-muted-foreground">{row.rightLineNumber ?? ''}</span>
                                        <span className="whitespace-pre-wrap break-all">{row.rightText}</span>
                                      </div>
                                    ))}
                                  </div>
                                </div>
                              </ScrollArea>
                            </div>
                          )}
                        </TabsContent>
                      </Tabs>
                    </div>
                  ) : (
                    <div className="flex h-[65vh] items-center justify-center text-sm text-muted-foreground">
                      {t('config.selectVersionToPreview')}
                    </div>
                  )}
                </div>
              </>
            )}
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setVersionDialogOpen(false)}>{t('common.close')}</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </Card>
  );
}
