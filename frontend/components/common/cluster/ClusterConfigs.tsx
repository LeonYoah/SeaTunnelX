'use client';

/**
 * Cluster Configs Component
 * 集群配置组件
 */

import {useState, useEffect, useCallback} from 'react';
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
import {toast} from 'sonner';
import {Settings, RefreshCw, FileText, Server, Check, AlertTriangle, Edit, History, Upload, Download, Loader2, FolderSync} from 'lucide-react';
import {ConfigService} from '@/lib/services/config';
import type {ConfigInfo, ConfigVersionInfo} from '@/lib/services/config';
import {ConfigType, ConfigTypeNames, getConfigTypesForMode} from '@/lib/services/config';
import services from '@/lib/services';
import type {NodeInfo} from '@/lib/services/cluster/types';

interface ClusterConfigsProps {
  clusterId: number;
  deploymentMode: string;
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
  const [versionDialogOpen, setVersionDialogOpen] = useState(false);
  const [versionConfig, setVersionConfig] = useState<ConfigInfo | null>(null);
  const [versions, setVersions] = useState<ConfigVersionInfo[]>([]);
  const [loadingVersions, setLoadingVersions] = useState(false);
  
  // Init config dialog state
  const [initDialogOpen, setInitDialogOpen] = useState(false);
  const [selectedNodeId, setSelectedNodeId] = useState<string>('');
  const [initLoading, setInitLoading] = useState(false);
  const [syncAllLoading, setSyncAllLoading] = useState(false);

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

  const handleEdit = (config: ConfigInfo) => {
    setEditingConfig(config);
    setEditContent(config.content);
    setEditComment('');
    setEditDialogOpen(true);
  };

