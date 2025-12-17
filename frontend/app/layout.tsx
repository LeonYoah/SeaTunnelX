import type {Metadata} from 'next';
// eslint-disable-next-line camelcase
import {Inter, Noto_Sans_SC} from 'next/font/google';
import {Toaster} from '@/components/ui/sonner';
import {ThemeProvider} from '@/components/common/layout/ThemeProvider';
import {I18nProvider} from '@/lib/i18n';
import './globals.css';

// eslint-disable-next-line new-cap
const inter = Inter({
  variable: '--font-inter',
  subsets: ['latin'],
  display: 'swap',
});

// eslint-disable-next-line new-cap
const notoSansSC = Noto_Sans_SC({
  variable: '--font-noto-sans-sc',
  subsets: ['latin'],
  display: 'swap',
  weight: ['300', '400', '500', '600', '700'],
});

export const metadata: Metadata = {
  title: {
    template: '%s - Seatunnel X',
    default: 'Seatunnel X',
  },
  description: 'Seatunnel X ,Seatunnel的一站式运维管理平台',
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html
      lang='zh-CN'
      className={`${inter.variable} ${notoSansSC.variable} hide-scrollbar font-sans`}
      suppressHydrationWarning
    >
      <body
        className={`${inter.variable} ${notoSansSC.variable} hide-scrollbar font-sans antialiased`}
      >
        <I18nProvider>
          <ThemeProvider
            attribute='class'
            defaultTheme='system'
            enableSystem
            disableTransitionOnChange
          >
            {children}
            <Toaster />
          </ThemeProvider>
        </I18nProvider>
      </body>
    </html>
  );
}
