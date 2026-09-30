import { existsSync } from 'node:fs';
import path from 'node:path';

type Props = {
	/** A scene painted by scripts/hero-images.ts into public/heroes. */
	name: 'guests' | 'laptop' | 'phone';
	/** Alt text: what the picture shows. */
	alt: string;
};

// See components/screenshot.tsx: the base path has to be added by hand.
const basePath = process.env.NEXT_PUBLIC_BASE_PATH ?? '';
const frame = 'not-prose my-6 w-full rounded-xl border border-fd-border';

/**
 * A wide mascot scene. Scenes with a screen come in a light and a dark
 * variant, the app's screenshot in each theme, swapped like <Screenshot>;
 * the others are one picture for both.
 */
export function Hero({ name, alt }: Props) {
	const themed = existsSync(path.join(process.cwd(), 'public', 'heroes', `${name}-light.jpg`));
	/* eslint-disable @next/next/no-img-element -- plain files under the base path, see screenshot.tsx */
	if (!themed) return <img src={`${basePath}/heroes/${name}.jpg`} alt={alt} className={frame} />;
	return (
		<>
			<img src={`${basePath}/heroes/${name}-light.jpg`} alt={alt} className={`${frame} dark:hidden`} />
			<img src={`${basePath}/heroes/${name}-dark.jpg`} alt={alt} className={`${frame} hidden dark:block`} />
		</>
	);
	/* eslint-enable @next/next/no-img-element */
}
