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

import {useCallback, useEffect, useMemo, useState} from 'react';
import {useTranslations} from 'next-intl';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import {Badge} from '@/components/ui/badge';
import {Button} from '@/components/ui/button';
import {Separator} from '@/components/ui/separator';
import {ScrollArea} from '@/components/ui/scroll-area';
import {Checkbox} from '@/components/ui/checkbox';
import {toast} from 'sonner';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import {
  Ban,
  Database,
  Download,
  ExternalLink,
  FolderOpen,
  Layers3,
  Package,
  RefreshCw,
  RotateCcw,
} from 'lucide-react';
import type {
  OfficialDependenciesResponse,
  Plugin,
  PluginCategory,
  PluginDependency,
  PluginDependencyDisable,
} from '@/lib/services/plugin';
import {PluginService} from '@/lib/services/plugin';
import {PluginDependencyConfigSection} from './DependencyConfigDialog';

interface PluginDetailDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  plugin: Plugin;
  seatunnelVersion?: string;
  selectedProfileKeys?: string[];
  onSelectedProfileKeysChange?: (keys: string[]) => void;
  onDownload?: (plugin: Plugin, profileKeys: string[]) => void;
}

function getCategoryIcon(category: PluginCategory) {
  switch (category) {
    case 'source':
    case 'sink':
    case 'connector':
    case 'transform':
    default:
      return <Database className='h-5 w-5' />;
  }
}

function getCategoryColor(category: PluginCategory): string {
  switch (category) {
    case 'source':
      return 'bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-300';
    case 'sink':
      return 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-300';
    case 'transform':
      return 'bg-purple-100 text-purple-800 dark:bg-purple-900 dark:text-purple-300';
    default:
      return 'bg-slate-100 text-slate-800 dark:bg-slate-900 dark:text-slate-300';
  }
}

function supportsIsolatedDependency(version?: string): boolean {
  if (!version) {
    return false;
  }
  const parse = (input: string) =>
    input
      .split('.')
      .map((part) => Number.parseInt(part.split('-')[0] || '0', 10) || 0);
  const current = parse(version);
  const baseline = [2, 3, 12];
  for (let index = 0; index < baseline.length; index += 1) {
    const left = current[index] ?? 0;
    const right = baseline[index];
    if (left !== right) {
      return left > right;
    }
  }
  return true;
}

function dedupeStrings(values: string[]): string[] {
  return Array.from(new Set(values.filter(Boolean)));
}

function splitDependencies(items: PluginDependency[]) {
  const attachedConnectors = dedupeStrings(
    items
      .filter((item) => item.target_dir === 'connectors')
      .map((item) => item.artifact_id),
  );
  const extraDependencies = items.filter((item) => item.target_dir !== 'connectors');
  return {attachedConnectors, extraDependencies};
}

