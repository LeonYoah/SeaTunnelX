'use client';

/**
 * Host Detail Page Component
 * 主机详情页面组件
 *
 * Fetches host by ID and displays in HostDetail sheet.
 * 根据 ID 获取主机并在 HostDetail 中展示。
 */

import {useState, useEffect, useCallback} from 'react';
import {useRouter} from 'next/navigation';
import {useTranslations} from 'next-intl';
import {Button} from '@/components/ui/button';
import {ArrowLeft, Loader2} from 'lucide-react';
import Link from 'next/link';
import services from '@/lib/services';
import {HostInfo} from '@/lib/services/host/types';
import {HostDetail} from './HostDetail';
import {EditHostDialog} from './EditHostDialog';

interface HostDetailPageProps {
  hostId: number;
}

export function HostDetailPage({hostId}: HostDetailPageProps) {
  const t = useTranslations();
  const router = useRouter();
  const [host, setHost] = useState<HostInfo | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [detailOpen, setDetailOpen] = useState(false);

  const loadHost = useCallback(async () => {
    setLoading(true);
    setError(null);
    const result = await services.host.getHostSafe(hostId);
    setLoading(false);
    if (result.success && result.data) {
      setHost(result.data);
      setDetailOpen(true);
    } else {
      setError(result.error || t('host.loadError'));
    }
  }, [hostId, t]);

  useEffect(() => {
    loadHost();
  }, [loadHost]);

  const handleDetailClose = (open: boolean) => {
    if (!open) {
      router.push('/hosts');
    }
  };

  const [isEditOpen, setIsEditOpen] = useState(false);

  const handleEdit = () => {
    setIsEditOpen(true);
  };

  const handleEditSuccess = () => {
    setIsEditOpen(false);
    loadHost();
  };

  if (loading) {
    return (
      <div className='flex flex-col items-center justify-center py-16'>
        <Loader2 className='h-8 w-8 animate-spin text-muted-foreground' />
        <p className='mt-4 text-sm text-muted-foreground'>{t('common.loading')}</p>
      </div>
    );
  }

  if (error || !host) {
    return (
      <div className='flex flex-col items-center justify-center py-16'>
        <p className='text-destructive mb-4'>{error || t('host.loadError')}</p>
        <Link href='/hosts'>
          <Button variant='outline'>
            <ArrowLeft className='h-4 w-4 mr-2' />
            {t('common.back')}
          </Button>
        </Link>
      </div>
    );
  }

  return (
    <div>
      <div className='mb-6'>
        <Link href='/hosts'>
          <Button variant='ghost' size='sm'>
            <ArrowLeft className='h-4 w-4 mr-2' />
            {t('common.back')}
          </Button>
        </Link>
      </div>
      <HostDetail
        open={detailOpen}
        onOpenChange={handleDetailClose}
        host={host}
        onEdit={handleEdit}
      />
      {host && (
        <EditHostDialog
          open={isEditOpen}
          onOpenChange={setIsEditOpen}
          host={host}
          onSuccess={handleEditSuccess}
        />
      )}
    </div>
  );
}
