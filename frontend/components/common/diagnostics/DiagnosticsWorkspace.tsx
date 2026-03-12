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
import Link from 'next/link';
import {usePathname, useRouter, useSearchParams} from 'next/navigation';
import {useTranslations} from 'next-intl';
import {Bug, RefreshCw, Settings} from 'lucide-react';
import {toast} from 'sonner';
import services from '@/lib/services';
import type {
  DiagnosticsTabKey,
  DiagnosticsWorkspaceBootstrapData,
} from '@/lib/services/diagnostics';
import {Badge} from '@/components/ui/badge';
import {Button} from '@/components/ui/button';
import {Card, CardContent, CardHeader, CardTitle} from '@/components/ui/card';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import {Tabs, TabsContent, TabsList, TabsTrigger} from '@/components/ui/tabs';
import {DiagnosticsErrorCenter} from './DiagnosticsErrorCenter';
import {DiagnosticsInspectionCenter} from './DiagnosticsInspectionCenter';
import {AutoPolicyConfigPanel} from './AutoPolicyConfigPanel';

function resolveTab(
  tab: string | null,
  fallback: DiagnosticsTabKey = 'errors',
): DiagnosticsTabKey {
  if (tab === 'errors' || tab === 'inspections') {
    return tab;
  }
  return fallback;
}

