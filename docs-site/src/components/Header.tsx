'use client';

import Link from 'next/link';
import { useState } from 'react';
import { navigation } from '@/lib/navigation';

export default function Header() {
  const [open, setOpen] = useState(false);

  return (
    <header className="lg:hidden sticky top-0 z-50 bg-gray-900/95 backdrop-blur border-b border-gray-800">
      <div className="flex items-center justify-between px-4 py-3">
        <Link href="/" className="flex items-center gap-2">
          <div className="w-8 h-8 rounded-lg bg-gradient-to-br from-cyan-500 to-blue-600 flex items-center justify-center">
            <span className="text-white font-bold text-sm">G</span>
          </div>
          <span className="text-lg font-bold text-white">Gait</span>
        </Link>

        <button onClick={() => setOpen(!open)} className="p-2 text-gray-400 hover:text-white" aria-label="Toggle navigation">
          {open ? 'Close' : 'Menu'}
        </button>
      </div>

      {open && (
        <nav className="px-4 pb-4 max-h-[70vh] overflow-y-auto">
          {navigation.map((section) => (
            <div key={section.title} className="mb-4">
              <h3 className="text-xs font-semibold uppercase tracking-wider text-gray-500 mb-2">{section.title}</h3>
              <div className="space-y-1">
                {section.children?.map((item) => (
                  <Link
                    key={item.href}
                    href={item.href}
                    onClick={() => setOpen(false)}
                    className="block py-2 px-3 text-sm text-gray-400 hover:text-white hover:bg-gray-800 rounded-md"
                  >
                    {item.title}
                  </Link>
                ))}
              </div>
            </div>
          ))}
        </nav>
      )}
    </header>
  );
}
