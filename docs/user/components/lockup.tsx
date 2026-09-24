type Props = {
	variant: 'compact' | 'horizontal';
	/** Sets the height; the width follows the lockup's own proportions. */
	className?: string;
};

// Plain <img> and not next/image: the lockups are SVGs, which next/image does
// not optimize anyway, and its unoptimized loader would still need the base
// path added by hand. Light and dark are two files swapped by the theme class,
// like the screenshots (components/screenshot.tsx).
const basePath = process.env.NEXT_PUBLIC_BASE_PATH ?? '';

export function Lockup({ variant, className = '' }: Props) {
	const src = (scheme: '' | '-dark') => `${basePath}/brand/rezepte-lockup-${variant}${scheme}.svg`;
	return (
		<>
			{/* eslint-disable-next-line @next/next/no-img-element -- see above */}
			<img src={src('')} alt="Rezepte" className={`w-auto dark:hidden ${className}`} />
			{/* eslint-disable-next-line @next/next/no-img-element -- see above */}
			<img src={src('-dark')} alt="Rezepte" className={`hidden w-auto dark:block ${className}`} />
		</>
	);
}
