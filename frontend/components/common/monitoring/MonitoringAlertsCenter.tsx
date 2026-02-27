'use client';

import {useCallback, useEffect, useMemo, useState} from 'react';
import {useTranslations} from 'next-intl';
import {toast} from 'sonner';
import {RefreshCw} from 'lucide-react';
import services from '@/lib/services';
import type {RemoteAlertEvent} from '@/lib/services/monitoring';
import {Button} from '@/components/ui/button';
import {Badge} from '@/components/ui/badge';
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

function formatUnixSeconds(value?: number): string {
  if (!value || value <= 0) {
    return '-';
  }
  return formatDateTime(new Date(value * 1000).toISOString());
}

function normalizeStatus(value: string): string {
  return value.trim().toLowerCase();
}

function resolveBadgeVariant(
  status: string,
): 'default' | 'secondary' | 'outline' | 'destructive' {
  const normalized = normalizeStatus(status);
  if (normalized === 'firing') {
    return 'destructive';
  }
  if (normalized === 'resolved') {
    return 'secondary';
  }
  return 'outline';
}

export function MonitoringAlertsCenter() {
  const t = useTranslations('monitoringCenter');

  const [clusterOptions, setClusterOptions] = useState<ClusterOption[]>([]);
  const [clusterFilter, setClusterFilter] = useState<string>('all');
  const [statusFilter, setStatusFilter] = useState<string>('all');

  const [alerts, setAlerts] = useState<RemoteAlertEvent[]>([]);
  const [total, setTotal] = useState<number>(0);
  const [loading, setLoading] = useState<boolean>(true);

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
      const result = await services.monitoring.getRemoteAlertsSafe({
        cluster_id: clusterFilter === 'all' ? undefined : clusterFilter,
        status: statusFilter === 'all' ? undefined : statusFilter,
        page: 1,
        page_size: 200,
      });

      if (!result.success || !result.data) {
        toast.error(result.error || t('alerts.loadError'));
        setAlerts([]);
        setTotal(0);
        return;
      }

      setAlerts(result.data.alerts || []);
      setTotal(result.data.total || 0);
    } finally {
      setLoading(false);
    }
  }, [clusterFilter, statusFilter, t]);

  useEffect(() => {
    loadClusters();
  }, [loadClusters]);

  useEffect(() => {
    loadAlerts();
  }, [loadAlerts]);

  const alertStats = useMemo(
    () =>
      alerts.reduce(
        (acc, alert) => {
          const status = normalizeStatus(alert.status || '');
          if (status === 'firing') {
            acc.firing += 1;
            return acc;
          }
          if (status === 'resolved') {
            acc.resolved += 1;
            return acc;
          }
          acc.others += 1;
          return acc;
        },
        {
          firing: 0,
          resolved: 0,
          others: 0,
        },
      ),
    [alerts],
  );

  const resolveStatusLabel = useCallback(
    (status: string) => {
      const normalized = normalizeStatus(status);
      if (normalized === 'firing') {
        return t('alerts.statusFiring');
      }
      if (normalized === 'resolved') {
        return t('alerts.statusResolved');
      }
      return status || '-';
    },
    [t],
  );

  return (
    <div className='space-y-4'>
      <Card>
        <CardHeader>
          <CardTitle>{t('alerts.title')}</CardTitle>
          <div className='flex flex-col gap-2 lg:flex-row lg:items-center lg:justify-between'>
            <div className='flex flex-col gap-2 md:flex-row'>
              <div className='w-full md:w-56'>
                <Select value={clusterFilter} onValueChange={setClusterFilter}>
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
                <Select value={statusFilter} onValueChange={setStatusFilter}>
                  <SelectTrigger>
                    <SelectValue placeholder={t('alerts.statusFilter')} />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value='all'>{t('alerts.allStatus')}</SelectItem>
                    <SelectItem value='firing'>
                      {t('alerts.statusFiring')}
                    </SelectItem>
                    <SelectItem value='resolved'>
                      {t('alerts.statusResolved')}
                    </SelectItem>
                  </SelectContent>
                </Select>
              </div>
            </div>

            <div className='flex items-center gap-2'>
              <Badge variant='outline'>{`${t('alerts.totalCount')}: ${total}`}</Badge>
              <Badge variant='destructive'>{`${t('alerts.firingCount')}: ${alertStats.firing}`}</Badge>
              <Badge variant='secondary'>{`${t('alerts.resolvedCount')}: ${alertStats.resolved}`}</Badge>
              {alertStats.others > 0 ? (
                <Badge variant='outline'>{`${t('alerts.otherCount')}: ${alertStats.others}`}</Badge>
              ) : null}
              <Button variant='outline' onClick={loadAlerts}>
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
                <TableHead>{t('alerts.alertName')}</TableHead>
                <TableHead>{t('alerts.severity')}</TableHead>
                <TableHead>{t('alerts.status')}</TableHead>
                <TableHead>{t('alerts.summary')}</TableHead>
                <TableHead>{t('alerts.eventTime')}</TableHead>
                <TableHead>{t('alerts.lastReceivedAt')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {loading ? (
                <TableRow>
                  <TableCell
                    colSpan={7}
                    className='text-center text-muted-foreground'
                  >
                    {t('loading')}
                  </TableCell>
                </TableRow>
              ) : !alerts.length ? (
                <TableRow>
                  <TableCell
                    colSpan={7}
                    className='text-center text-muted-foreground'
                  >
                    {t('alerts.noAlerts')}
                  </TableCell>
                </TableRow>
              ) : (
                alerts.map((alert) => (
                  <TableRow key={`${alert.id}-${alert.fingerprint}`}>
                    <TableCell>
                      {alert.cluster_name || alert.cluster_id || '-'}
                    </TableCell>
                    <TableCell>{alert.alert_name || '-'}</TableCell>
                    <TableCell>{alert.severity || '-'}</TableCell>
                    <TableCell>
                      <Badge variant={resolveBadgeVariant(alert.status)}>
                        {resolveStatusLabel(alert.status || '')}
                      </Badge>
                    </TableCell>
                    <TableCell
                      className='max-w-[360px] truncate'
                      title={alert.summary || alert.description || ''}
                    >
                      {alert.summary || alert.description || '-'}
                    </TableCell>
                    <TableCell>{formatUnixSeconds(alert.starts_at)}</TableCell>
                    <TableCell>
                      {formatDateTime(alert.last_received_at)}
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </CardContent>
      </Card>
    </div>
  );
}
