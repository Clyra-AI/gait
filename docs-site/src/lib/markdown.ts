import path from 'path';
import { marked } from 'marked';

function slugify(text: string): string {
  return text
    .toLowerCase()
    .replace(/[^\w\s-]/g, '')
    .replace(/\s+/g, '-')
    .replace(/-+/g, '-')
    .trim();
}

function convertMarkdownHref(href: string, currentSlug: string): string {
  if (!href || href.startsWith('http://') || href.startsWith('https://') || href.startsWith('#')) {
    return href;
  }

  if (!href.endsWith('.md')) {
    return href;
  }

  const cleanHref = href.replace(/^\//, '');

  if (cleanHref.startsWith('docs/')) {
    return `/docs/${cleanHref.slice('docs/'.length).replace(/\.md$/i, '').toLowerCase()}`;
  }
  if (cleanHref === 'README.md') {
    return '/docs/start-here';
  }
  if (cleanHref === 'SECURITY.md') {
    return '/docs/security';
  }
  if (cleanHref === 'CONTRIBUTING.md') {
    return '/docs/contributing';
  }

  const currentDir = path.posix.dirname(currentSlug);
  const resolved = path.posix.normalize(path.posix.join(currentDir, cleanHref));
  const target = resolved.replace(/\.md$/i, '').toLowerCase();
  if (target === 'readme') {
    return '/docs/start-here';
  }
  return `/docs/${target}`;
}

marked.setOptions({
  gfm: true,
  breaks: false,
});

function rendererForSlug(currentSlug: string) {
  const renderer = new marked.Renderer();
  renderer.link = function ({ href = '', title, text }) {
    const mappedHref = convertMarkdownHref(href, currentSlug);
    if (mappedHref.startsWith('http://') || mappedHref.startsWith('https://')) {
      return `<a href="${mappedHref}" target="_blank" rel="noopener noreferrer"${title ? ` title="${title}"` : ''}>${text}</a>`;
    }
    return `<a href="${mappedHref}"${title ? ` title="${title}"` : ''}>${text}</a>`;
  };

  renderer.code = function ({ text, lang }) {
    const language = (lang || '').toLowerCase();
    if (language === 'mermaid') {
      return `<div class="mermaid">${text}</div>`;
    }

    const escaped = text
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;');
    return `<pre><code class="language-${language}">${escaped}</code></pre>`;
  };

  renderer.codespan = function ({ text }) {
    return `<code class="inline-code">${text}</code>`;
  };

  renderer.heading = function ({ text, depth }) {
    const slug = slugify(text);
    return `<h${depth} id="${slug}">${text}</h${depth}>`;
  };

  return renderer;
}

export function markdownToHtml(markdown: string, currentSlug = ''): string {
  return marked.parse(markdown, { renderer: rendererForSlug(currentSlug) }) as string;
}
