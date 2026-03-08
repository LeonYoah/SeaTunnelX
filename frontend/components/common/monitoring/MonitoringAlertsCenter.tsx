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
import {toast} from 'sonner';
import {RefreshCw} from 'lucide-react';
import services from '@/lib/services';
import type {
  AlertHandlingStatus,
  AlertInstance,
  AlertInstanceStats,
  AlertLifecycleStatus,
  AlertSourceType,
} from '@/lib/services/monitoring';
import {Button} from '@/components/ui/button';
import {Badge} from '@/components/ui/badge';
import {Input} from '@/components/ui/input';
import {Card, CardContent, CardHeader, CardTitle} from '@/components/ui/card';
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
  pending: 0,
  acknowledged: 0,
  silenced: 0,
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

function resolveLifecycleVariant(
  status: AlertLifecycleStatus,
): 'default' | 'secondary' | 'outline' | 'destructive' {
  return status === 'firing' ? 'destructive' : 'secondary';
}

function resolveHandlingVariant(
  status: AlertHandlingStatus,
): 'default' | 'secondary' | 'outline' | 'destructive' {
  if (status === 'silenced') {
    return 'outline';
  }
  if (status === 'acknowledged') {
    return 'secondary';
  }
  return 'default';
}

export function MonitoringAlertsCenter() {
  const t = useTranslations('monitoringCenter');
  const searchParams = useSearchParams();

  const [clusterOptions, setClusterOptions] = useState<ClusterOption[]>([]);
  const [clusterFilter, setClusterFilter] = useState<string>('all');
  const [sourceFilter, setSourceFilter] = useState<string>('all');
  const [lifecycleFilter, setLifecycleFilter] = useState<string>('all');
  const [handlingFilter, setHandlingFilter] = useState<string>('all');
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
        size: 200,
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
        lifecycle_status:
          lifecycleFilter === 'all'
            ? undefined
            : (lifecycleFilter as AlertLifecycleStatus),
        handling_status:
          handlingFilter === 'all'
            ? undefined
            : (handlingFilter as AlertHandlingStatus),
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
      if (result.data.page && result.data.page !== page) {
        setPage(result.data.page);
      }
    } finally {
      setLoading(false);
    }
  }, [
    clusterFilter,
    sourceFilter,
    lifecycleFilter,
    handlingFilter,
    startTimeFilter,
    endTimeFilter,
    page,
    pageSizeNumber,
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

  const resolveLifecycleLabel = useCallback(
    (status: AlertLifecycleStatus) => {
      if (status === 'resolved') {
        return t('alerts.lifecycleStatuses.resolved');
      }
      return t('alerts.lifecycleStatuses.firing');
    },
    [t],
  );

  const resolveHandlingLabel = useCallback(
    (status: AlertHandlingStatus) => {
      if (status === 'acknowledged') {
        return t('alerts.handlingStatuses.acknowledged');
      }
      if (status === 'silenced') {
        return t('alerts.handlingStatuses.silenced');
      }
      return t('alerts.handlingStatuses.pending');
    },
    [t],
  );

  const resolveSeverityLabel = useCallback(
    (severity: string) => {
      const normalized = severity.trim().toLowerCase();
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
  }, [total, pageSizeNumber]);

  const handleAcknowledge = async (alert: AlertInstance) => {
    setActingAlertId(alert.alert_id);
    try {
      const result = await services.monitoring.acknowledgeAlertInstanceSafe(
        alert.alert_id,
      );
      if (!result.success) {
        toast.error(result.error || t('alerts.ackError'));
        return;
      }
      toast.success(t('alerts.ackSuccess'));
      await loadAlerts();
    } finally {
      setActingAlertId(null);
    }
  };

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

  return (
    <div className='space-y-4'>
      <Card>
        <CardHeader>
          <CardTitle>{t('alerts.title')}</CardTitle>
          <div className='flex flex-col gap-2 lg:flex-row lg:items-center lg:justify-between'>
            <div className='flex flex-col gap-2 md:flex-row md:flex-wrap'>
              <div className='w-full md:w-56'>
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
                    <SelectItem value='all'>
                      {t('alerts.allClusters')}
                    </SelectItem>
                    {clusterOptions.map((cluster) => (
                      <SelectItem key={cluster.id} value={cluster.id}>
                        {cluster.name}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>

              <div className='w-full md:w-56'>
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

              <div className='w-full md:w-56'>
                <Select
                  value={lifecycleFilter}
                  onValueChange={(value) => {
                    setLifecycleFilter(value);
                    setPage(1);
                  }}
                >
                  <SelectTrigger>
                    <SelectValue
                      placeholder={t('alerts.lifecycleStatusFilter')}
                    />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value='all'>{t('alerts.allLifecycle')}</SelectItem>
                    <SelectItem value='firing'>
                      {t('alerts.lifecycleStatuses.firing')}
                    </SelectItem>
                    <SelectItem value='resolved'>
                      {t('alerts.lifecycleStatuses.resolved')}
                    </SelectItem>
                  </SelectContent>
                </Select>
              </div>

              <div className='w-full md:w-56'>
                <Select
                  value={handlingFilter}
                  onValueChange={(value) => {
                    setHandlingFilter(value);
                    setPage(1);
                  }}
                >
                  <SelectTrigger>
                    <SelectValue
                      placeholder={t('alerts.handlingStatusFilter')}
                    />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value='all'>{t('alerts.allHandling')}</SelectItem>
                    <SelectItem value='pending'>
                      {t('alerts.handlingStatuses.pending')}
                    </SelectItem>
                    <SelectItem value='acknowledged'>
                      {t('alerts.handlingStatuses.acknowledged')}
                    </SelectItem>
                    <SelectItem value='silenced'>
                      {t('alerts.handlingStatuses.silenced')}
                    </SelectItem>
                  </SelectContent>
                </Select>
              </div>

              <div className='w-full md:w-64'>
                <Input
                  type='datetime-local'
                  value={startTimeFilter}
                  onChange={(event) => {
                    setStartTimeFilter(event.target.value);
                    setPage(1);
                  }}
                  placeholder={t('alerts.startTime')}
                />
              </div>

              <div className='w-full md:w-64'>
                <Input
                  type='datetime-local'
                  value={endTimeFilter}
                  onChange={(event) => {
                    setEndTimeFilter(event.target.value);
                    setPage(1);
                  }}
                  placeholder={t('alerts.endTime')}
                />
              </div>

              <Button
                variant='outline'
                onClick={() => {
                  setStartTimeFilter('');
                  setEndTimeFilter('');
                  setPage(1);
                }}
                disabled={!startTimeFilter && !endTimeFilter}
              >
                {t('alerts.clearTimeFilter')}
              </Button>
            </div>

            <div className='flex flex-wrap items-center gap-2'>
              <Badge variant='outline'>{`${t('alerts.totalCount')}: ${total}`}</Badge>
              <Badge variant='destructive'>{`${t('alerts.firingCount')}: ${stats.firing}`}</Badge>
              <Badge variant='secondary'>{`${t('alerts.resolvedCount')}: ${stats.resolved}`}</Badge>
              <Badge variant='default'>{`${t('alerts.pendingCount')}: ${stats.pending}`}</Badge>
              <Badge variant='secondary'>{`${t('alerts.acknowledgedCount')}: ${stats.acknowledged}`}</Badge>
              <Badge variant='outline'>{`${t('alerts.silencedCount')}: ${stats.silenced}`}</Badge>
              <Button variant='outline' onClick={loadAlerts} disabled={loading}>
                <RefreshCw className='mr-2 h-4 w-4' />
                {t('refresh')}
              </Button>
            </div>
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
                <TableHead>{t('alerts.lifecycleStatus')}</TableHead>
                <TableHead>{t('alerts.handlingStatus')}</TableHead>
                <TableHead>{t('alerts.summary')}</TableHead>
                <TableHead>{t('alerts.eventTime')}</TableHead>
                <TableHead>{t('alerts.lastSeenAt')}</TableHead>
                <TableHead>{t('actions')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {loading ? (
                <TableRow>
                  <TableCell
                    colSpan={10}
                    className='text-center text-muted-foreground'
                  >
                    {t('loading')}
                  </TableCell>
                </TableRow>
              ) : !alerts.length ? (
                <TableRow>
                  <TableCell
                    colSpan={10}
                    className='text-center text-muted-foreground'
                  >
                    {t('alerts.noAlerts')}
                  </TableCell>
                </TableRow>
              ) : (
                alerts.map((alert) => {
                  const busy = actingAlertId === alert.alert_id;
                  const canAcknowledge =
                    alert.lifecycle_status === 'firing' &&
                    alert.handling_status === 'pending';
                  const canSilence =
                    alert.lifecycle_status === 'firing' &&
                    alert.handling_status !== 'silenced';

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
                          {resolveSeverityLabel(String(alert.severity || ''))}
                        </Badge>
                      </TableCell>
                      <TableCell>
                        <Badge
                          variant={resolveLifecycleVariant(
                            alert.lifecycle_status,
                          )}
                        >
                          {resolveLifecycleLabel(alert.lifecycle_status)}
                        </Badge>
                      </TableCell>
                      <TableCell>
                        <Badge
                          variant={resolveHandlingVariant(alert.handling_status)}
                        >
                          {resolveHandlingLabel(alert.handling_status)}
                        </Badge>
                      </TableCell>
                      <TableCell
                        className='max-w-[320px] truncate'
                        title={alert.summary || alert.description || ''}
                      >
                        {alert.summary || alert.description || '-'}
                      </TableCell>
                      <TableCell>{formatDateTime(alert.firing_at)}</TableCell>
                      <TableCell>{formatDateTime(alert.last_seen_at)}</TableCell>
                      <TableCell>
                        <div className='flex flex-wrap gap-2'>
                          <Button
                            size='sm'
                            variant='outline'
                            disabled={!canAcknowledge || busy}
                            onClick={() => handleAcknowledge(alert)}
                          >
                            {t('alerts.ack')}
                          </Button>
                          <Button
                            size='sm'
                            variant='outline'
                            disabled={!canSilence || busy}
                            onClick={() => handleSilence(alert)}
                          >
                            {t('alerts.silence30m')}
                          </Button>
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
