import type { BaseLayoutProps } from 'fumadocs-ui/layouts/shared';

export function baseOptions(): BaseLayoutProps {
	return {
		// The brand carries the app's display face; Fumadocs renders whatever
		// node it is given, which is steadier than styling its generated markup.
		nav: { title: <span className="font-display font-semibold">Rezepte</span> },
		githubUrl: 'https://github.com/s-frei/rezepte'
	};
}
