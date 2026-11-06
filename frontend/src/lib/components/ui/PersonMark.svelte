<script lang="ts">
	import { avatarUrl } from '#lib/user/avatar.js';
	import { userColorClasses, userRingClass } from '#lib/user/color.js';

	type Size =
		| 'xs'
		| 'xs-plus'
		| 'corner'
		| 'sm'
		| 'md'
		| 'lg'
		| 'ribbon'
		| 'xl'
		| 'band'
		| 'card'
		| 'profile';

	let {
		person,
		size,
		ringed = false,
		inverse = false,
		strong = false,
		src: srcOverride,
		class: extra = ''
	}: {
		person: { id: string; displayName: string; color: string; avatarId?: string | null };
		size: Size;
		/** Ringed whatever it shows, the initial too: the top bar's own
		 * circle, which says whose app this is. */
		ringed?: boolean;
		/** On the `inverse` ground (the author popover): a photo's ring takes
		 * the pair's base tone, the strong one would vanish there. */
		inverse?: boolean;
		/** The pair turned over, the strong tone as ground: a mark on a
		 * ribbon in the person's own color. */
		strong?: boolean;
		/** The picture's address when it is not the session's `/avatars/...`:
		 * the public page's share-scoped one. */
		src?: string | null;
		/** Placement only: a ring, an overlap margin. */
		class?: string;
	} = $props();

	// Letter size per circle size. `xs` to `sm` use text-micro, the rest
	// inherit or step up to the heading size, as the circles did before.
	// The two inline marks, which sit in a line of text, are sized in `em`
	// of their own letter (20 and 24px at the 12px micro size): with text
	// scaled up (Android's font size, `text-size-adjust`) the letter grows,
	// and a px circle left it spilling out, a dot beside a doubled caption.
	const SIZES: Record<Size, string> = {
		xs: 'size-[calc(20em/12)] text-micro',
		'xs-plus': 'size-[2em] text-micro',
		// A recipe card's author circles: sized by the card's corner, not by
		// a line of text, so they stay px - 24px, 28px on the desktop card.
		corner: 'size-6 text-micro desk:size-7',
		sm: 'size-7 text-micro',
		md: 'size-8',
		lg: 'size-9',
		// The public page's "Shared by" ribbon: 40px on a 32px ribbon, so it
		// stands out over both of its edges.
		ribbon: 'size-10 text-body-lg',
		xl: 'size-14 text-heading',
		band: 'size-[min(3.25rem,64px)] text-heading md:size-[min(3.75rem,72px)]',
		card: 'size-16 text-heading',
		profile: 'size-26 text-display-md md:size-28'
	};

	// A photo from `md` up is ringed in the person's color, apart from the
	// photo by a gap: an outline, so the gap is the ground itself, whatever
	// it is, and no parent's background has to be named. `profile` and `band`
	// sit on a band with a 4px cut-out of their own (the caller's `ring-4`),
	// so their outline starts past it. Smaller marks stay plain: at 28px a
	// ring ate the face.
	const RINGS: Partial<Record<Size, string>> = {
		md: 'outline-2 outline-offset-2',
		lg: 'outline-2 outline-offset-2',
		ribbon: 'outline-2 outline-offset-2',
		xl: 'outline-2 outline-offset-2',
		card: 'outline-2 outline-offset-2',
		band: 'outline-2 outline-offset-4',
		profile: 'outline-[2.5px] outline-offset-4'
	};

	const initial = $derived(person.displayName.charAt(0).toUpperCase());
	const colors = $derived(userColorClasses(person.color, strong));

	const src = $derived(srcOverride !== undefined ? srcOverride : avatarUrl(person));
	// The address whose file failed to load. Keyed by address rather than a
	// boolean, so a new picture gets its own chance without an effect
	// resetting it.
	let failed = $state<string | null>(null);
	const showImage = $derived(src !== null && failed !== src);
	// `ringed` draws the top bar's finer line, a hairline on a 1x screen.
	const geometry = $derived(
		ringed ? 'outline-[1.5px] outline-offset-2' : showImage ? RINGS[size] : undefined
	);
	const ring = $derived(
		geometry ? `${geometry} outline-solid ${userRingClass(person.color, inverse)}` : ''
	);
</script>

<!--
	`relative` is what makes an overlap read as one circle in front of
	another: unpositioned boxes paint every background first and every letter
	afterwards, so a later circle covered the earlier circle but not its
	letter. Positioned elements are painted whole, in document order.
-->
<span
	aria-hidden="true"
	class="relative flex shrink-0 items-center justify-center overflow-hidden rounded-full initial-centered font-display font-semibold {SIZES[
		size
	]} {colors} {ring} {extra}"
>
	<!--
		At 20 px the letter needs a nudge: `items-center` centers the line box,
		not the glyph, and a capital has no descender. For Literata at 12px
		(inkAscent 8.41, fontDescent 4, fontAscent 14, measured through canvas
		`measureText`) the nudge (inkAscent + fontDescent - fontAscent) / 2 is
		-0.79px, rounded to 0.8 - in `em`, so it scales with the letter.
	-->
	{#if showImage}
		<!-- Decorative: the name is always beside the mark or in its trigger's label. -->
		<!-- Absolute: the circle's bottom padding (the letter's centering) would stop an in-flow image short. -->
		<img
			{src}
			alt=""
			loading="lazy"
			decoding="async"
			class="absolute inset-0 size-full object-cover"
			onerror={() => (failed = src)}
		/>
	{:else}
		<span class={size === 'xs' ? '-translate-y-[calc(0.8em/12)]' : ''}>{initial}</span>
	{/if}
</span>
