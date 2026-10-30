'use client';

import { useTheme } from 'next-themes';
import { useEffect, useId, useState } from 'react';

export function Mermaid({ chart }: { chart: string }) {
	const id = useId().replace(/[^a-zA-Z0-9]/g, '');
	const { resolvedTheme } = useTheme();
	const [svg, setSvg] = useState('');

	useEffect(() => {
		let canceled = false;
		void (async () => {
			const mermaid = (await import('mermaid')).default;
			// The docs' own palette instead of Mermaid's purple default: the
			// `base` theme takes its colors from the --color-fd-* tokens that
			// app/global.css sets for the current theme.
			const token = (name: string) => getComputedStyle(document.documentElement).getPropertyValue(`--color-fd-${name}`).trim();
			mermaid.initialize({
				startOnLoad: false,
				securityLevel: 'loose',
				theme: 'base',
				themeVariables: {
					darkMode: resolvedTheme === 'dark',
					fontFamily: 'Karla, system-ui, sans-serif',
					background: token('background'),
					primaryColor: token('card'),
					primaryTextColor: token('foreground'),
					primaryBorderColor: token('border'),
					secondaryColor: token('muted'),
					tertiaryColor: token('muted'),
					lineColor: token('muted-foreground'),
					textColor: token('foreground'),
					actorBkg: token('card'),
					actorBorder: token('border'),
					actorTextColor: token('foreground'),
					actorLineColor: token('border'),
					signalColor: token('muted-foreground'),
					signalTextColor: token('foreground'),
					edgeLabelBackground: token('background'),
					clusterBkg: token('muted')
				},
				// The base theme's drop shadow reads as a glow on dark paper.
				themeCSS: '* { filter: none !important; }'
			});
			const { svg } = await mermaid.render(`mermaid-${id}`, chart.replaceAll('\\n', '\n'));
			if (!canceled) setSvg(svg);
		})();
		return () => {
			canceled = true;
		};
	}, [chart, id, resolvedTheme]);

	return <div className="my-4 overflow-x-auto" dangerouslySetInnerHTML={{ __html: svg }} />;
}