export function PluginDetailDialog({
  open,
  onOpenChange,
  plugin,
  seatunnelVersion,
  selectedProfileKeys = [],
  onSelectedProfileKeysChange,
  onDownload,
}: PluginDetailDialogProps) {
  const t = useTranslations();
  const [officialDeps, setOfficialDeps] =
    useState<OfficialDependenciesResponse | null>(null);
  const [loadingOfficialDeps, setLoadingOfficialDeps] = useState(false);
  const [operatingDependencyKey, setOperatingDependencyKey] = useState<string | null>(null);

  const loadOfficialDeps = useCallback(async () => {
    if (!plugin?.name) {
      return;
    }
    setLoadingOfficialDeps(true);
    try {
      const data = await PluginService.getOfficialDependencies(
        plugin.name,
        seatunnelVersion || plugin.version,
      );
      setOfficialDeps(data);
    } catch (error) {
      console.error('Failed to load official dependencies:', error);
      setOfficialDeps(null);
    } finally {
      setLoadingOfficialDeps(false);
    }
  }, [plugin?.name, plugin?.version, seatunnelVersion]);

  useEffect(() => {
    if (open && plugin?.name) {
      void loadOfficialDeps();
    }
  }, [loadOfficialDeps, open, plugin?.name]);

  const availableProfiles = useMemo(
    () => officialDeps?.profiles || [],
    [officialDeps?.profiles],
  );
  const hasMultiProfiles = availableProfiles.length > 1;
  const effectiveVersion = seatunnelVersion || plugin.version;
  const useIsolatedDeps = supportsIsolatedDependency(effectiveVersion);
  const disabledDependencies = useMemo(
    () => officialDeps?.disabled_dependencies || [],
    [officialDeps?.disabled_dependencies],
  );

  const effectiveDependencies = useMemo(() => {
    if (!officialDeps) {
      return [];
    }
    if (!hasMultiProfiles) {
      return officialDeps.effective_dependencies || [];
    }
    if (selectedProfileKeys.length === 0) {
      return [];
    }
    const selected = new Set(selectedProfileKeys);
    const merged = new Map<string, PluginDependency>();
    availableProfiles.forEach((profile) => {
      if (!selected.has(profile.profile_key)) {
        return;
      }
      (profile.items || []).forEach((item) => {
        if (item.disabled) {
          return;
        }
        const key = `${item.group_id}:${item.artifact_id}:${item.version}:${item.target_dir}`;
        if (!merged.has(key)) {
          merged.set(key, {
            group_id: item.group_id,
            artifact_id: item.artifact_id,
            version: item.version,
            target_dir: item.target_dir,
            source_type: 'official',
          });
        }
      });
    });
    return Array.from(merged.values());
  }, [availableProfiles, hasMultiProfiles, officialDeps, selectedProfileKeys]);

  const {attachedConnectors, extraDependencies} = useMemo(
    () => splitDependencies(effectiveDependencies),
    [effectiveDependencies],
  );
  const dependencyTargetDirs = useMemo(
    () => dedupeStrings(extraDependencies.map((item) => item.target_dir)),
    [extraDependencies],
  );

  const handleToggleProfile = (profileKey: string, checked: boolean) => {
    const next = new Set(selectedProfileKeys);
    if (checked) {
      next.add(profileKey);
    } else {
      next.delete(profileKey);
    }
    onSelectedProfileKeysChange?.(Array.from(next).sort());
  };

  const downloadDisabled = hasMultiProfiles && selectedProfileKeys.length === 0;

  const handleDisableOfficialDependency = async (dep: PluginDependency) => {
    const actionKey = `${dep.group_id}:${dep.artifact_id}:${dep.version}:${dep.target_dir}:disable`;
    setOperatingDependencyKey(actionKey);
    try {
      await PluginService.disableOfficialDependency(plugin.name, {
        seatunnel_version: effectiveVersion,
        group_id: dep.group_id,
        artifact_id: dep.artifact_id,
        version: dep.version,
        target_dir: dep.target_dir,
      });
      toast.success(t('plugin.disableOfficialDependencySuccess'));
      await loadOfficialDeps();
    } catch (error) {
      const errorMsg =
        error instanceof Error ? error.message : t('plugin.disableOfficialDependencyFailed');
      toast.error(errorMsg);
    } finally {
      setOperatingDependencyKey(null);
    }
  };

  const handleEnableOfficialDependency = async (item: PluginDependencyDisable) => {
    const actionKey = `${item.id}:enable`;
    setOperatingDependencyKey(actionKey);
    try {
      await PluginService.enableOfficialDependency(plugin.name, item.id);
      toast.success(t('plugin.enableOfficialDependencySuccess'));
      await loadOfficialDeps();
    } catch (error) {
      const errorMsg =
        error instanceof Error ? error.message : t('plugin.enableOfficialDependencyFailed');
      toast.error(errorMsg);
    } finally {
      setOperatingDependencyKey(null);
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='w-[96vw] max-w-[96vw] sm:max-w-[980px] lg:max-w-[1100px] max-h-[88vh] overflow-hidden p-0 gap-0'>
        <DialogHeader className='px-6 pt-6 pb-4'>
          <div className='flex items-center gap-3'>
            <div className={`p-2 rounded-lg ${getCategoryColor(plugin.category)}`}>
              {getCategoryIcon(plugin.category)}
            </div>
            <div>
              <DialogTitle className='text-xl'>
                {plugin.display_name || plugin.name}
              </DialogTitle>
              <DialogDescription className='text-sm'>
                {plugin.name}
              </DialogDescription>
            </div>
          </div>
        </DialogHeader>

        <ScrollArea className='max-h-[calc(88vh-92px)]'>
          <div className='space-y-6 px-6 pb-6'>
            <div className='space-y-3'>
              <div className='flex items-center gap-2'>
                <Badge variant='outline' className={getCategoryColor(plugin.category)}>
                  {t(`plugin.category.${plugin.category}`)}
                </Badge>
                <Badge variant='secondary'>v{plugin.version}</Badge>
              </div>

              <p className='text-sm text-muted-foreground'>
                {plugin.description || t('plugin.noDescription')}
              </p>

              {plugin.doc_url && (
                <Button variant='outline' size='sm' asChild>
                  <a href={plugin.doc_url} target='_blank' rel='noopener noreferrer'>
                    <ExternalLink className='h-4 w-4 mr-2' />
                    {t('plugin.viewDocumentation')}
                  </a>
                </Button>
              )}
            </div>

            <Separator />

            <div className='space-y-3'>
              <h4 className='font-medium flex items-center gap-2'>
                <Package className='h-4 w-4' />
                {t('plugin.mavenCoordinates')}
              </h4>
              <div className='bg-muted p-3 rounded-md font-mono text-sm'>
                <div>
                  <span className='text-muted-foreground'>groupId:</span> {plugin.group_id}
                </div>
                <div>
                  <span className='text-muted-foreground'>artifactId:</span> {plugin.artifact_id}
                </div>
                <div>
                  <span className='text-muted-foreground'>version:</span> {plugin.version}
                </div>
              </div>
            </div>

            <Separator />

            <div className='space-y-3'>
              <div>
                <h4 className='font-medium flex items-center gap-2'>
                  <Layers3 className='h-4 w-4' />
                  {t('plugin.officialDependencies')}
                </h4>
                <p className='text-xs text-muted-foreground mt-1'>
                  {t('plugin.officialDependenciesDesc')}
                </p>
              </div>

              {loadingOfficialDeps ? (
                <div className='text-sm text-muted-foreground py-4 text-center'>
                  {t('common.loading')}
                </div>
              ) : !officialDeps ? (
                <div className='text-sm text-muted-foreground py-4 text-center bg-muted/30 rounded-md'>
                  {t('plugin.officialDependenciesUnavailable')}
                </div>
              ) : (
                <div className='space-y-4'>
                  {hasMultiProfiles && (
                    <div className='space-y-2'>
                      <div className='text-sm font-medium'>
                        {t('plugin.profileSelection')}
                      </div>
                      <div className='grid gap-2 rounded-lg border p-3'>
                        {availableProfiles.map((profile) => {
                          const itemCount = (profile.items || []).length;
                          return (
                            <label
                              key={profile.id}
                              className='flex items-center gap-3 rounded-md border bg-muted/20 px-3 py-2'
                            >
                              <Checkbox
                                checked={selectedProfileKeys.includes(profile.profile_key)}
                                onCheckedChange={(checked) =>
                                  handleToggleProfile(profile.profile_key, checked === true)
                                }
                              />
                              <div className='flex min-w-0 flex-1 flex-col'>
                                <span className='text-sm font-medium'>
                                  {profile.profile_name || profile.profile_key}
                                </span>
                                <span className='text-xs text-muted-foreground'>
                                  {itemCount > 0
                                    ? t('plugin.profileDependencyCount', {count: itemCount})
                                    : t('plugin.noExtraDependencies')}
                                </span>
                              </div>
                            </label>
                          );
                        })}
                      </div>
                    </div>
                  )}

                  {attachedConnectors.length > 0 && (
                    <div className='space-y-2'>
                      <div className='text-sm font-medium'>
                        {t('plugin.attachedConnectors')}
                      </div>
                      <div className='flex flex-wrap gap-2'>
                        {attachedConnectors.map((artifact) => (
                          <Badge key={artifact} variant='secondary'>
                            {artifact}
                          </Badge>
                        ))}
                      </div>
                    </div>
                  )}

                  {extraDependencies.length > 0 ? (
                    <div className='space-y-3'>
                      <div className='flex items-center justify-between gap-2'>
                        <div>
                          <h5 className='text-sm font-medium'>
                            {t('plugin.extraDependencies')} ({extraDependencies.length})
                          </h5>
                          <p className='text-xs text-muted-foreground mt-1'>
                            {t('plugin.extraDependenciesDesc')}
                          </p>
                        </div>
                        {dependencyTargetDirs.length > 0 && (
                          <div className='flex flex-wrap justify-end gap-2'>
                            {dependencyTargetDirs.map((targetDir) => (
                              <Badge key={targetDir} variant='outline' className='max-w-[260px] whitespace-normal break-all'>
                                {targetDir}
                              </Badge>
                            ))}
                          </div>
                        )}
                      </div>
                      <div className='overflow-x-auto rounded-lg border'>
                        <Table className='min-w-[760px]'>
                          <TableHeader>
                            <TableRow>
                              <TableHead>{t('plugin.groupId')}</TableHead>
                              <TableHead>{t('plugin.artifactId')}</TableHead>
                              <TableHead>{t('plugin.version')}</TableHead>
                              <TableHead>{t('plugin.targetDir')}</TableHead>
                            </TableRow>
                          </TableHeader>
                          <TableBody>
                            {extraDependencies.map((dep, index) => (
                              <TableRow key={`${dep.group_id}:${dep.artifact_id}:${index}`}>
                                <TableCell className='font-mono text-xs whitespace-nowrap'>
                                  {dep.group_id}
                                </TableCell>
                                <TableCell className='font-mono text-xs whitespace-nowrap'>
                                  {dep.artifact_id}
                                </TableCell>
                                <TableCell className='font-mono text-xs whitespace-nowrap'>
                                  {dep.version}
                                </TableCell>
                                <TableCell className='min-w-[220px] align-top'>
                                  <Badge
                                    variant='outline'
                                    className='text-xs whitespace-normal break-all text-left'
                                  >
                                    {dep.target_dir}
                                  </Badge>
                                </TableCell>
                              </TableRow>
                            ))}
                          </TableBody>
                        </Table>
                      </div>
                    </div>
                  ) : officialDeps.dependency_status === 'not_required' ? (
                    <div className='text-sm text-muted-foreground py-4 text-center bg-muted/30 rounded-md'>
                      {t('plugin.officialDependenciesNotRequired')}
                    </div>
                  ) : hasMultiProfiles && selectedProfileKeys.length === 0 ? (
                    <div className='text-sm text-muted-foreground py-4 text-center bg-muted/30 rounded-md'>
                      {t('plugin.selectProfilesToPreview')}
                    </div>
                  ) : (
                    <div className='text-sm text-muted-foreground py-4 text-center bg-muted/30 rounded-md'>
                      {t('plugin.officialDependenciesUnknown')}
                    </div>
                  )}

                  {effectiveDependencies.length > 0 && (
                    <div className='space-y-3'>
                      <div>
                        <h5 className='text-sm font-medium'>
                          {t('plugin.activeOfficialDependencies')}
                        </h5>
                        <p className='mt-1 text-xs text-muted-foreground'>
                          {t('plugin.activeOfficialDependenciesDesc')}
                        </p>
                      </div>
                      <div className='overflow-x-auto rounded-lg border'>
                        <Table className='min-w-[860px]'>
                          <TableHeader>
                            <TableRow>
                              <TableHead>{t('plugin.groupId')}</TableHead>
                              <TableHead>{t('plugin.artifactId')}</TableHead>
                              <TableHead>{t('plugin.version')}</TableHead>
                              <TableHead>{t('plugin.targetDir')}</TableHead>
                              <TableHead className='w-[96px]'>{t('common.actions')}</TableHead>
                            </TableRow>
                          </TableHeader>
                          <TableBody>
                            {effectiveDependencies.map((dep) => {
                              const actionKey = `${dep.group_id}:${dep.artifact_id}:${dep.version}:${dep.target_dir}:disable`;
                              const busy = operatingDependencyKey === actionKey;
                              return (
                                <TableRow key={`${dep.group_id}:${dep.artifact_id}:${dep.version}:${dep.target_dir}`}>
                                  <TableCell className='font-mono text-xs whitespace-nowrap'>
                                    {dep.group_id}
                                  </TableCell>
                                  <TableCell className='font-mono text-xs whitespace-nowrap'>
                                    {dep.artifact_id}
                                  </TableCell>
                                  <TableCell className='font-mono text-xs whitespace-nowrap'>
                                    {dep.version}
                                  </TableCell>
                                  <TableCell className='font-mono text-xs break-all'>
                                    {dep.target_dir}
                                  </TableCell>
                                  <TableCell>
                                    <Button
                                      variant='ghost'
                                      size='sm'
                                      className='text-amber-600 hover:text-amber-700'
                                      disabled={busy}
                                      onClick={() => handleDisableOfficialDependency(dep)}
                                    >
                                      {busy ? <RefreshCw className='h-4 w-4 animate-spin' /> : <Ban className='h-4 w-4' />}
                                    </Button>
                                  </TableCell>
                                </TableRow>
                              );
                            })}
                          </TableBody>
                        </Table>
                      </div>
                    </div>
                  )}

                  {disabledDependencies.length > 0 && (
                    <div className='space-y-3'>
                      <div>
                        <h5 className='text-sm font-medium'>
                          {t('plugin.disabledOfficialDependencies')}
                        </h5>
                        <p className='mt-1 text-xs text-muted-foreground'>
                          {t('plugin.disabledOfficialDependenciesDesc')}
                        </p>
                      </div>
                      <div className='overflow-x-auto rounded-lg border'>
                        <Table className='min-w-[860px]'>
                          <TableHeader>
                            <TableRow>
                              <TableHead>{t('plugin.groupId')}</TableHead>
                              <TableHead>{t('plugin.artifactId')}</TableHead>
                              <TableHead>{t('plugin.version')}</TableHead>
                              <TableHead>{t('plugin.targetDir')}</TableHead>
                              <TableHead className='w-[96px]'>{t('common.actions')}</TableHead>
                            </TableRow>
                          </TableHeader>
                          <TableBody>
                            {disabledDependencies.map((item) => {
                              const actionKey = `${item.id}:enable`;
                              const busy = operatingDependencyKey === actionKey;
                              return (
                                <TableRow key={item.id}>
                                  <TableCell className='font-mono text-xs whitespace-nowrap'>
                                    {item.group_id}
                                  </TableCell>
                                  <TableCell className='font-mono text-xs whitespace-nowrap'>
                                    {item.artifact_id}
                                  </TableCell>
                                  <TableCell className='font-mono text-xs whitespace-nowrap'>
                                    {item.version}
                                  </TableCell>
                                  <TableCell className='font-mono text-xs break-all'>
                                    {item.target_dir}
                                  </TableCell>
                                  <TableCell>
                                    <Button
                                      variant='ghost'
                                      size='sm'
                                      className='text-emerald-600 hover:text-emerald-700'
                                      disabled={busy}
                                      onClick={() => handleEnableOfficialDependency(item)}
                                    >
                                      {busy ? <RefreshCw className='h-4 w-4 animate-spin' /> : <RotateCcw className='h-4 w-4' />}
                                    </Button>
                                  </TableCell>
                                </TableRow>
                              );
                            })}
                          </TableBody>
                        </Table>
                      </div>
                    </div>
                  )}
                </div>
              )}
            </div>

            <Separator />

            <PluginDependencyConfigSection
              pluginName={plugin.name}
              seatunnelVersion={effectiveVersion}
              compact
              onChanged={loadOfficialDeps}
            />

            <Separator />

            <div className='space-y-3'>
              <h4 className='font-medium flex items-center gap-2'>
                <FolderOpen className='h-4 w-4' />
                {t('plugin.installPath')}
              </h4>
              <div className='text-sm space-y-2'>
                <div className='flex items-start gap-2'>
                  <Badge variant='outline'>{t('plugin.connectorPath')}</Badge>
                  <code className='text-xs bg-muted px-2 py-1 rounded'>connectors/</code>
                </div>
                {dependencyTargetDirs.length > 0 ? (
                  dependencyTargetDirs.map((targetDir) => (
                    <div key={targetDir} className='flex items-start gap-2'>
                      <Badge variant='outline'>
                        {targetDir === 'lib'
                          ? t('plugin.libPath')
                          : t('plugin.pluginDependencyPath')}
                      </Badge>
                      <code className='text-xs bg-muted px-2 py-1 rounded break-all'>
                        {targetDir}
                      </code>
                    </div>
                  ))
                ) : (
                  <p className='text-xs text-muted-foreground'>
                    {useIsolatedDeps
                      ? t('plugin.installPathIsolatedDesc')
                      : t('plugin.installPathLegacyDesc')}
                  </p>
                )}
              </div>
            </div>

            {onDownload && (
              <div className='flex justify-end'>
                <Button
                  onClick={() => onDownload(plugin, selectedProfileKeys)}
                  disabled={downloadDisabled}
                >
                  <Download className='mr-2 h-4 w-4' />
                  {t('plugin.download')}
                </Button>
              </div>
            )}
          </div>
        </ScrollArea>
      </DialogContent>
    </Dialog>
  );
}
