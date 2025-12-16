import {Suspense} from 'react';
import {ReceivedMain} from '@/components/common/received';
import {Metadata} from 'next';

export const metadata: Metadata = {
  title: '我的领取',
};

export default function ReceivedPage() {
  return (
    <div className="container max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
      <Suspense>
        <ReceivedMain />
      </Suspense>
    </div>
  );
}
