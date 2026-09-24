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
export function userColorClasses(color: string | null | undefined): string {
	return isUserColor(color) ? USER_COLOR_CLASSES[color] : NEUTRAL;
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
export function leastUsedColor(usage: Array<{ color: UserColor; count: number }>): UserColor {
	const counts = new Map(usage.map((entry) => [entry.color, entry.count]));
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
