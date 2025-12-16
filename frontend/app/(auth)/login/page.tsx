import {Suspense} from 'react';
import {LoginForm} from '@/components/common/auth/LoginForm';
import {Metadata} from 'next';

export const metadata: Metadata = {
  title: '登录',
};

export default function LoginPage() {
  return (
    <div className="bg-background flex min-h-svh flex-col items-center justify-center gap-6 p-6 md:p-10">
      <div className="w-full max-w-sm">
        <Suspense>
          <LoginForm />
        </Suspense>
      </div>
    </div>
  );
}
