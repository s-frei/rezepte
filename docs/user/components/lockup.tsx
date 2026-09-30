type Props = {
	variant: 'horizontal';
	/** Sizes it; the other side follows the lockup's own proportions. */
	className?: string;
};

// Plain <img> and not next/image: the lockups are SVGs, which next/image does
// not optimize anyway, and its unoptimized loader would still need the base
// path added by hand. One file serves both themes: each lockup carries its own
// cream box, as in the app.
const basePath = process.env.NEXT_PUBLIC_BASE_PATH ?? '';

export function Lockup({ variant, className = '' }: Props) {
	const src = `${basePath}/brand/rezepte-lockup-${variant}.svg`;
	// eslint-disable-next-line @next/next/no-img-element -- see above
	return <img src={src} alt="Rezepte" className={className} />;
}
