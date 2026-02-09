'use client';

import { useEffect, useRef } from 'react';
import hljs from 'highlight.js/lib/core';
import bash from 'highlight.js/lib/languages/bash';
import json from 'highlight.js/lib/languages/json';
import yaml from 'highlight.js/lib/languages/yaml';
import go from 'highlight.js/lib/languages/go';
import python from 'highlight.js/lib/languages/python';

hljs.registerLanguage('bash', bash);
hljs.registerLanguage('sh', bash);
hljs.registerLanguage('json', json);
hljs.registerLanguage('yaml', yaml);
hljs.registerLanguage('yml', yaml);
hljs.registerLanguage('go', go);
hljs.registerLanguage('python', python);

interface Props {
  html: string;
}

export default function MarkdownRenderer({ html }: Props) {
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const root = ref.current;
    if (!root) {
      return;
    }

    root.querySelectorAll('pre code').forEach((block) => {
      hljs.highlightElement(block as HTMLElement);
    });

    const mermaidNodes = root.querySelectorAll<HTMLElement>('.mermaid');
    if (mermaidNodes.length > 0) {
      void import('mermaid').then((module) => {
        const mermaid = module.default;
        mermaid.initialize({
          startOnLoad: false,
          theme: 'dark',
          securityLevel: 'strict',
        });
        void mermaid.run({ nodes: mermaidNodes });
      });
    }
  }, [html]);

  return <div ref={ref} className="prose prose-invert max-w-none" dangerouslySetInnerHTML={{ __html: html }} />;
}
