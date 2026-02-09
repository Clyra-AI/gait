export const SITE_ORIGIN = 'https://davidahmann.github.io';
export const SITE_BASE_PATH = '/gait';

export function canonicalUrl(pathname: string): string {
  const normalized = pathname.startsWith('/') ? pathname : `/${pathname}`;
  return `${SITE_ORIGIN}${SITE_BASE_PATH}${normalized}`;
}
