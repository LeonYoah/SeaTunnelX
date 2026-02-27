'use client';

import {useCallback, useEffect, useMemo, useRef, useState} from 'react';
import Link from 'next/link';
import {useTranslations} from 'next-intl';
import {useTheme} from 'next-themes';
import {ExternalLink, Loader2, RefreshCw} from 'lucide-react';
import {toast} from 'sonner';
import services from '@/lib/services';
import type {
  ClusterHealthItem,
  PlatformHealthData,
} from '@/lib/services/monitoring';
import {useLocale} from '@/lib/i18n';
import {Button} from '@/components/ui/button';
import {Badge} from '@/components/ui/badge';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import {Card, CardContent, CardHeader, CardTitle} from '@/components/ui/card';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';

type TimeRange = 'now-1h' | 'now-6h' | 'now-24h' | 'now-7d';
type RefreshInterval = 'off' | '15s' | '30s' | '1m';
type GrafanaTheme = 'light' | 'dark';
const DEFAULT_LOAD_TIMEOUT_MS = 60000;

function resolveDashboardUID(locale: string): string {
  return locale === 'zh' ? 'seatunnel-overview-zh' : 'seatunnel-overview-en';
}

function resolveDashboardSlug(locale: string): string {
  return locale === 'zh'
    ? 'seatunnelx-shen-du-jian-kong'
    : 'seatunnelx-deep-monitoring';
}

function buildGrafanaProxyDashboardURL(
  locale: string,
  timeFrom: TimeRange,
  refresh: RefreshInterval,
  grafanaTheme: GrafanaTheme,
): string {
  const uid = resolveDashboardUID(locale);
  const slug = resolveDashboardSlug(locale);
  const path = `/api/v1/monitoring/proxy/grafana/d/${uid}/${slug}`;

  const search = new URLSearchParams({
    orgId: '1',
    from: timeFrom,
    to: 'now',
  });
  if (refresh !== 'off') {
    search.set('refresh', refresh);
  }
  search.set('theme', grafanaTheme);

  // Use full kiosk mode to hide Grafana side/top navigation as much as possible.
  // 使用完整 kiosk 模式，尽量隐藏 Grafana 左侧/顶部导航。
  return `${path}?${search.toString()}&kiosk`;
}

function resolveHealthBadgeVariant(
  status: string,
): 'default' | 'secondary' | 'outline' | 'destructive' {
  const normalized = status.trim().toLowerCase();
  if (normalized === 'unhealthy') {
    return 'destructive';
  }
  if (normalized === 'degraded') {
    return 'secondary';
  }
  if (normalized === 'healthy') {
    return 'default';
  }
  return 'outline';
}

