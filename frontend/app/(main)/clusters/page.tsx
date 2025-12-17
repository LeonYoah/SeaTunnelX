/**
 * Cluster Management Page
 * 集群管理页面
 */

import {Suspense} from 'react';
import {ClusterMain} from '@/components/common/cluster';
import {Metadata} from 'next';

export const metadata: Metadata = {
  title: '集群管理',
};

export default function ClustersPage() {
  return (
    <div className='container max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8'>
      <Suspense>
        <ClusterMain />
      </Suspense>
    </div>
  );
}
