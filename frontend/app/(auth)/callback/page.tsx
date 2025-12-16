import {Suspense} from 'react';
import {CallbackHandler} from '@/components/common/auth/CallbackHandler';
import {Metadata} from 'next';

export const metadata: Metadata = {
  title: '登录处理',
};

export default function AuthCallbackPage() {
  return (
    <div className="bg-background flex min-h-svh flex-col items-center justify-center gap-6 p-6 md:p-10">
      <div className="w-full max-w-sm">
        <Suspense>
          <CallbackHandler />
        </Suspense>
      </div>
    </div>
  );
}
