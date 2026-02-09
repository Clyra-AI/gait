import fs from 'fs';
import path from 'path';
import matter from 'gray-matter';

export interface DocMeta {
  slug: string;
  title: string;
  description?: string;
}

interface DocSource {
  slug: string;
  title?: string;
  filePath: string;
}

const repoRoot = path.join(process.cwd(), '..');
const docsDirectory = path.join(repoRoot, 'docs');

const rootDocSources: DocSource[] = [
  {
    slug: 'security',
    title: 'Security Policy',
    filePath: path.join(repoRoot, 'SECURITY.md'),
  },
  {
    slug: 'contributing',
    title: 'Contributing Guide',
    filePath: path.join(repoRoot, 'CONTRIBUTING.md'),
  },
  {
    slug: 'start-here',
    title: 'README',
    filePath: path.join(repoRoot, 'README.md'),
  },
];

function toSlug(raw: string): string {
  return raw.replace(/\\/g, '/').replace(/\.md$/i, '').toLowerCase();
}

function extractTitle(markdown: string, fallback: string): string {
  const heading = markdown
    .split('\n')
    .map((line) => line.trim())
    .find((line) => line.startsWith('# '));
  if (!heading) {
    return fallback;
  }
  return heading.replace(/^#\s+/, '').trim();
}

function collectDocsFromTree(): DocSource[] {
  const docs: DocSource[] = [];

  function walk(dir: string, prefix = ''): void {
    const entries = fs.readdirSync(dir, { withFileTypes: true });
    for (const entry of entries) {
      const fullPath = path.join(dir, entry.name);
      if (entry.isDirectory()) {
        walk(fullPath, path.join(prefix, entry.name));
        continue;
      }
      if (!entry.name.endsWith('.md')) {
        continue;
      }
      const relPath = path.join(prefix, entry.name);
      const slug = toSlug(relPath);
      if (slug === 'readme') {
        continue;
      }
      docs.push({ slug, filePath: fullPath });
    }
  }

  if (fs.existsSync(docsDirectory)) {
    walk(docsDirectory);
  }

  return docs;
}

function allSources(): DocSource[] {
  return [...collectDocsFromTree(), ...rootDocSources.filter((source) => fs.existsSync(source.filePath))];
}

export function getAllDocSlugs(): string[] {
  return allSources().map((source) => source.slug).sort((a, b) => a.localeCompare(b));
}

export function getDocsIndex(): DocMeta[] {
  return allSources()
    .map((source) => {
      const content = fs.readFileSync(source.filePath, 'utf-8');
      const { data, content: markdown } = matter(content);
      const title = source.title || (typeof data.title === 'string' ? data.title : extractTitle(markdown, source.slug));
      const description = typeof data.description === 'string' ? data.description : undefined;
      return {
        slug: source.slug,
        title,
        description,
      };
    })
    .sort((a, b) => a.title.localeCompare(b.title));
}

export function getDocContent(slug: string): { content: string; title: string; description?: string } | null {
  const normalizedSlug = toSlug(slug);
  const source = allSources().find((item) => item.slug === normalizedSlug);
  if (!source) {
    return null;
  }

  const fileContents = fs.readFileSync(source.filePath, 'utf-8');
  const { data, content } = matter(fileContents);
  const title = source.title || (typeof data.title === 'string' ? data.title : extractTitle(content, normalizedSlug));
  const description = typeof data.description === 'string' ? data.description : undefined;

  return {
    content,
    title,
    description,
  };
}
