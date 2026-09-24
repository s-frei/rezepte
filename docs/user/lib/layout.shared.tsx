import type { BaseLayoutProps } from 'fumadocs-ui/layouts/shared';
import { Lockup } from '@/components/lockup';

export function baseOptions(): BaseLayoutProps {
	return {
		// The compact lockup, as in the app's top bar; Fumadocs renders whatever
		// node it is given, which is steadier than styling its generated markup.
		nav: { title: <Lockup variant="compact" className="h-8" /> },
		githubUrl: 'https://github.com/s-frei/rezepte',
		// The provider already defaults to `system`; without this the switch only
		// offers light and dark, so a visitor following the OS cannot get back to it
		// once they have touched the toggle.
		themeSwitch: { mode: 'light-dark-system' }
	};
}
