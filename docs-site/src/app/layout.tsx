import type { Metadata } from 'next';
import './globals.css';
import Sidebar from '@/components/Sidebar';
import Header from '@/components/Header';
import { SITE_BASE_PATH, SITE_ORIGIN } from '@/lib/site';

export const metadata: Metadata = {
  metadataBase: new URL(`${SITE_ORIGIN}${SITE_BASE_PATH}`),
  title: 'Gait | Durable Agent Runtime with Signed Proof',
  description:
    'Gait is an offline-first runtime for production AI agents: durable jobs, signed packs, voice agent gating, context evidence, deterministic regressions, and fail-closed policy gates.',
  keywords:
    'agent control plane, agent safety, ai governance, durable agent jobs, signed packs, voice agent gating, context evidence, callpack, saytoken, policy gate, deterministic regression, agent replay, prompt injection defense, ai incident response',
  openGraph: {
    title: 'Gait | Durable Agent Runtime with Signed Proof',
    description:
      'Offline-first runtime for production AI agents: durable jobs, signed packs, voice gating, context evidence, and fail-closed policy enforcement.',
    url: 'https://davidahmann.github.io/gait',
    siteName: 'Gait',
    type: 'website',
    images: [
      {
        url: '/og.svg',
        width: 1200,
        height: 630,
        alt: 'Gait',
      },
    ],
  },
  icons: {
    icon: [
      { url: `${SITE_BASE_PATH}/favicon.svg`, type: 'image/svg+xml' },
      { url: `${SITE_BASE_PATH}/favicon.ico`, type: 'image/x-icon' },
    ],
    shortcut: `${SITE_BASE_PATH}/favicon.ico`,
    apple: `${SITE_BASE_PATH}/favicon.svg`,
  },
  twitter: {
    card: 'summary_large_image',
    title: 'Gait | Durable Agent Runtime with Signed Proof',
    description:
      'Durable agent jobs, signed packs, voice gating, context evidence, and fail-closed policy enforcement for production AI agents.',
    images: ['/og.svg'],
  },
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en" className="dark">
      <body className="antialiased">
        <Header />
        <div className="flex max-w-7xl mx-auto px-4 lg:px-8">
          <Sidebar />
          <main className="flex-1 min-w-0 py-8 lg:pl-8">
            <article className="prose prose-invert max-w-none">{children}</article>
          </main>
        </div>
      </body>
    </html>
  );
}
