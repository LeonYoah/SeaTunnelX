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
import {useSearchParams} from 'next/navigation';
import {useTranslations} from 'next-intl';
import {RefreshCw} from 'lucide-react';
import {toast} from 'sonner';
import services from '@/lib/services';
import type {
  AlertDisplayStatus,
  AlertInstance,
  AlertInstanceStats,
  AlertSeverity,
  AlertSourceType,
} from '@/lib/services/monitoring';
import {Badge} from '@/components/ui/badge';
import {Button} from '@/components/ui/button';
import {Card, CardContent, CardHeader, CardTitle} from '@/components/ui/card';
import {Input} from '@/components/ui/input';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';

type ClusterOption = {
  id: string;
  name: string;
};

const EMPTY_STATS: AlertInstanceStats = {
  firing: 0,
  resolved: 0,
  closed: 0,
};

function formatDateTime(value?: string | null): string {
  if (!value) {
    return '-';
  }
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) {
    return value;
  }
  return parsed.toLocaleString();
}

function toRFC3339(value: string): string | undefined {
  if (!value) {
    return undefined;
  }
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) {
    return undefined;
  }
  return parsed.toISOString();
}

function resolveSeverityVariant(
  severity: string,
): 'default' | 'secondary' | 'outline' | 'destructive' {
  const normalized = severity.trim().toLowerCase();
  if (normalized === 'critical') {
    return 'destructive';
  }
  if (normalized === 'warning') {
    return 'secondary';
  }
  return 'outline';
}

function resolveStatusVariant(
  status: AlertDisplayStatus,
): 'default' | 'secondary' | 'outline' | 'destructive' {
  switch (status) {
    case 'firing':
      return 'destructive';
    case 'resolved':
      return 'secondary';
    case 'closed':
      return 'outline';
    default:
      return 'outline';
  }
}

function isSilenceActive(value?: string | null): boolean {
  if (!value) {
    return false;
  }
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) {
    return false;
  }
  return parsed.getTime() > Date.now();
}

function resolveLastChangedAt(alert: AlertInstance): string | null {
  return alert.closed_at || alert.resolved_at || alert.last_seen_at || null;
}

