import {Suspense} from 'react';
import {DashboardMain} from '@/components/common/dashboard';
import {Metadata} from 'next';

export const metadata: Metadata = {
  title: '实时数据',
};

export default function DashboardPage() {
  return (
    <div className="container max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
      <Suspense>
        <DashboardMain />
      </Suspense>
    </div>
  );
}