export function MonitoringOverview() {
  const t = useTranslations('monitoringCenter');
  const {locale} = useLocale();
  const {resolvedTheme} = useTheme();

  const [timeRange, setTimeRange] = useState<TimeRange>('now-6h');
  const [refreshInterval, setRefreshInterval] =
    useState<RefreshInterval>('15s');
  const [iframeKey, setIframeKey] = useState(1);
  const [loaded, setLoaded] = useState(false);
  const [loadFailed, setLoadFailed] = useState(false);
  const [loadTimeoutMs, setLoadTimeoutMs] = useState(DEFAULT_LOAD_TIMEOUT_MS);
  const [iframeHeight, setIframeHeight] = useState(900);
  const [platformHealth, setPlatformHealth] =
    useState<PlatformHealthData | null>(null);
  const [clusterHealth, setClusterHealth] = useState<ClusterHealthItem[]>([]);
  const [healthLoading, setHealthLoading] = useState<boolean>(true);
  const frameContainerRef = useRef<HTMLDivElement | null>(null);
  const timeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const grafanaTheme: GrafanaTheme =
    resolvedTheme === 'light' ? 'light' : 'dark';

  const clearLoadTimeout = () => {
    if (timeoutRef.current) {
      clearTimeout(timeoutRef.current);
      timeoutRef.current = null;
    }
  };

  const loadHealth = useCallback(async () => {
    setHealthLoading(true);
    try {
      const [platformResult, clustersResult] = await Promise.all([
        services.monitoring.getPlatformHealthSafe(),
        services.monitoring.getClustersHealthSafe(),
      ]);

      if (platformResult.success && platformResult.data) {
        setPlatformHealth(platformResult.data);
      } else {
        setPlatformHealth(null);
      }

      if (clustersResult.success && clustersResult.data) {
        setClusterHealth(clustersResult.data.clusters || []);
      } else {
        setClusterHealth([]);
      }

      if (!platformResult.success && !clustersResult.success) {
        toast.error(
          platformResult.error ||
            clustersResult.error ||
            t('platformHealth.loadError'),
        );
      }
    } finally {
      setHealthLoading(false);
    }
  }, [t]);

  const dashboardURL = useMemo(
    () =>
      buildGrafanaProxyDashboardURL(
        locale,
        timeRange,
        refreshInterval,
        grafanaTheme,
      ),
    [locale, timeRange, refreshInterval, grafanaTheme],
  );
  const embedURL = useMemo(
    () =>
      buildGrafanaProxyDashboardURL(
        locale,
        timeRange,
        refreshInterval,
        grafanaTheme,
      ),
    [locale, timeRange, refreshInterval, grafanaTheme],
  );

  const resolveHealthLabel = useCallback(
    (status: string) => {
      const normalized = status.trim().toLowerCase();
      if (normalized === 'healthy') {
        return t('healthStatuses.healthy');
      }
      if (normalized === 'degraded') {
        return t('healthStatuses.degraded');
      }
      if (normalized === 'unhealthy') {
        return t('healthStatuses.unhealthy');
      }
      return t('healthStatuses.unknown');
    },
    [t],
  );

  useEffect(() => {
    loadHealth();
  }, [loadHealth]);

  useEffect(() => {
    if (typeof navigator === 'undefined') {
      setLoadTimeoutMs(DEFAULT_LOAD_TIMEOUT_MS);
      return;
    }

    const connection = (
      navigator as Navigator & {
        connection?: {effectiveType?: string; saveData?: boolean};
      }
    ).connection;
    const effectiveType = connection?.effectiveType || '';
    const saveData = connection?.saveData === true;
    const slowNetwork =
      saveData || effectiveType.includes('2g') || effectiveType.includes('3g');
    setLoadTimeoutMs(slowNetwork ? 90000 : DEFAULT_LOAD_TIMEOUT_MS);
  }, []);

  useEffect(() => {
    setLoaded(false);
    setLoadFailed(false);
    clearLoadTimeout();
    timeoutRef.current = setTimeout(() => {
      setLoadFailed(true);
    }, loadTimeoutMs);
    return () => clearLoadTimeout();
  }, [embedURL, iframeKey, loadTimeoutMs]);

  useEffect(() => {
    if (typeof window === 'undefined') {
      return;
    }

    let rafId = 0;
    const recalcHeight = () => {
      if (rafId) {
        cancelAnimationFrame(rafId);
      }
      rafId = requestAnimationFrame(() => {
        const el = frameContainerRef.current;
        if (!el) {
          return;
        }
        const rect = el.getBoundingClientRect();
        if (!Number.isFinite(rect.top) || rect.top <= 0) {
          return;
        }

        // Keep one smooth scroll context by making iframe fill the remaining viewport.
        // 让 iframe 填充剩余视口高度，避免外层与内层双重滚动错位。
        const next = Math.max(
          720,
          Math.floor(window.innerHeight - rect.top - 12),
        );
        setIframeHeight((prev) => (Math.abs(prev - next) > 2 ? next : prev));
      });
    };

    recalcHeight();
    window.addEventListener('resize', recalcHeight);
    window.addEventListener('scroll', recalcHeight, {passive: true});

    let observer: ResizeObserver | null = null;
    if (typeof ResizeObserver !== 'undefined') {
      observer = new ResizeObserver(recalcHeight);
      if (frameContainerRef.current) {
        observer.observe(frameContainerRef.current);
      }
    }

    return () => {
      if (rafId) {
        cancelAnimationFrame(rafId);
      }
      window.removeEventListener('resize', recalcHeight);
      window.removeEventListener('scroll', recalcHeight);
      observer?.disconnect();
    };
  }, [locale]);

  return (
    <div className='space-y-4'>
      <Card>
        <CardHeader className='space-y-2'>
          <div className='flex items-center justify-between'>
            <CardTitle>{t('platformHealth.title')}</CardTitle>
            <Button variant='outline' onClick={loadHealth}>
              <RefreshCw className='mr-2 h-4 w-4' />
              {t('refresh')}
            </Button>
          </div>
        </CardHeader>
        <CardContent className='space-y-4'>
          {healthLoading ? (
            <div className='text-sm text-muted-foreground'>{t('loading')}</div>
          ) : platformHealth ? (
            <>
              <div className='grid grid-cols-2 gap-3 md:grid-cols-4 lg:grid-cols-8'>
                <div className='rounded-md border p-3'>
                  <div className='text-xs text-muted-foreground'>
                    {t('totalClusters')}
                  </div>
                  <div className='mt-1 text-xl font-semibold'>
                    {platformHealth.total_clusters}
                  </div>
                </div>
                <div className='rounded-md border p-3'>
                  <div className='text-xs text-muted-foreground'>
                    {t('healthyClusters')}
                  </div>
                  <div className='mt-1 text-xl font-semibold'>
                    {platformHealth.healthy_clusters}
                  </div>
                </div>
                <div className='rounded-md border p-3'>
                  <div className='text-xs text-muted-foreground'>
                    {t('degradedClusters')}
                  </div>
                  <div className='mt-1 text-xl font-semibold'>
                    {platformHealth.degraded_clusters}
                  </div>
                </div>
                <div className='rounded-md border p-3'>
                  <div className='text-xs text-muted-foreground'>
                    {t('unhealthyClusters')}
                  </div>
                  <div className='mt-1 text-xl font-semibold'>
                    {platformHealth.unhealthy_clusters}
                  </div>
                </div>
                <div className='rounded-md border p-3'>
                  <div className='text-xs text-muted-foreground'>
                    {t('activeAlerts')}
                  </div>
                  <div className='mt-1 text-xl font-semibold'>
                    {platformHealth.active_alerts}
                  </div>
                </div>
                <div className='rounded-md border p-3'>
                  <div className='text-xs text-muted-foreground'>
                    {t('criticalAlerts')}
                  </div>
                  <div className='mt-1 text-xl font-semibold'>
                    {platformHealth.critical_alerts}
                  </div>
                </div>
                <div className='col-span-2 rounded-md border p-3'>
                  <div className='text-xs text-muted-foreground'>
                    {t('healthStatus')}
                  </div>
                  <div className='mt-1'>
                    <Badge
                      variant={resolveHealthBadgeVariant(
                        platformHealth.health_status,
                      )}
                    >
                      {resolveHealthLabel(platformHealth.health_status)}
                    </Badge>
                  </div>
                </div>
              </div>

              <div className='overflow-x-auto'>
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>{t('clusterName')}</TableHead>
                      <TableHead>{t('healthStatus')}</TableHead>
                      <TableHead>{t('nodes')}</TableHead>
                      <TableHead>{t('activeAlerts')}</TableHead>
                      <TableHead>{t('criticalAlerts')}</TableHead>
                      <TableHead>{t('actions')}</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {!clusterHealth.length ? (
                      <TableRow>
                        <TableCell
                          colSpan={6}
                          className='text-center text-muted-foreground'
                        >
                          {t('noClusters')}
                        </TableCell>
                      </TableRow>
                    ) : (
                      clusterHealth.map((cluster) => (
                        <TableRow key={cluster.cluster_id}>
                          <TableCell>{cluster.cluster_name}</TableCell>
                          <TableCell>
                            <Badge
                              variant={resolveHealthBadgeVariant(
                                cluster.health_status,
                              )}
                            >
                              {resolveHealthLabel(cluster.health_status)}
                            </Badge>
                          </TableCell>
                          <TableCell>{`${cluster.online_nodes}/${cluster.total_nodes}`}</TableCell>
                          <TableCell>{cluster.active_alerts}</TableCell>
                          <TableCell>{cluster.critical_alerts}</TableCell>
                          <TableCell>
                            <div className='flex items-center gap-2'>
                              <Button asChild size='sm' variant='outline'>
                                <Link
                                  href={`/monitoring/clusters/${cluster.cluster_id}`}
                                >
                                  {t('viewDetails')}
                                </Link>
                              </Button>
                              <Button asChild size='sm' variant='outline'>
                                <Link
                                  href={`/monitoring?tab=alerts&cluster_id=${cluster.cluster_id}`}
                                >
                                  {t('tabs.alerts')}
                                </Link>
                              </Button>
                            </div>
                          </TableCell>
                        </TableRow>
                      ))
                    )}
                  </TableBody>
                </Table>
              </div>
            </>
          ) : (
            <div className='text-sm text-muted-foreground'>
              {t('platformHealth.unavailable')}
            </div>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader className='space-y-3'>
          <div className='flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between'>
            <CardTitle>{t('grafana.title')}</CardTitle>
            <div className='flex flex-wrap items-center gap-2'>
              <Button
                variant='outline'
                onClick={() => setIframeKey((value) => value + 1)}
                className='shrink-0'
              >
                <RefreshCw className='mr-2 h-4 w-4' />
                {t('grafana.reload')}
              </Button>
              <Button asChild variant='outline' className='shrink-0'>
                <a href={dashboardURL} target='_blank' rel='noreferrer'>
                  <ExternalLink className='mr-2 h-4 w-4' />
                  {t('grafana.open')}
                </a>
              </Button>
            </div>
          </div>

          <div className='flex flex-col gap-2 md:flex-row md:items-center'>
            <div className='w-full md:w-56'>
              <Select
                value={timeRange}
                onValueChange={(value) => setTimeRange(value as TimeRange)}
              >
                <SelectTrigger>
                  <SelectValue placeholder={t('grafana.timeRange.label')} />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value='now-1h'>
                    {t('grafana.timeRange.last1h')}
                  </SelectItem>
                  <SelectItem value='now-6h'>
                    {t('grafana.timeRange.last6h')}
                  </SelectItem>
                  <SelectItem value='now-24h'>
                    {t('grafana.timeRange.last24h')}
                  </SelectItem>
                  <SelectItem value='now-7d'>
                    {t('grafana.timeRange.last7d')}
                  </SelectItem>
                </SelectContent>
              </Select>
            </div>

            <div className='w-full md:w-56'>
              <Select
                value={refreshInterval}
                onValueChange={(value) =>
                  setRefreshInterval(value as RefreshInterval)
                }
              >
                <SelectTrigger>
                  <SelectValue
                    placeholder={t('grafana.refreshInterval.label')}
                  />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value='off'>
                    {t('grafana.refreshInterval.off')}
                  </SelectItem>
                  <SelectItem value='15s'>15s</SelectItem>
                  <SelectItem value='30s'>30s</SelectItem>
                  <SelectItem value='1m'>1m</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>
        </CardHeader>

        <CardContent className='p-0'>
          <div
            ref={frameContainerRef}
            className='relative overflow-hidden border-t bg-muted/20'
            style={{height: `${iframeHeight}px`}}
          >
            {!loaded && !loadFailed ? (
              <div className='absolute inset-0 z-10 flex items-center justify-center bg-background/70 backdrop-blur-sm'>
                <div className='flex items-center gap-2 text-sm text-muted-foreground'>
                  <Loader2 className='h-4 w-4 animate-spin' />
                  {t('grafana.loading')}
                </div>
              </div>
            ) : null}

            {loadFailed ? (
              <div className='absolute inset-0 z-20 flex flex-col items-center justify-center gap-3 bg-background/80 px-6 text-center backdrop-blur-sm'>
                <div className='text-sm text-muted-foreground'>
                  {t('grafana.loadFailed')}
                </div>
                <Button
                  variant='outline'
                  onClick={() => setIframeKey((value) => value + 1)}
                >
                  {t('grafana.retry')}
                </Button>
              </div>
            ) : null}

            <iframe
              key={iframeKey}
              title='Seatunnel Grafana Dashboard'
              src={embedURL}
              className='h-full w-full rounded-b-xl border-0'
              sandbox='allow-scripts allow-same-origin allow-forms allow-popups allow-downloads'
              referrerPolicy='strict-origin-when-cross-origin'
              loading='eager'
              onLoad={() => {
                clearLoadTimeout();
                setLoaded(true);
                setLoadFailed(false);
              }}
              onError={() => {
                clearLoadTimeout();
                setLoadFailed(true);
              }}
            />

            <div className='pointer-events-none absolute left-0 right-0 top-0 h-12 bg-background/95' />
            <div className='pointer-events-none absolute bottom-0 left-0 right-0 h-2 bg-background/95' />
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
