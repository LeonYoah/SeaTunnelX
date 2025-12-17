/**
 * Host Management Page
 * 主机管理页面
 */

import {Suspense} from 'react';
import {HostMain} from '@/components/common/host';
import {Metadata} from 'next';

export const metadata: Metadata = {
  title: '主机管理',
};

export default function HostsPage() {
  return (
    <div className='container max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8'>
      <Suspense>
        <HostMain />
      </Suspense>
    </div>
  );
}
