'use client';

import {useEffect, useState} from 'react';
import {useTranslations} from 'next-intl';
import {motion} from 'motion/react';
import {Card, CardContent, CardHeader, CardTitle} from '@/components/ui/card';
import {
  Server,
  Database,
  Activity,
  CheckCircle,
  AlertCircle,
  Clock,
  Ship,
  Layers,
  RefreshCw,
} from 'lucide-react';
import {OverviewService, OverviewData} from '@/lib/services/dashboard';
import {Button} from '@/components/ui/button';
import Link from 'next/link';

export default function DashboardPage() {
  const t = useTranslations('dashboard');
  const [data, setData] = useState<OverviewData | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const fetchData = async () => {
    setLoading(true);
    const result = await OverviewService.getOverviewDataSafe();
    if (result.success && result.data) {
      setData(result.data);
      setError(null);
    } else {
      setError(result.error || 'Failed to fetch data');
    }
    setLoading(false);
  };

  useEffect(() => {
    fetchData();
    const interval = setInterval(fetchData, 30000);
    return () => clearInterval(interval);
  }, []);

  const stats = data?.stats;

  const statCards = [
    {
      title: t('totalHosts'),
      value: stats?.total_hosts ?? 0,
      subValue: `${stats?.online_hosts ?? 0} ${t('online')}`,
      icon: Server,
      color: 'text-primary',
      bgColor: 'bg-primary/10',
      href: '/hosts',
    },
    {
      title: t('totalClusters'),
      value: stats?.total_clusters ?? 0,
      subValue: `${stats?.running_clusters ?? 0} ${t('running')}`,
      icon: Database,
      color: 'text-primary',
      bgColor: 'bg-primary/10',
      href: '/clusters',
    },
    {
      title: t('totalNodes'),
      value: stats?.total_nodes ?? 0,
      subValue: `${stats?.running_nodes ?? 0} ${t('running')}`,
      icon: Layers,
      color: 'text-primary',
      bgColor: 'bg-primary/10',
      href: '/clusters',
    },
    {
      title: t('onlineAgents'),
      value: `${stats?.online_agents ?? 0}/${stats?.total_agents ?? 0}`,
      subValue: stats?.total_agents
        ? t('onlineRate', {rate: Math.round((stats.online_agents / stats.total_agents) * 100)})
        : t('noAgent'),
      icon: Activity,
      color: 'text-primary',
      bgColor: 'bg-primary/10',
      href: '/hosts',
    },
  ];

  return (
    <div className='space-y-6'>
      <motion.div
        initial={{opacity: 0, y: -20}}
        animate={{opacity: 1, y: 0}}
        transition={{duration: 0.5}}
        className='flex items-center justify-between'
      >
        <div className='flex items-center gap-3'>
          <Ship className='w-8 h-8 text-primary' />
          <div>
            <h1 className='text-2xl font-bold'>{t('title')}</h1>
            <p className='text-muted-foreground text-sm'>{t('subtitle')}</p>
          </div>
        </div>
        <Button variant='outline' size='sm' onClick={fetchData} disabled={loading}>
          <RefreshCw className={`w-4 h-4 mr-2 ${loading ? 'animate-spin' : ''}`} />
          {t('refresh')}
        </Button>
      </motion.div>

      {error && (
        <div className='p-4 bg-red-500/10 border border-red-500/20 rounded-lg text-red-500'>
          {error}
        </div>
      )}

      <div className='grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4'>
        {statCards.map((stat, index) => (
          <motion.div
            key={stat.title}
            initial={{opacity: 0, y: 20}}
            animate={{opacity: 1, y: 0}}
            transition={{delay: index * 0.1, duration: 0.5}}
          >
            <Link href={stat.href}>
              <Card className='hover:shadow-md transition-shadow cursor-pointer'>
                <CardHeader className='flex flex-row items-center justify-between pb-2'>
                  <CardTitle className='text-sm font-medium text-muted-foreground'>
                    {stat.title}
                  </CardTitle>
                  <div className={`p-2 rounded-lg ${stat.bgColor}`}>
                    <stat.icon className={`w-4 h-4 ${stat.color}`} />
                  </div>
                </CardHeader>
                <CardContent>
                  <div className='text-2xl font-bold'>{stat.value}</div>
                  <p className='text-xs text-muted-foreground mt-1'>{stat.subValue}</p>
                </CardContent>
              </Card>
            </Link>
          </motion.div>
        ))}
      </div>


      <div className='grid grid-cols-1 lg:grid-cols-2 gap-6'>
        <motion.div
          initial={{opacity: 0, x: -20}}
          animate={{opacity: 1, x: 0}}
          transition={{delay: 0.4, duration: 0.5}}
        >
          <Card>
            <CardHeader>
              <CardTitle className='flex items-center gap-2'>
                <Database className='w-5 h-5 text-primary' />
                {t('clusterStatus')}
              </CardTitle>
            </CardHeader>
            <CardContent>
              <div className='space-y-4'>
                {data?.cluster_summaries && data.cluster_summaries.length > 0 ? (
                  data.cluster_summaries.map((cluster) => (
                    <Link key={cluster.id} href={`/clusters/${cluster.id}`}>
                      <div className='p-4 rounded-lg bg-muted/50 space-y-3 hover:bg-muted transition-colors cursor-pointer'>
                        <div className='flex items-center justify-between'>
                          <div className='flex items-center gap-2'>
                            <span
                              className={`w-2 h-2 rounded-full ${
                                cluster.status === 'running'
                                  ? 'bg-green-500'
                                  : cluster.status === 'error'
                                  ? 'bg-red-500'
                                  : 'bg-gray-400'
                              }`}
                            />
                            <span className='font-medium'>{cluster.name}</span>
                          </div>
                          <span className='text-sm text-muted-foreground'>
                            {cluster.total_nodes} {t('nodes')}
                          </span>
                        </div>
                        <div className='grid grid-cols-3 gap-4 text-sm text-muted-foreground'>
                          <div>{t('master')}: {cluster.master_nodes}</div>
                          <div>{t('worker')}: {cluster.worker_nodes}</div>
                          <div>{t('runningNodes')}: {cluster.running_nodes}</div>
                        </div>
                      </div>
                    </Link>
                  ))
                ) : (
                  <div className='text-center py-8 text-muted-foreground'>
                    {t('noClusterData')}
                  </div>
                )}
              </div>
            </CardContent>
          </Card>
        </motion.div>

        <motion.div
          initial={{opacity: 0, x: 20}}
          animate={{opacity: 1, x: 0}}
          transition={{delay: 0.4, duration: 0.5}}
        >
          <Card>
            <CardHeader>
              <CardTitle className='flex items-center gap-2'>
                <Server className='w-5 h-5 text-primary' />
                {t('hostStatus')}
              </CardTitle>
            </CardHeader>
            <CardContent>
              <div className='space-y-4'>
                {data?.host_summaries && data.host_summaries.length > 0 ? (
                  data.host_summaries.map((host) => (
                    <Link key={host.id} href={`/hosts/${host.id}`}>
                      <div className='p-4 rounded-lg bg-muted/50 hover:bg-muted transition-colors cursor-pointer'>
                        <div className='flex items-center justify-between'>
                          <div className='flex items-center gap-2'>
                            <span
                              className={`w-2 h-2 rounded-full ${
                                host.is_online ? 'bg-green-500' : 'bg-gray-400'
                              }`}
                            />
                            <span className='font-medium'>{host.name}</span>
                            <span className='text-sm text-muted-foreground'>
                              ({host.ip_address})
                            </span>
                          </div>
                          <span className='text-sm text-muted-foreground'>
                            {host.node_count} {t('nodes')}
                          </span>
                        </div>
                        <div className='mt-2 text-sm text-muted-foreground'>
                          {t('agent')}: {host.agent_status !== 'installed' && host.agent_status !== 'offline'
                            ? t('notInstalled')
                            : host.is_online
                              ? t('online')
                              : t('offline')}
                        </div>
                      </div>
                    </Link>
                  ))
                ) : (
                  <div className='text-center py-8 text-muted-foreground'>
                    {t('noHostData')}
                  </div>
                )}
              </div>
            </CardContent>
          </Card>
        </motion.div>
      </div>


      <motion.div
        initial={{opacity: 0, y: 20}}
        animate={{opacity: 1, y: 0}}
        transition={{delay: 0.5, duration: 0.5}}
      >
        <Card>
          <CardHeader>
            <CardTitle className='flex items-center gap-2'>
              <Activity className='w-5 h-5 text-primary' />
              {t('recentActivities')}
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className='space-y-4'>
              {data?.recent_activities && data.recent_activities.length > 0 ? (
                data.recent_activities.map((activity) => (
                  <div
                    key={activity.id}
                    className='flex items-start gap-3 pb-3 border-b last:border-0 last:pb-0'
                  >
                    <div className='mt-0.5'>
                      {activity.type === 'success' && (
                        <CheckCircle className='w-4 h-4 text-green-500' />
                      )}
                      {activity.type === 'warning' && (
                        <AlertCircle className='w-4 h-4 text-yellow-500' />
                      )}
                      {activity.type === 'error' && (
                        <AlertCircle className='w-4 h-4 text-red-500' />
                      )}
                      {activity.type === 'info' && (
                        <Activity className='w-4 h-4 text-primary' />
                      )}
                    </div>
                    <div className='flex-1 min-w-0'>
                      <p className='text-sm'>{activity.message}</p>
                      <p className='text-xs text-muted-foreground flex items-center gap-1 mt-1'>
                        <Clock className='w-3 h-3' />
                        {activity.timestamp}
                      </p>
                    </div>
                  </div>
                ))
              ) : (
                <div className='text-center py-8 text-muted-foreground'>
                  {t('noActivityData')}
                </div>
              )}
            </div>
          </CardContent>
        </Card>
      </motion.div>

      <motion.div
        initial={{opacity: 0, y: 20}}
        animate={{opacity: 1, y: 0}}
        transition={{delay: 0.6, duration: 0.5}}
      >
        <Card className='bg-primary/5 border-primary/20'>
          <CardContent className='py-4'>
            <div className='flex items-center justify-between'>
              <div className='flex items-center gap-3'>
                <Ship className='w-6 h-6 text-primary' />
                <div>
                  <p className='font-medium'>{t('welcome')}</p>
                  <p className='text-sm text-muted-foreground'>{t('welcomeDesc')}</p>
                </div>
              </div>
              <div className='flex gap-2'>
                <Link href='/hosts'>
                  <Button variant='outline' size='sm'>{t('addHost')}</Button>
                </Link>
                <Link href='/clusters'>
                  <Button size='sm'>{t('createCluster')}</Button>
                </Link>
              </div>
            </div>
          </CardContent>
        </Card>
      </motion.div>
    </div>
  );
}
