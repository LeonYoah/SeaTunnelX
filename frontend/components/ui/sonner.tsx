'use client';

import {useTheme} from 'next-themes';
import {Toaster as Sonner, ToasterProps} from 'sonner';

const Toaster = ({...props}: ToasterProps) => {
  const {theme = 'system'} = useTheme();

  return (
    <Sonner
      theme={theme as ToasterProps['theme']}
      className="toaster group"
      position="top-center"
      toastOptions={{
        style: {
          background: 'var(--popover)',
          border: '1px solid var(--border)',
          color: 'var(--popover-foreground)',
        },
        className: 'group',
        unstyled: false,
      }}
      style={
        {
          '--normal-bg': 'var(--popover)',
          '--normal-text': 'var(--popover-foreground)',
          '--normal-border': 'var(--border)',
          '--success-bg': 'var(--popover)',
          '--success-text': 'var(--popover-foreground)',
          '--success-border': 'hsl(var(--border))',
          '--error-bg': 'var(--popover)',
          '--error-text': 'var(--popover-foreground)',
          '--error-border': 'hsl(var(--border))',
          '--warning-bg': 'var(--popover)',
          '--warning-text': 'var(--popover-foreground)',
          '--warning-border': 'hsl(var(--border))',
          '--info-bg': 'var(--popover)',
          '--info-text': 'var(--popover-foreground)',
          '--info-border': 'hsl(var(--border))',
        } as React.CSSProperties
      }
      {...props}
    />
  );
};

export {Toaster};
