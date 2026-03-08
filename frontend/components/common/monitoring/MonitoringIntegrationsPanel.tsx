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
import {toast} from 'sonner';
import services from '@/lib/services';
import type {
  AlertSeverity,
  AlertSourceType,
  NotificationChannel,
  NotificationChannelType,
  NotificationRoute,
  UpsertNotificationChannelRequest,
  UpsertNotificationRouteRequest,
} from '@/lib/services/monitoring';
import {Card, CardContent, CardHeader, CardTitle} from '@/components/ui/card';
import {Button} from '@/components/ui/button';
import {Input} from '@/components/ui/input';
import {Label} from '@/components/ui/label';
import {Switch} from '@/components/ui/switch';
import {Textarea} from '@/components/ui/textarea';
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

const DEFAULT_CHANNEL_FORM: UpsertNotificationChannelRequest = {
  name: '',
  type: 'webhook',
  endpoint: '',
  secret: '',
  description: '',
  enabled: true,
};

const DEFAULT_ROUTE_FORM = {
  name: '',
  source_type: 'all',
  cluster_id: 'all',
  severity: 'all',
  rule_key: '',
  channel_id: '',
  enabled: true,
  send_resolved: true,
  mute_if_acknowledged: true,
  mute_if_silenced: true,
};

export function MonitoringIntegrationsPanel() {
  const t = useTranslations('monitoringCenter');

  const [channels, setChannels] = useState<NotificationChannel[]>([]);
  const [routes, setRoutes] = useState<NotificationRoute[]>([]);
  const [clusterOptions, setClusterOptions] = useState<ClusterOption[]>([]);

  const [loadingChannels, setLoadingChannels] = useState<boolean>(true);
  const [loadingRoutes, setLoadingRoutes] = useState<boolean>(true);
  const [creatingChannel, setCreatingChannel] = useState<boolean>(false);
  const [creatingRoute, setCreatingRoute] = useState<boolean>(false);
  const [updatingChannelId, setUpdatingChannelId] = useState<number | null>(
    null,
  );
  const [updatingRouteId, setUpdatingRouteId] = useState<number | null>(null);
  const [deletingChannelId, setDeletingChannelId] = useState<number | null>(
    null,
  );
  const [deletingRouteId, setDeletingRouteId] = useState<number | null>(null);
  const [testingChannelId, setTestingChannelId] = useState<number | null>(null);

  const [channelForm, setChannelForm] =
    useState<UpsertNotificationChannelRequest>(DEFAULT_CHANNEL_FORM);
  const [routeForm, setRouteForm] = useState(DEFAULT_ROUTE_FORM);

  const typeLabelMap = useMemo(
    () => ({
      webhook: t('phase3.channelTypes.webhook'),
      email: t('phase3.channelTypes.email'),
      wecom: t('phase3.channelTypes.wecom'),
      dingtalk: t('phase3.channelTypes.dingtalk'),
      feishu: t('phase3.channelTypes.feishu'),
    }),
    [t],
  );

  const sourceTypeLabelMap = useMemo(
    () => ({
      local_process_event: t('alerts.sourceTypes.local_process_event'),
      remote_alertmanager: t('alerts.sourceTypes.remote_alertmanager'),
    }),
    [t],
  );

  const severityLabelMap = useMemo(
    () => ({
      warning: t('alertSeverity.warning'),
      critical: t('alertSeverity.critical'),
    }),
    [t],
  );

  const loadClusters = useCallback(async () => {
    try {
      const data = await services.cluster.getClusters({current: 1, size: 200});
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

  const loadChannels = useCallback(async () => {
    setLoadingChannels(true);
    try {
      const result = await services.monitoring.listNotificationChannelsSafe();
      if (!result.success || !result.data) {
        toast.error(result.error || t('phase3.channelLoadError'));
        setChannels([]);
        return;
      }
      setChannels(result.data.channels || []);
    } finally {
      setLoadingChannels(false);
    }
  }, [t]);

  const loadRoutes = useCallback(async () => {
    setLoadingRoutes(true);
    try {
      const result = await services.monitoring.listNotificationRoutesSafe();
      if (!result.success || !result.data) {
        toast.error(result.error || t('phase3.routeLoadError'));
        setRoutes([]);
        return;
      }
      setRoutes(result.data.routes || []);
    } finally {
      setLoadingRoutes(false);
    }
  }, [t]);

  useEffect(() => {
    loadClusters();
    loadChannels();
    loadRoutes();
  }, [loadChannels, loadClusters, loadRoutes]);

  const handleCreateChannel = async () => {
    if (!channelForm.name.trim() || !channelForm.endpoint.trim()) {
      toast.error(t('phase3.channelFormInvalid'));
      return;
    }

    setCreatingChannel(true);
    try {
      const result = await services.monitoring.createNotificationChannelSafe({
        ...channelForm,
        name: channelForm.name.trim(),
        endpoint: channelForm.endpoint.trim(),
      });
      if (!result.success) {
        toast.error(result.error || t('phase3.channelCreateError'));
        return;
      }
      toast.success(t('phase3.channelCreateSuccess'));
      setChannelForm(DEFAULT_CHANNEL_FORM);
      await loadChannels();
    } finally {
      setCreatingChannel(false);
    }
  };

  const handleDeleteChannel = async (channelId: number) => {
    setDeletingChannelId(channelId);
    try {
      const result =
        await services.monitoring.deleteNotificationChannelSafe(channelId);
      if (!result.success) {
        toast.error(result.error || t('phase3.channelDeleteError'));
        return;
      }
      toast.success(t('phase3.channelDeleteSuccess'));
      await loadChannels();
      await loadRoutes();
    } finally {
      setDeletingChannelId(null);
    }
  };

  const handleToggleChannelEnabled = async (
    channel: NotificationChannel,
    enabled: boolean,
  ) => {
    setUpdatingChannelId(channel.id);
    try {
      const result = await services.monitoring.updateNotificationChannelSafe(
        channel.id,
        {
          name: channel.name,
          type: channel.type,
          endpoint: channel.endpoint,
          secret: channel.secret,
          description: channel.description,
          enabled,
        },
      );
      if (!result.success) {
        toast.error(result.error || t('phase3.channelCreateError'));
        return;
      }
      await loadChannels();
    } finally {
      setUpdatingChannelId(null);
    }
  };

  const handleTestChannel = async (channelId: number) => {
    setTestingChannelId(channelId);
    try {
      const result = await services.monitoring.testNotificationChannelSafe(
        channelId,
      );
      if (!result.success || !result.data) {
        toast.error(result.error || t('phase3.channelTestError'));
        return;
      }
      if (result.data.status === 'sent') {
        toast.success(t('phase3.channelTestSuccess'));
        return;
      }
      toast.error(result.data.last_error || t('phase3.channelTestError'));
    } finally {
      setTestingChannelId(null);
    }
  };

  const handleCreateRoute = async () => {
    if (!routeForm.name.trim() || !routeForm.channel_id) {
      toast.error(t('phase3.routeFormInvalid'));
      return;
    }

    const payload: UpsertNotificationRouteRequest = {
      name: routeForm.name.trim(),
      enabled: routeForm.enabled,
      source_type:
        routeForm.source_type === 'all'
          ? ''
          : (routeForm.source_type as AlertSourceType),
      cluster_id:
        routeForm.cluster_id === 'all' ? '' : routeForm.cluster_id.trim(),
      severity:
        routeForm.severity === 'all'
          ? ''
          : (routeForm.severity as AlertSeverity),
      rule_key: routeForm.rule_key.trim(),
      channel_id: Number(routeForm.channel_id),
      send_resolved: routeForm.send_resolved,
      mute_if_acknowledged: routeForm.mute_if_acknowledged,
      mute_if_silenced: routeForm.mute_if_silenced,
    };

    setCreatingRoute(true);
    try {
      const result =
        await services.monitoring.createNotificationRouteSafe(payload);
      if (!result.success) {
        toast.error(result.error || t('phase3.routeCreateError'));
        return;
      }
      toast.success(t('phase3.routeCreateSuccess'));
      setRouteForm(DEFAULT_ROUTE_FORM);
      await loadRoutes();
    } finally {
      setCreatingRoute(false);
    }
  };

  const handleDeleteRoute = async (routeId: number) => {
    setDeletingRouteId(routeId);
    try {
      const result =
        await services.monitoring.deleteNotificationRouteSafe(routeId);
      if (!result.success) {
        toast.error(result.error || t('phase3.routeDeleteError'));
        return;
      }
      toast.success(t('phase3.routeDeleteSuccess'));
      await loadRoutes();
    } finally {
      setDeletingRouteId(null);
    }
  };

  const handleToggleRouteEnabled = async (
    route: NotificationRoute,
    enabled: boolean,
  ) => {
    setUpdatingRouteId(route.id);
    try {
      const result = await services.monitoring.updateNotificationRouteSafe(
        route.id,
        {
          name: route.name,
          enabled,
          source_type: (route.source_type || '') as AlertSourceType | '',
          cluster_id: route.cluster_id || '',
          severity: (route.severity || '') as AlertSeverity | '',
          rule_key: route.rule_key || '',
          channel_id: route.channel_id,
          send_resolved: route.send_resolved,
          mute_if_acknowledged: route.mute_if_acknowledged,
          mute_if_silenced: route.mute_if_silenced,
        },
      );
      if (!result.success) {
        toast.error(result.error || t('phase3.routeUpdateError'));
        return;
      }
      await loadRoutes();
    } finally {
      setUpdatingRouteId(null);
    }
  };

  const channelNameMap = useMemo(() => {
    return new Map(channels.map((channel) => [channel.id, channel.name]));
  }, [channels]);

  const resolveRouteSummary = (route: NotificationRoute): string => {
    const parts: string[] = [];
    if (route.source_type) {
      parts.push(
        sourceTypeLabelMap[
          route.source_type as keyof typeof sourceTypeLabelMap
        ] || route.source_type,
      );
    }
    if (route.cluster_id) {
      const clusterName =
        clusterOptions.find((cluster) => cluster.id === route.cluster_id)?.name ||
        route.cluster_id;
      parts.push(clusterName);
    }
    if (route.severity) {
      parts.push(
        severityLabelMap[route.severity as keyof typeof severityLabelMap] ||
          route.severity,
      );
    }
    if (route.rule_key) {
      parts.push(route.rule_key);
    }
    return parts.join(' / ') || t('phase3.routeMatchAll');
  };

  return (
    <div className='space-y-4'>
      <Card>
        <CardHeader>
          <CardTitle>{t('phase3.channelCreateTitle')}</CardTitle>
        </CardHeader>
        <CardContent className='space-y-4'>
          <div className='grid grid-cols-1 gap-4 md:grid-cols-2'>
            <div className='space-y-2'>
              <Label>{t('phase3.channelName')}</Label>
              <Input
                value={channelForm.name}
                onChange={(event) =>
                  setChannelForm((prev) => ({...prev, name: event.target.value}))
                }
              />
            </div>
            <div className='space-y-2'>
              <Label>{t('phase3.channelType')}</Label>
              <Select
                value={channelForm.type}
                onValueChange={(value) =>
                  setChannelForm((prev) => ({
                    ...prev,
                    type: value as NotificationChannelType,
                  }))
                }
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value='webhook'>{typeLabelMap.webhook}</SelectItem>
                  <SelectItem value='email'>{typeLabelMap.email}</SelectItem>
                  <SelectItem value='wecom'>{typeLabelMap.wecom}</SelectItem>
                  <SelectItem value='dingtalk'>
                    {typeLabelMap.dingtalk}
                  </SelectItem>
                  <SelectItem value='feishu'>{typeLabelMap.feishu}</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>

          <div className='space-y-2'>
            <Label>{t('phase3.channelEndpoint')}</Label>
            <Input
              value={channelForm.endpoint}
              onChange={(event) =>
                setChannelForm((prev) => ({
                  ...prev,
                  endpoint: event.target.value,
                }))
              }
            />
          </div>

          <div className='space-y-2'>
            <Label>{t('phase3.channelSecret')}</Label>
            <Input
              value={channelForm.secret || ''}
              onChange={(event) =>
                setChannelForm((prev) => ({...prev, secret: event.target.value}))
              }
            />
          </div>

          <div className='space-y-2'>
            <Label>{t('phase3.channelDescription')}</Label>
            <Textarea
              rows={3}
              value={channelForm.description || ''}
              onChange={(event) =>
                setChannelForm((prev) => ({
                  ...prev,
                  description: event.target.value,
                }))
              }
            />
          </div>

          <div className='flex items-center gap-3'>
            <Switch
              checked={channelForm.enabled ?? true}
              onCheckedChange={(checked) =>
                setChannelForm((prev) => ({...prev, enabled: checked}))
              }
            />
            <Label>{t('phase3.channelEnabled')}</Label>
          </div>

          <Button onClick={handleCreateChannel} disabled={creatingChannel}>
            {t('phase3.channelCreate')}
          </Button>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t('phase3.channelListTitle')}</CardTitle>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t('phase3.channelName')}</TableHead>
                <TableHead>{t('phase3.channelType')}</TableHead>
                <TableHead>{t('phase3.channelEndpoint')}</TableHead>
                <TableHead>{t('phase3.channelEnabled')}</TableHead>
                <TableHead>{t('actions')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {loadingChannels ? (
                <TableRow>
                  <TableCell
                    colSpan={5}
                    className='text-center text-muted-foreground'
                  >
                    {t('loading')}
                  </TableCell>
                </TableRow>
              ) : !channels.length ? (
                <TableRow>
                  <TableCell
                    colSpan={5}
                    className='text-center text-muted-foreground'
                  >
                    {t('phase3.channelEmpty')}
                  </TableCell>
                </TableRow>
              ) : (
                channels.map((channel) => (
                  <TableRow key={channel.id}>
                    <TableCell>{channel.name}</TableCell>
                    <TableCell>
                      {typeLabelMap[channel.type as NotificationChannelType] ||
                        channel.type}
                    </TableCell>
                    <TableCell className='max-w-[380px] break-all'>
                      {channel.endpoint}
                    </TableCell>
                    <TableCell>
                      <Switch
                        checked={channel.enabled}
                        disabled={updatingChannelId === channel.id}
                        onCheckedChange={(checked) =>
                          handleToggleChannelEnabled(channel, checked)
                        }
                      />
                    </TableCell>
                    <TableCell>
                      <div className='flex flex-wrap gap-2'>
                        <Button
                          size='sm'
                          variant='outline'
                          disabled={testingChannelId === channel.id}
                          onClick={() => handleTestChannel(channel.id)}
                        >
                          {t('phase3.testSend')}
                        </Button>
                        <Button
                          size='sm'
                          variant='destructive'
                          disabled={deletingChannelId === channel.id}
                          onClick={() => handleDeleteChannel(channel.id)}
                        >
                          {t('phase3.delete')}
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t('phase3.routeCreateTitle')}</CardTitle>
        </CardHeader>
        <CardContent className='space-y-4'>
          <div className='grid grid-cols-1 gap-4 md:grid-cols-2'>
            <div className='space-y-2'>
              <Label>{t('phase3.routeName')}</Label>
              <Input
                value={routeForm.name}
                onChange={(event) =>
                  setRouteForm((prev) => ({...prev, name: event.target.value}))
                }
              />
            </div>

            <div className='space-y-2'>
              <Label>{t('phase3.routeChannel')}</Label>
              <Select
                value={routeForm.channel_id}
                onValueChange={(value) =>
                  setRouteForm((prev) => ({...prev, channel_id: value}))
                }
              >
                <SelectTrigger>
                  <SelectValue placeholder={t('phase3.routeChannelSelect')} />
                </SelectTrigger>
                <SelectContent>
                  {channels.map((channel) => (
                    <SelectItem key={channel.id} value={String(channel.id)}>
                      {channel.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className='space-y-2'>
              <Label>{t('phase3.routeSourceType')}</Label>
              <Select
                value={routeForm.source_type}
                onValueChange={(value) =>
                  setRouteForm((prev) => ({...prev, source_type: value}))
                }
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value='all'>{t('phase3.routeMatchAll')}</SelectItem>
                  <SelectItem value='local_process_event'>
                    {t('alerts.sourceTypes.local_process_event')}
                  </SelectItem>
                  <SelectItem value='remote_alertmanager'>
                    {t('alerts.sourceTypes.remote_alertmanager')}
                  </SelectItem>
                </SelectContent>
              </Select>
            </div>

            <div className='space-y-2'>
              <Label>{t('phase3.routeCluster')}</Label>
              <Select
                value={routeForm.cluster_id}
                onValueChange={(value) =>
                  setRouteForm((prev) => ({...prev, cluster_id: value}))
                }
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value='all'>{t('phase3.routeMatchAll')}</SelectItem>
                  {clusterOptions.map((cluster) => (
                    <SelectItem key={cluster.id} value={cluster.id}>
                      {cluster.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className='space-y-2'>
              <Label>{t('phase3.routeSeverity')}</Label>
              <Select
                value={routeForm.severity}
                onValueChange={(value) =>
                  setRouteForm((prev) => ({...prev, severity: value}))
                }
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value='all'>{t('phase3.routeMatchAll')}</SelectItem>
                  <SelectItem value='warning'>{t('alertSeverity.warning')}</SelectItem>
                  <SelectItem value='critical'>{t('alertSeverity.critical')}</SelectItem>
                </SelectContent>
              </Select>
            </div>

            <div className='space-y-2'>
              <Label>{t('phase3.routeRuleKey')}</Label>
              <Input
                value={routeForm.rule_key}
                placeholder='process_crashed'
                onChange={(event) =>
                  setRouteForm((prev) => ({
                    ...prev,
                    rule_key: event.target.value,
                  }))
                }
              />
            </div>
          </div>

          <div className='grid grid-cols-1 gap-4 md:grid-cols-2'>
            <div className='flex items-center gap-3'>
              <Switch
                checked={routeForm.enabled}
                onCheckedChange={(checked) =>
                  setRouteForm((prev) => ({...prev, enabled: checked}))
                }
              />
              <Label>{t('phase3.routeEnabled')}</Label>
            </div>
            <div className='flex items-center gap-3'>
              <Switch
                checked={routeForm.send_resolved}
                onCheckedChange={(checked) =>
                  setRouteForm((prev) => ({...prev, send_resolved: checked}))
                }
              />
              <Label>{t('phase3.routeSendResolved')}</Label>
            </div>
            <div className='flex items-center gap-3'>
              <Switch
                checked={routeForm.mute_if_acknowledged}
                onCheckedChange={(checked) =>
                  setRouteForm((prev) => ({
                    ...prev,
                    mute_if_acknowledged: checked,
                  }))
                }
              />
              <Label>{t('phase3.routeMuteIfAcknowledged')}</Label>
            </div>
            <div className='flex items-center gap-3'>
              <Switch
                checked={routeForm.mute_if_silenced}
                onCheckedChange={(checked) =>
                  setRouteForm((prev) => ({
                    ...prev,
                    mute_if_silenced: checked,
                  }))
                }
              />
              <Label>{t('phase3.routeMuteIfSilenced')}</Label>
            </div>
          </div>

          <Button
            onClick={handleCreateRoute}
            disabled={creatingRoute || channels.length === 0}
          >
            {t('phase3.routeCreate')}
          </Button>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t('phase3.routeListTitle')}</CardTitle>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t('phase3.routeName')}</TableHead>
                <TableHead>{t('phase3.routeMatch')}</TableHead>
                <TableHead>{t('phase3.routeChannel')}</TableHead>
                <TableHead>{t('phase3.routeEnabled')}</TableHead>
                <TableHead>{t('actions')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {loadingRoutes ? (
                <TableRow>
                  <TableCell
                    colSpan={5}
                    className='text-center text-muted-foreground'
                  >
                    {t('loading')}
                  </TableCell>
                </TableRow>
              ) : !routes.length ? (
                <TableRow>
                  <TableCell
                    colSpan={5}
                    className='text-center text-muted-foreground'
                  >
                    {t('phase3.routeEmpty')}
                  </TableCell>
                </TableRow>
              ) : (
                routes.map((route) => (
                  <TableRow key={route.id}>
                    <TableCell>{route.name}</TableCell>
                    <TableCell className='max-w-[420px] break-all'>
                      {resolveRouteSummary(route)}
                    </TableCell>
                    <TableCell>
                      {channelNameMap.get(route.channel_id) || route.channel_id}
                    </TableCell>
                    <TableCell>
                      <Switch
                        checked={route.enabled}
                        disabled={updatingRouteId === route.id}
                        onCheckedChange={(checked) =>
                          handleToggleRouteEnabled(route, checked)
                        }
                      />
                    </TableCell>
                    <TableCell>
                      <Button
                        size='sm'
                        variant='destructive'
                        disabled={deletingRouteId === route.id}
                        onClick={() => handleDeleteRoute(route.id)}
                      >
                        {t('phase3.delete')}
                      </Button>
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
