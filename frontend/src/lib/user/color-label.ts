import { m } from '#lib/paraglide/messages.js';
import { isUserColor, type UserColor } from './color.js';

// Paraglide compiles one function per key, so the color names cannot be
// looked up as `m['user_color_' + color]` - the compiler has to see every
// call. Kept apart from #lib/user/color.ts, the class table, which holds no
// copy.
const COLOR_LABELS: Record<UserColor, () => string> = {
	amber: m.user_color_amber,
	clay: m.user_color_clay,
	rose: m.user_color_rose,
	plum: m.user_color_plum,
	sage: m.user_color_sage,
	olive: m.user_color_olive,
	teal: m.user_color_teal,
	slate: m.user_color_slate
};

/** A palette color's name in the current language; empty for a color this build does not know. */
export function userColorLabel(color: string): string {
	return isUserColor(color) ? COLOR_LABELS[color]() : '';
}