export function MonitoringAlertsCenter() {
  const t = useTranslations('monitoringCenter');
  const searchParams = useSearchParams();

  const [clusterOptions, setClusterOptions] = useState<ClusterOption[]>([]);
  const [clusterFilter, setClusterFilter] = useState<string>('all');
  const [sourceFilter, setSourceFilter] = useState<string>('all');
  const [statusFilter, setStatusFilter] = useState<string>('all');
  const [startTimeFilter, setStartTimeFilter] = useState<string>('');
  const [endTimeFilter, setEndTimeFilter] = useState<string>('');
  const [page, setPage] = useState<number>(1);
  const [pageSize, setPageSize] = useState<string>('50');

  const [alerts, setAlerts] = useState<AlertInstance[]>([]);
  const [stats, setStats] = useState<AlertInstanceStats>(EMPTY_STATS);
  const [total, setTotal] = useState<number>(0);
  const [loading, setLoading] = useState<boolean>(true);
  const [actingAlertId, setActingAlertId] = useState<string | null>(null);

  const pageSizeNumber = useMemo(
    () => Number.parseInt(pageSize, 10) || 50,
    [pageSize],
  );

  const loadClusters = useCallback(async () => {
    const healthResult = await services.monitoring.getClustersHealthSafe();
    if (healthResult.success && healthResult.data) {
      const options = (healthResult.data.clusters || [])
        .map((cluster) => ({
          id: String(cluster.cluster_id),
          name: cluster.cluster_name || `Cluster-${cluster.cluster_id}`,
        }))
        .sort((a, b) => Number.parseInt(a.id, 10) - Number.parseInt(b.id, 10));
      setClusterOptions(options);
      return;
    }

    try {
      const data = await services.cluster.getClusters({
        current: 1,
        size: 100,
      });
      setClusterOptions(
        (data.clusters || []).map((cluster) => ({
          id: String(cluster.id),
          name: cluster.name,
        })),
      );
    } catch {
      setClusterOptions([]);
    }
  }, []);

  const loadAlerts = useCallback(async () => {
    setLoading(true);
    try {
      const result = await services.monitoring.getAlertInstancesSafe({
        cluster_id: clusterFilter === 'all' ? undefined : clusterFilter,
        source_type:
          sourceFilter === 'all'
            ? undefined
            : (sourceFilter as AlertSourceType),
        status:
          statusFilter === 'all'
            ? undefined
            : (statusFilter as AlertDisplayStatus),
        start_time: toRFC3339(startTimeFilter),
        end_time: toRFC3339(endTimeFilter),
        page,
        page_size: pageSizeNumber,
      });

      if (!result.success || !result.data) {
        toast.error(result.error || t('alerts.loadError'));
        setAlerts([]);
        setStats(EMPTY_STATS);
        setTotal(0);
        return;
      }

      setAlerts(result.data.alerts || []);
      setStats(result.data.stats || EMPTY_STATS);
      setTotal(result.data.total || 0);
    } finally {
      setLoading(false);
    }
  }, [
    clusterFilter,
    endTimeFilter,
    page,
    pageSizeNumber,
    sourceFilter,
    startTimeFilter,
    statusFilter,
    t,
  ]);

  useEffect(() => {
    loadClusters();
  }, [loadClusters]);

  useEffect(() => {
    const clusterIDFromQuery = searchParams.get('cluster_id');
    if (!clusterIDFromQuery) {
      return;
    }
    setClusterFilter(clusterIDFromQuery);
    setPage(1);
  }, [searchParams]);

  useEffect(() => {
    loadAlerts();
  }, [loadAlerts]);

  const resolveSourceLabel = useCallback(
    (sourceType: AlertSourceType) => {
      if (sourceType === 'local_process_event') {
        return t('alerts.sourceTypes.local_process_event');
      }
      if (sourceType === 'remote_alertmanager') {
        return t('alerts.sourceTypes.remote_alertmanager');
      }
      return sourceType;
    },
    [t],
  );

  const resolveStatusLabel = useCallback(
    (status: AlertDisplayStatus) => {
      if (status === 'resolved') {
        return t('alerts.statuses.resolved');
      }
      if (status === 'closed') {
        return t('alerts.statuses.closed');
      }
      return t('alerts.statuses.firing');
    },
    [t],
  );

  const resolveSeverityLabel = useCallback(
    (severity: AlertSeverity | string) => {
      const normalized = String(severity || '')
        .trim()
        .toLowerCase();
      if (normalized === 'critical') {
        return t('alertSeverity.critical');
      }
      if (normalized === 'warning') {
        return t('alertSeverity.warning');
      }
      return severity || '-';
    },
    [t],
  );

  const totalPages = useMemo(() => {
    if (total <= 0) {
      return 1;
    }
    return Math.max(1, Math.ceil(total / pageSizeNumber));
  }, [pageSizeNumber, total]);

  const handleSilence = async (alert: AlertInstance) => {
    setActingAlertId(alert.alert_id);
    try {
      const result = await services.monitoring.silenceAlertInstanceSafe(
        alert.alert_id,
        {duration_minutes: 30},
      );
      if (!result.success) {
        toast.error(result.error || t('alerts.silenceError'));
        return;
      }
      toast.success(t('alerts.silenceSuccess'));
      await loadAlerts();
    } finally {
      setActingAlertId(null);
    }
  };

  const handleClose = async (alert: AlertInstance) => {
    setActingAlertId(alert.alert_id);
    try {
      const result = await services.monitoring.closeAlertInstanceSafe(
        alert.alert_id,
      );
      if (!result.success) {
        toast.error(result.error || t('alerts.closeError'));
        return;
      }
      toast.success(t('alerts.closeSuccess'));
      await loadAlerts();
    } finally {
      setActingAlertId(null);
    }
  };

  const renderMarkers = (alert: AlertInstance) => {
    const silenceActive = isSilenceActive(alert.silenced_until);
    const hasAcknowledged = Boolean(alert.acknowledged_at);
    const hasNote = Boolean(alert.latest_note?.trim());

    if (!silenceActive && !hasAcknowledged && !hasNote) {
      return null;
    }

    return (
      <div className='mt-2 flex flex-wrap gap-2'>
        {hasAcknowledged ? (
          <Badge variant='secondary'>{t('alerts.markers.acknowledged')}</Badge>
        ) : null}
        {silenceActive ? (
          <Badge variant='outline'>
            {t('alerts.markers.silencedUntil', {
              time: formatDateTime(alert.silenced_until),
            })}
          </Badge>
        ) : null}
        {hasNote ? (
          <Badge variant='outline'>
            {t('alerts.markers.latestNote', {note: alert.latest_note || ''})}
          </Badge>
        ) : null}
      </div>
    );
  };

  return (
    <div className='space-y-4'>
      <Card>
        <CardHeader className='space-y-4'>
          <CardTitle>{t('alerts.title')}</CardTitle>
          <div className='grid grid-cols-1 gap-2 md:grid-cols-2 xl:grid-cols-6'>
            <div className='w-full'>
              <Select
                value={clusterFilter}
                onValueChange={(value) => {
                  setClusterFilter(value);
                  setPage(1);
                }}
              >
                <SelectTrigger>
                  <SelectValue placeholder={t('alerts.clusterFilter')} />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value='all'>{t('alerts.allClusters')}</SelectItem>
                  {clusterOptions.map((cluster) => (
                    <SelectItem key={cluster.id} value={cluster.id}>
                      {cluster.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className='w-full'>
              <Select
                value={sourceFilter}
                onValueChange={(value) => {
                  setSourceFilter(value);
                  setPage(1);
                }}
              >
                <SelectTrigger>
                  <SelectValue placeholder={t('alerts.sourceFilter')} />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value='all'>{t('alerts.allSources')}</SelectItem>
                  <SelectItem value='local_process_event'>
                    {t('alerts.sourceTypes.local_process_event')}
                  </SelectItem>
                  <SelectItem value='remote_alertmanager'>
                    {t('alerts.sourceTypes.remote_alertmanager')}
                  </SelectItem>
                </SelectContent>
              </Select>
            </div>

            <div className='w-full'>
              <Select
                value={statusFilter}
                onValueChange={(value) => {
                  setStatusFilter(value);
                  setPage(1);
                }}
              >
                <SelectTrigger>
                  <SelectValue placeholder={t('alerts.statusFilter')} />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value='all'>{t('alerts.allStatuses')}</SelectItem>
                  <SelectItem value='firing'>
                    {t('alerts.statuses.firing')}
                  </SelectItem>
                  <SelectItem value='resolved'>
                    {t('alerts.statuses.resolved')}
                  </SelectItem>
                  <SelectItem value='closed'>
                    {t('alerts.statuses.closed')}
                  </SelectItem>
                </SelectContent>
              </Select>
            </div>

            <Input
              type='datetime-local'
              value={startTimeFilter}
              onChange={(event) => {
                setStartTimeFilter(event.target.value);
                setPage(1);
              }}
              placeholder={t('alerts.startTime')}
            />

            <Input
              type='datetime-local'
              value={endTimeFilter}
              onChange={(event) => {
                setEndTimeFilter(event.target.value);
                setPage(1);
              }}
              placeholder={t('alerts.endTime')}
            />

            <Button
              variant='outline'
              onClick={loadAlerts}
              disabled={loading}
              className='w-full xl:w-auto'
            >
              <RefreshCw className='mr-2 h-4 w-4' />
              {t('refresh')}
            </Button>
          </div>

          <div className='flex flex-wrap gap-2'>
            <Badge variant='outline'>{`${t('alerts.totalCount')}: ${total}`}</Badge>
            <Badge variant='destructive'>{`${t('alerts.firingCount')}: ${stats.firing}`}</Badge>
            <Badge variant='secondary'>{`${t('alerts.resolvedCount')}: ${stats.resolved}`}</Badge>
            <Badge variant='outline'>{`${t('alerts.closedCount')}: ${stats.closed}`}</Badge>
          </div>
        </CardHeader>

        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t('alerts.cluster')}</TableHead>
                <TableHead>{t('alerts.sourceType')}</TableHead>
                <TableHead>{t('alerts.alertName')}</TableHead>
                <TableHead>{t('alerts.severity')}</TableHead>
                <TableHead>{t('alerts.status')}</TableHead>
                <TableHead>{t('alerts.summary')}</TableHead>
                <TableHead>{t('alerts.firstFiredAt')}</TableHead>
                <TableHead>{t('alerts.lastChangedAt')}</TableHead>
                <TableHead>{t('actions')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {loading ? (
                <TableRow>
                  <TableCell
                    colSpan={9}
                    className='text-center text-muted-foreground'
                  >
                    {t('loading')}
                  </TableCell>
                </TableRow>
              ) : !alerts.length ? (
                <TableRow>
                  <TableCell
                    colSpan={9}
                    className='text-center text-muted-foreground'
                  >
                    {t('alerts.noAlerts')}
                  </TableCell>
                </TableRow>
              ) : (
                alerts.map((alert) => {
                  const busy = actingAlertId === alert.alert_id;
                  const silenceActive = isSilenceActive(alert.silenced_until);
                  const canSilence =
                    alert.status === 'firing' && !silenceActive;
                  const canClose = alert.status !== 'closed';
                  const silenceHint = canSilence
                    ? t('alerts.actionHints.silenceReady')
                    : t('alerts.actionHints.silenceUnavailable');
                  const closeHint = canClose
                    ? t('alerts.actionHints.closeReady')
                    : t('alerts.actionHints.closeClosed');

                  return (
                    <TableRow key={alert.alert_id}>
                      <TableCell>
                        {alert.cluster_name || alert.cluster_id || '-'}
                      </TableCell>
                      <TableCell>
                        <Badge variant='outline'>
                          {resolveSourceLabel(alert.source_type)}
                        </Badge>
                      </TableCell>
                      <TableCell>{alert.alert_name || '-'}</TableCell>
                      <TableCell>
                        <Badge variant={resolveSeverityVariant(alert.severity)}>
                          {resolveSeverityLabel(alert.severity)}
                        </Badge>
                      </TableCell>
                      <TableCell>
                        <Badge variant={resolveStatusVariant(alert.status)}>
                          {resolveStatusLabel(alert.status)}
                        </Badge>
                      </TableCell>
                      <TableCell
                        className='max-w-[360px] align-top'
                        title={alert.summary || alert.description || ''}
                      >
                        <div className='line-clamp-2'>
                          {alert.summary || alert.description || '-'}
                        </div>
                        {renderMarkers(alert)}
                      </TableCell>
                      <TableCell>{formatDateTime(alert.firing_at)}</TableCell>
                      <TableCell>
                        {formatDateTime(resolveLastChangedAt(alert))}
                      </TableCell>
                      <TableCell>
                        <div className='flex flex-wrap gap-2'>
                          <span title={silenceHint}>
                            <Button
                              size='sm'
                              variant='outline'
                              disabled={!canSilence || busy}
                              onClick={() => handleSilence(alert)}
                            >
                              {t('alerts.silence30m')}
                            </Button>
                          </span>
                          <span title={closeHint}>
                            <Button
                              size='sm'
                              variant='secondary'
                              disabled={!canClose || busy}
                              onClick={() => handleClose(alert)}
                            >
                              {t('alerts.manualClose')}
                            </Button>
                          </span>
                        </div>
                      </TableCell>
                    </TableRow>
                  );
                })
              )}
            </TableBody>
          </Table>

          <div className='mt-4 flex flex-col gap-2 border-t pt-4 md:flex-row md:items-center md:justify-between'>
            <div className='flex items-center gap-2'>
              <span className='text-sm text-muted-foreground'>
                {t('alerts.pageSize')}
              </span>
              <Select
                value={pageSize}
                onValueChange={(value) => {
                  setPageSize(value);
                  setPage(1);
                }}
              >
                <SelectTrigger className='w-24'>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value='20'>20</SelectItem>
                  <SelectItem value='50'>50</SelectItem>
                  <SelectItem value='100'>100</SelectItem>
                  <SelectItem value='200'>200</SelectItem>
                </SelectContent>
              </Select>
            </div>

            <div className='flex items-center gap-2'>
              <Button
                variant='outline'
                onClick={() => setPage((prev) => Math.max(1, prev - 1))}
                disabled={loading || page <= 1}
              >
                {t('alerts.prevPage')}
              </Button>
              <span className='text-sm text-muted-foreground'>
                {t('alerts.pageInfo', {current: page, total: totalPages})}
              </span>
              <Button
                variant='outline'
                onClick={() =>
                  setPage((prev) => Math.min(totalPages, prev + 1))
                }
                disabled={loading || page >= totalPages}
              >
                {t('alerts.nextPage')}
              </Button>
            </div>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
