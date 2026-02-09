'use client';

import Link from 'next/link';
import { usePathname } from 'next/navigation';
import { navigation, NavItem } from '@/lib/navigation';

function NavLink({ item }: { item: NavItem }) {
  const pathname = usePathname();
  const isActive = pathname === item.href || pathname === `${item.href}/`;

  return (
    <Link
      href={item.href}
      className={`block py-1.5 px-3 text-sm rounded-md transition-colors ml-4 border-l-2 ${
        isActive
          ? 'bg-cyan-500/10 text-cyan-400 border-cyan-400'
          : 'text-gray-400 hover:text-gray-200 hover:bg-gray-800/50 border-transparent'
      }`}
    >
      {item.title}
    </Link>
  );
}

function Section({ item }: { item: NavItem }) {
  const pathname = usePathname();
  const active = item.children?.some((child) => pathname === child.href || pathname === `${child.href}/`);

  return (
    <div className="mb-6">
      <h3 className={`px-3 mb-2 text-xs font-semibold uppercase tracking-wider ${active ? 'text-cyan-400' : 'text-gray-500'}`}>
        {item.title}
      </h3>
      <div className="space-y-1">{item.children?.map((child) => <NavLink key={child.href} item={child} />)}</div>
    </div>
  );
}

export default function Sidebar() {
  return (
    <aside className="w-64 flex-shrink-0 hidden lg:block">
      <div className="sticky top-0 h-screen overflow-y-auto py-8 pr-4">
        <Link href="/" className="flex items-center gap-2 px-3 mb-6">
          <div className="w-8 h-8 rounded-lg bg-gradient-to-br from-cyan-500 to-blue-600 flex items-center justify-center">
            <span className="text-white font-bold text-sm">G</span>
          </div>
          <span className="text-xl font-bold text-white">Gait</span>
        </Link>

        <nav>
          {navigation.map((section) => (
            <Section key={section.title} item={section} />
          ))}
        </nav>

        <div className="mt-8 pt-8 border-t border-gray-800 px-3 space-y-3">
          <Link href="/llms" className="block text-sm text-gray-400 hover:text-gray-200">
            LLM Context
          </Link>
          <a
            href="https://github.com/davidahmann/gait"
            target="_blank"
            rel="noopener noreferrer"
            className="block text-sm text-gray-400 hover:text-gray-200"
          >
            GitHub
          </a>
        </div>
      </div>
    </aside>
  );
}
