/**
 * Host Detail Page
 * 主机详情页面
 */

import {Suspense} from 'react';
import Link from 'next/link';
import {HostDetailPage} from '@/components/common/host/HostDetailPage';
import {Metadata} from 'next';

export const metadata: Metadata = {
  title: '主机详情',
};

interface HostDetailPageProps {
  params: Promise<{id: string}>;
}

export default async function HostDetailRoute({params}: HostDetailPageProps) {
  const {id} = await params;
  const hostId = parseInt(id, 10);

  if (isNaN(hostId) || hostId < 1) {
    return (
      <div className='container max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8'>
        <p className='text-destructive'>无效的主机 ID</p>
        <Link href='/hosts' className='text-primary underline mt-4 inline-block'>
          返回主机列表
        </Link>
      </div>
    );
  }

  return (
    <div className='container max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8'>
      <Suspense>
        <HostDetailPage hostId={hostId} />
      </Suspense>
    </div>
  );
}
