type Props = {
	variant: 'horizontal';
	/** Sizes it; the other side follows the lockup's own proportions. */
	className?: string;
};

// Plain <img> and not next/image: the lockups are SVGs, which next/image does
// not optimize anyway. One file serves both themes: each lockup carries its own
// cream box, as in the app.

export function Lockup({ variant, className = '' }: Props) {
	const src = `/brand/rezepte-lockup-${variant}.svg`;
	// eslint-disable-next-line @next/next/no-img-element -- see above
	return <img src={src} alt="Rezepte" className={className} />;
}
