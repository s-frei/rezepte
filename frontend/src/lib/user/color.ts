/**
 * The color palette a person picks from, in palette order - the same order
 * and the same names as `user.Colors` in service/internal/user/profile.go and
 * the `--color-user-*` tokens in app.css. Adding a color means editing all
 * three; the API's enum is what keeps a client from inventing a ninth.
 */
export const USER_COLORS = [
	'amber',
	'clay',
	'rose',
	'plum',
	'sage',
	'olive',
	'teal',
	'slate'
] as const;

export type UserColor = (typeof USER_COLORS)[number];

/**
 * The one place a color becomes classes. Tailwind cannot compose a class
 * name at runtime - `bg-user-${color}` is never emitted - so every pair is
 * written out.
 */
export const USER_COLOR_CLASSES: Record<UserColor, string> = {
	amber: 'bg-user-amber text-user-amber-foreground',
	clay: 'bg-user-clay text-user-clay-foreground',
	rose: 'bg-user-rose text-user-rose-foreground',
	plum: 'bg-user-plum text-user-plum-foreground',
	sage: 'bg-user-sage text-user-sage-foreground',
	olive: 'bg-user-olive text-user-olive-foreground',
	teal: 'bg-user-teal text-user-teal-foreground',
	slate: 'bg-user-slate text-user-slate-foreground'
};

/**
 * The same pair turned over, the strong tone as ground: a mark set on a
 * ribbon in the person's own color (the public page's "Shared by"), where
 * the plain pair would melt into it. Written out for the same reason.
 */
const USER_COLOR_CLASSES_STRONG: Record<UserColor, string> = {
	amber: 'bg-user-amber-foreground text-user-amber',
	clay: 'bg-user-clay-foreground text-user-clay',
	rose: 'bg-user-rose-foreground text-user-rose',
	plum: 'bg-user-plum-foreground text-user-plum',
	sage: 'bg-user-sage-foreground text-user-sage',
	olive: 'bg-user-olive-foreground text-user-olive',
	teal: 'bg-user-teal-foreground text-user-teal',
	slate: 'bg-user-slate-foreground text-user-slate'
};

/**
 * The ring around a person's photo (`PersonMark`) and the top bar's own
 * avatar: an outline in the pair's strong tone, which holds against the
 * page and the surface in both themes. Written out for the same reason.
 */
const USER_RINGS: Record<UserColor, string> = {
	amber: 'outline-user-amber-foreground',
	clay: 'outline-user-clay-foreground',
	rose: 'outline-user-rose-foreground',
	plum: 'outline-user-plum-foreground',
	sage: 'outline-user-sage-foreground',
	olive: 'outline-user-olive-foreground',
	teal: 'outline-user-teal-foreground',
	slate: 'outline-user-slate-foreground'
};

/**
 * The same ring on the `inverse` ground (the author popover): there the
 * strong tone is the ground's own lightness, so the pair's base tone rings.
 */
const USER_RINGS_INVERSE: Record<UserColor, string> = {
	amber: 'outline-user-amber',
	clay: 'outline-user-clay',
	rose: 'outline-user-rose',
	plum: 'outline-user-plum',
	sage: 'outline-user-sage',
	olive: 'outline-user-olive',
	teal: 'outline-user-teal',
	slate: 'outline-user-slate'
};

/** The ring's outline color for a color from the API; muted for an unknown one. */
export function userRingClass(color: string | null | undefined, inverse = false): string {
	if (!isUserColor(color)) return inverse ? 'outline-inverse-muted' : 'outline-text-muted';
	return (inverse ? USER_RINGS_INVERSE : USER_RINGS)[color];
}

/**
 * The overview's dotted leader in the signed-in person's own tone - their
 * app, tinted for them. At 55%: at full strength the dots outweighed the
 * section name, and amber and clay read as the primary orange beside it.
 * Written out for the same reason.
 */
const USER_LEADERS: Record<UserColor, string> = {
	amber: 'border-user-amber-foreground/55',
	clay: 'border-user-clay-foreground/55',
	rose: 'border-user-rose-foreground/55',
	plum: 'border-user-plum-foreground/55',
	sage: 'border-user-sage-foreground/55',
	olive: 'border-user-olive-foreground/55',
	teal: 'border-user-teal-foreground/55',
	slate: 'border-user-slate-foreground/55'
};

/** The leader's border color; the neutral handle for no one or an unknown color. */
export function userLeaderClass(color: string | null | undefined): string {
	return isUserColor(color) ? USER_LEADERS[color] : 'border-handle';
}

/**
 * The comment composer, focused, in the writer's own paper - the tone their
 * entry will be in once sent. Written out for the same reason.
 */
const USER_COMPOSERS: Record<UserColor, string> = {
	amber: 'has-[textarea:focus-visible]:bg-user-amber',
	clay: 'has-[textarea:focus-visible]:bg-user-clay',
	rose: 'has-[textarea:focus-visible]:bg-user-rose',
	plum: 'has-[textarea:focus-visible]:bg-user-plum',
	sage: 'has-[textarea:focus-visible]:bg-user-sage',
	olive: 'has-[textarea:focus-visible]:bg-user-olive',
	teal: 'has-[textarea:focus-visible]:bg-user-teal',
	slate: 'has-[textarea:focus-visible]:bg-user-slate'
};

/** The composer's focused paper; none (it stays `surface-elevated`) for an unknown color. */
export function userComposerClass(color: string | null | undefined): string {
	return isUserColor(color) ? USER_COMPOSERS[color] : '';
}

/** Neutral, for a color this build does not know. */
const NEUTRAL = 'bg-accent text-accent-foreground';

export function isUserColor(value: unknown): value is UserColor {
	return typeof value === 'string' && (USER_COLORS as readonly string[]).includes(value);
}

/**
 * Classes for a color that came from the API. A newer server could send a
 * ninth color this build has no token for; that renders neutral rather than
 * transparent, which would leave a circle with no background at all.
 */
export function userColorClasses(color: string | null | undefined, strong = false): string {
	if (!isUserColor(color)) return NEUTRAL;
	return (strong ? USER_COLOR_CLASSES_STRONG : USER_COLOR_CLASSES)[color];
}

/**
 * The color the fewest accounts hold, ties broken by palette order - the
 * same rule `leastUsed` in service/internal/user/profile.go applies when a
 * client omits the color on `POST /users`. This is the client's own copy,
 * for the create dialog's preselection alone: the server decides the real
 * default when the field is actually omitted, and this function only has to
 * agree with it closely enough that the preselected swatch does not jump
 * once the request comes back.
 */
export function leastUsedColor(people: Array<{ color: string }>): UserColor {
	const counts = new Map<string, number>();
	for (const p of people) counts.set(p.color, (counts.get(p.color) ?? 0) + 1);
	let best: UserColor = USER_COLORS[0];
	let bestCount = counts.get(best) ?? 0;
	for (const color of USER_COLORS) {
		const count = counts.get(color) ?? 0;
		if (count < bestCount) {
			best = color;
			bestCount = count;
		}
	}
	return best;
}