  const handleSave = async () => {
    if (!editingConfig) {return;}
    setSaving(true);
    try {
      const result = await ConfigService.updateConfig(editingConfig.id, {content: editContent, comment: editComment || undefined});
      if (result.push_error) {
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
    }
  };

  const handleViewVersions = async (config: ConfigInfo) => {
    setVersionConfig(config);
    setVersionDialogOpen(true);
    setLoadingVersions(true);
    try {
      const result = await ConfigService.getConfigVersions(config.id);
      setVersions(result || []);
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
    <Card>
      <CardHeader className="flex flex-row items-center justify-between">
        <CardTitle className="flex items-center gap-2">
          <Settings className="h-5 w-5" />
          {t('config.title')}
        </CardTitle>
        <div className="flex items-center gap-2">
          <Select value={selectedConfigType} onValueChange={(v) => setSelectedConfigType(v as ConfigType)}>
            <SelectTrigger className="w-[200px]"><SelectValue /></SelectTrigger>
            <SelectContent>
              {availableConfigTypes.map((type) => (
                <SelectItem key={type} value={type}>{ConfigTypeNames[type]}</SelectItem>
              ))}
            </SelectContent>
          </Select>
          <Button variant="outline" size="sm" onClick={handleOpenInitDialog} disabled={loading || nodes.length === 0}>
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
              <TabsTrigger value="template" className="flex items-center gap-2">
                <FileText className="h-4 w-4" />{t('config.clusterTemplate')}
              </TabsTrigger>
              <TabsTrigger value="nodes" className="flex items-center gap-2">
                <Server className="h-4 w-4" />{t('config.nodeConfigs')}
                {nodeConfigs.length > 0 && <Badge variant="secondary" className="ml-1">{nodeConfigs.length}</Badge>}
              </TabsTrigger>
            </TabsList>
            <TabsContent value="template" className="mt-4">
              {templateConfig ? (
                <div className="space-y-3">
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-2">
                      <Badge variant="outline">{ConfigTypeNames[selectedConfigType]}</Badge>
                      <Badge variant="secondary">v{templateConfig.version}</Badge>
                      <span className="text-sm text-muted-foreground">{new Date(templateConfig.updated_at).toLocaleString()}</span>
                    </div>
                    <div className="flex gap-1">
                      <Button variant="outline" size="sm" onClick={() => handleEdit(templateConfig)}>
                        <Edit className="h-4 w-4 mr-1" />{t('common.edit')}
                      </Button>
                      <Button variant="outline" size="sm" onClick={() => handleViewVersions(templateConfig)}>
                        <History className="h-4 w-4 mr-1" />{t('config.versions')}
                      </Button>
                      <Button variant="outline" size="sm" onClick={handleSyncToAllNodes} disabled={syncAllLoading || nodeConfigs.length === 0}>
                        {syncAllLoading ? <Loader2 className="h-4 w-4 mr-1 animate-spin" /> : <Download className="h-4 w-4 mr-1" />}
                        {t('config.syncToAllNodes')}
                      </Button>
                    </div>
                  </div>
                  <pre className="p-3 bg-muted rounded-md text-xs font-mono max-h-[300px] overflow-auto">{templateConfig.content}</pre>
                </div>
              ) : (
                <p className="text-sm text-muted-foreground py-4 text-center">{t('config.noTemplate')}</p>
              )}
            </TabsContent>
            <TabsContent value="nodes" className="mt-4">
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
                        <div className="flex gap-1">
                          <Button variant="ghost" size="sm" onClick={() => handleEdit(config)} title={t('common.edit')}><Edit className="h-4 w-4" /></Button>
                          <Button variant="ghost" size="sm" onClick={() => handleViewVersions(config)} title={t('config.versions')}><History className="h-4 w-4" /></Button>
                          {!config.match_template && (
                            <>
                              <Button variant="ghost" size="sm" onClick={() => handlePromote(config)} title={t('config.promoteToCluster')}><Upload className="h-4 w-4" /></Button>
                              <Button variant="ghost" size="sm" onClick={() => handleSyncFromTemplate(config)} title={t('config.syncFromTemplate')}><Download className="h-4 w-4" /></Button>
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
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('config.initFromNode')}</DialogTitle>
            <DialogDescription>{t('config.initFromNodeDesc')}</DialogDescription>
          </DialogHeader>
          <div className="space-y-4">
            <div>
              <Label>{t('config.selectNode')}</Label>
              <Select value={selectedNodeId} onValueChange={setSelectedNodeId}>
                <SelectTrigger className="w-full mt-2">
                  <SelectValue placeholder={t('config.selectNodePlaceholder')} />
                </SelectTrigger>
                <SelectContent>
                  {nodes.map((node) => (
                    <SelectItem key={node.id} value={String(node.id)}>
                      {node.host_name || `Node ${node.id}`} ({node.host_ip}) - {node.install_dir}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
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
            <Button onClick={handleInitConfigs} disabled={initLoading || !selectedNodeId}>
              {initLoading && <Loader2 className="h-4 w-4 mr-2 animate-spin" />}
              {t('config.initConfirm')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Edit Dialog */}
      <Dialog open={editDialogOpen} onOpenChange={setEditDialogOpen}>
        <DialogContent className="!max-w-5xl max-h-[90vh]">
          <DialogHeader>
            <DialogTitle>{t('config.editConfig')} - {editingConfig?.is_template ? t('config.clusterTemplate') : editingConfig?.host_name}</DialogTitle>
            <DialogDescription>{ConfigTypeNames[editingConfig?.config_type as ConfigType]}</DialogDescription>
          </DialogHeader>
          <div className="space-y-4">
            <div>
              <Label>{t('config.content')}</Label>
              <Textarea value={editContent} onChange={(e) => setEditContent(e.target.value)} className="font-mono text-sm h-[500px]" />
            </div>
            <div>
              <Label>{t('config.comment')}</Label>
              <Input value={editComment} onChange={(e) => setEditComment(e.target.value)} placeholder={t('config.commentPlaceholder')} />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setEditDialogOpen(false)}>{t('common.cancel')}</Button>
            <Button onClick={handleSave} disabled={saving}>{saving && <Loader2 className="h-4 w-4 mr-2 animate-spin" />}{t('common.save')}</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Version History Dialog */}
      <Dialog open={versionDialogOpen} onOpenChange={setVersionDialogOpen}>
        <DialogContent className="max-w-2xl max-h-[80vh]">
          <DialogHeader>
            <DialogTitle>{t('config.versionHistory')}</DialogTitle>
            <DialogDescription>{versionConfig?.is_template ? t('config.clusterTemplate') : versionConfig?.host_name}</DialogDescription>
          </DialogHeader>
          <div className="max-h-[500px] overflow-auto">
            {loadingVersions ? (
              <div className="flex items-center justify-center py-8"><Loader2 className="h-6 w-6 animate-spin" /></div>
            ) : versions.length === 0 ? (
              <p className="text-center py-8 text-muted-foreground">{t('config.noVersions')}</p>
            ) : (
              <div className="space-y-2">
                {versions.map((version) => (
                  <div key={version.id} className="border rounded-lg p-3 flex items-center justify-between">
                    <div>
                      <div className="flex items-center gap-2">
                        <Badge variant="secondary">v{version.version}</Badge>
                        <span className="text-sm text-muted-foreground">{new Date(version.created_at).toLocaleString()}</span>
                      </div>
                      {version.comment && <p className="text-sm text-muted-foreground mt-1">{version.comment}</p>}
                    </div>
                    {version.version !== versionConfig?.version && (
                      <Button variant="outline" size="sm" onClick={() => handleRollback(version.version)}>{t('config.rollback')}</Button>
                    )}
                  </div>
                ))}
              </div>
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
