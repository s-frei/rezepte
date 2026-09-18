'use client';

import { useTheme } from 'next-themes';
import { useEffect, useId, useState } from 'react';

export function Mermaid({ chart }: { chart: string }) {
	const id = useId().replace(/[^a-zA-Z0-9]/g, '');
	const { resolvedTheme } = useTheme();
	const [svg, setSvg] = useState('');

	useEffect(() => {
		let cancelled = false;
		void (async () => {
			const mermaid = (await import('mermaid')).default;
			mermaid.initialize({
				startOnLoad: false,
				securityLevel: 'loose',
				theme: resolvedTheme === 'dark' ? 'dark' : 'default'
			});
			const { svg } = await mermaid.render(`mermaid-${id}`, chart.replaceAll('\\n', '\n'));
			if (!cancelled) setSvg(svg);
		})();
		return () => {
			cancelled = true;
		};
	}, [chart, id, resolvedTheme]);

	return <div className="my-4 overflow-x-auto" dangerouslySetInnerHTML={{ __html: svg }} />;
}
