/**
 * Cluster Detail Page
 * 集群详情页面
 */

import {Suspense} from 'react';
import {ClusterDetail} from '@/components/common/cluster';
import {Metadata} from 'next';

export const metadata: Metadata = {
  title: '集群详情',
};

interface ClusterDetailPageProps {
  params: Promise<{id: string}>;
}

export default async function ClusterDetailPage({params}: ClusterDetailPageProps) {
  const {id} = await params;
  const clusterId = parseInt(id, 10);

  return (
    <div className='container max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8'>
      <Suspense>
        <ClusterDetail clusterId={clusterId} />
      </Suspense>
    </div>
  );
}
