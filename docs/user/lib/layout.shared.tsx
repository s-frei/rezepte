import type { BaseLayoutProps } from 'fumadocs-ui/layouts/shared';

export function baseOptions(): BaseLayoutProps {
	return {
		// The brand carries the app's display face; Fumadocs renders whatever
		// node it is given, which is steadier than styling its generated markup.
		nav: { title: <span className="font-display font-semibold">Rezepte</span> },
		githubUrl: 'https://github.com/s-frei/rezepte',
		// The provider already defaults to `system`; without this the switch only
		// offers light and dark, so a visitor following the OS cannot get back to it
		// once they have touched the toggle.
		themeSwitch: { mode: 'light-dark-system' }
	};
}