export function DiagnosticsWorkspace() {
  const t = useTranslations('diagnosticsCenter');
  const commonT = useTranslations('common');
  const searchParams = useSearchParams();
  const router = useRouter();
  const pathname = usePathname();

  const [loading, setLoading] = useState(true);
  const [bootstrap, setBootstrap] =
    useState<DiagnosticsWorkspaceBootstrapData | null>(null);
  const [autoPolicyOpen, setAutoPolicyOpen] = useState(false);

  const activeTab = useMemo(
    () =>
      resolveTab(
        searchParams.get('tab'),
        bootstrap?.default_tab ? resolveTab(bootstrap.default_tab) : 'errors',
      ),
    [bootstrap?.default_tab, searchParams],
  );

  const selectedClusterId = searchParams.get('cluster_id') || 'all';
  const source = searchParams.get('source') || '';
  const alertId = searchParams.get('alert_id') || '';
  const groupId = searchParams.get('group_id') || '';
  const reportId = searchParams.get('report_id') || '';
  const findingId = searchParams.get('finding_id') || '';
  const taskId = searchParams.get('task_id') || '';
  const hasWorkspaceContext = Boolean(
    selectedClusterId !== 'all' ||
      source ||
      alertId ||
      groupId ||
      reportId ||
      findingId ||
      taskId,
  );

  const updateQuery = useCallback(
    (updates: Record<string, string | null>) => {
      const next = new URLSearchParams(searchParams.toString());
      Object.entries(updates).forEach(([key, value]) => {
        if (!value || value === 'all') {
          next.delete(key);
          return;
        }
        next.set(key, value);
      });

      const queryString = next.toString();
      router.replace(queryString ? `${pathname}?${queryString}` : pathname);
    },
    [pathname, router, searchParams],
  );

  const loadBootstrap = useCallback(async () => {
    setLoading(true);
    try {
      const clusterIDValue =
        selectedClusterId !== 'all'
          ? Number.parseInt(selectedClusterId, 10)
          : 0;
      const result = await services.diagnostics.getWorkspaceBootstrapSafe({
        cluster_id: clusterIDValue > 0 ? clusterIDValue : undefined,
        source: source || undefined,
        alert_id: alertId || undefined,
      });

      if (!result.success || !result.data) {
        toast.error(result.error || t('loadError'));
        setBootstrap(null);
        return;
      }
      setBootstrap(result.data);
    } finally {
      setLoading(false);
    }
  }, [alertId, selectedClusterId, source, t]);

  useEffect(() => {
    void loadBootstrap();
  }, [loadBootstrap]);

  const selectedClusterName = useMemo(() => {
    if (!bootstrap || selectedClusterId === 'all') {
      return '';
    }
    const cluster = (bootstrap.cluster_options || []).find(
      (item) => String(item.cluster_id) === selectedClusterId,
    );
    return cluster?.cluster_name || '';
  }, [bootstrap, selectedClusterId]);
  const entrySourceLabel = useMemo(() => {
    switch (source) {
      case 'alerts':
        return t('tasks.entrySource.alerts');
      case 'inspection-finding':
        return t('tasks.entrySource.inspectionFinding');
      case 'cluster-detail':
        return t('tasks.entrySource.clusterDetail');
      case 'cluster-detail-summary':
        return t('tasks.entrySource.clusterDetailSummary');
      default:
        return source;
    }
  }, [source, t]);

  const tabs = bootstrap?.tabs || [];

  return (
    <div className='space-y-4'>
      <div className='flex items-center gap-3'>
        <Bug className='h-8 w-8 shrink-0 text-primary' />
        <div>
          <h1 className='text-2xl font-bold tracking-tight'>{t('title')}</h1>
          <p className='mt-1 text-muted-foreground'>{t('subtitle')}</p>
        </div>
      </div>

      <Card>
        <CardHeader className='space-y-4'>
          <div className='flex flex-col gap-3 xl:flex-row xl:items-center xl:justify-between'>
            <CardTitle>{t('workspaceTitle')}</CardTitle>
            <div className='flex flex-wrap gap-2'>
              <Button variant='outline' onClick={() => setAutoPolicyOpen(true)}>
                <Settings className='mr-2 h-4 w-4' />
                自动巡检设置
              </Button>
              <Button variant='outline' onClick={() => void loadBootstrap()}>
                <RefreshCw className='mr-2 h-4 w-4' />
                {commonT('refresh')}
              </Button>
              {hasWorkspaceContext ? (
                <Button
                  variant='outline'
                  onClick={() =>
                    updateQuery({
                      cluster_id: null,
                      source: null,
                      alert_id: null,
                      group_id: null,
                      report_id: null,
                      finding_id: null,
                      task_id: null,
                    })
                  }
                >
                  {t('clearContext')}
                </Button>
              ) : null}
            </div>
          </div>

          <div className='grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-4'>
            <div className='w-full'>
              <Select
                value={selectedClusterId}
                onValueChange={(value) =>
                  updateQuery({
                    cluster_id: value === 'all' ? null : value,
                    source: null,
                    alert_id: null,
                    group_id: null,
                    report_id: null,
                    finding_id: null,
                    task_id: null,
                  })
                }
              >
                <SelectTrigger>
                  <SelectValue placeholder={t('filters.cluster')} />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value='all'>{t('filters.allClusters')}</SelectItem>
                  {(bootstrap?.cluster_options || []).map((cluster) => (
                    <SelectItem
                      key={cluster.cluster_id}
                      value={String(cluster.cluster_id)}
                    >
                      {cluster.cluster_name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className='md:col-span-1 xl:col-span-3'>
              <div className='flex min-h-10 flex-wrap items-center gap-2 rounded-md border px-3 py-2 text-sm'>
                <Badge variant='outline'>
                  {selectedClusterName
                    ? t('context.clusterScoped', {name: selectedClusterName})
                    : t('context.global')}
                </Badge>
                {source ? (
                  <Badge variant='secondary'>
                    {t('context.source', {source: entrySourceLabel})}
                  </Badge>
                ) : null}
                {alertId ? (
                  <Badge variant='secondary'>
                    {t('context.alert', {id: alertId})}
                  </Badge>
                ) : null}
                {selectedClusterId !== 'all' ? (
                  <Button asChild variant='link' className='h-auto px-0'>
                    <Link href={`/clusters/${selectedClusterId}`}>
                      {t('context.goToCluster')}
                    </Link>
                  </Button>
                ) : null}
              </div>
            </div>
          </div>
        </CardHeader>
      </Card>

      <Tabs
        value={activeTab}
        onValueChange={(value) =>
          updateQuery({tab: resolveTab(value) as string})
        }
      >
        <TabsList className='grid w-full grid-cols-2 gap-1 md:w-[320px]'>
          <TabsTrigger value='errors'>{t('tabs.errors')}</TabsTrigger>
          <TabsTrigger value='inspections'>{t('tabs.inspections')}</TabsTrigger>
        </TabsList>

        {loading && !bootstrap ? (
          <Card className='mt-4'>
            <CardContent className='py-8 text-sm text-muted-foreground'>
              {t('loading')}
            </CardContent>
          </Card>
        ) : null}

        {tabs.map((tab) => (
          <TabsContent key={tab.key} value={tab.key} className='mt-4'>
            {tab.key === 'errors' ? (
              <DiagnosticsErrorCenter
                clusterId={
                  selectedClusterId !== 'all'
                    ? Number.parseInt(selectedClusterId, 10)
                    : undefined
                }
                clusterName={selectedClusterName || undefined}
                groupId={groupId ? Number.parseInt(groupId, 10) : undefined}
                onSelectGroup={(value) =>
                  updateQuery({group_id: value ? String(value) : null})
                }
              />
            ) : tab.key === 'inspections' ? (
              <DiagnosticsInspectionCenter
                clusterId={
                  selectedClusterId !== 'all'
                    ? Number.parseInt(selectedClusterId, 10)
                    : undefined
                }
                clusterName={selectedClusterName || undefined}
                reportId={reportId ? Number.parseInt(reportId, 10) : undefined}
                onSelectReport={(value) =>
                  updateQuery({report_id: value ? String(value) : null})
                }
              />
            ) : null}
          </TabsContent>
        ))}
      </Tabs>

      <AutoPolicyConfigPanel
        open={autoPolicyOpen}
        onOpenChange={setAutoPolicyOpen}
        clusterOptions={bootstrap?.cluster_options || []}
      />
    </div>
  );
}
