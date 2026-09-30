<script lang="ts">
	import { userColorClasses } from '$lib/user/color';

	type Size = 'xs' | 'sm' | 'md' | 'lg' | 'xl' | 'card';

	// eslint-disable-next-line svelte/no-unused-props -- id and avatarId are read once the picture lands
	let {
		person,
		size,
		neutral = false,
		class: extra = ''
	}: {
		person: { id: string; displayName: string; color: string; avatarId?: string | null };
		size: Size;
		/** The accent pair instead of the person's color - the user menu's
		 * trigger, which is chrome rather than a mark of somebody. */
		neutral?: boolean;
		/** Placement only: a ring, an overlap margin. */
		class?: string;
	} = $props();

	// Letter size per circle size. `xs` and `sm` use text-micro, the rest
	// inherit or step up to the heading size, as the circles did before.
	const SIZES: Record<Size, string> = {
		xs: 'size-5 text-micro',
		sm: 'size-7 text-micro',
		md: 'size-8',
		lg: 'size-9',
		xl: 'size-14 text-heading',
		card: 'size-16 text-heading'
	};

	const initial = $derived(person.displayName.charAt(0).toUpperCase());
	const colors = $derived(
		neutral ? 'bg-accent text-accent-foreground' : userColorClasses(person.color)
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
	]} {colors} {extra}"
>
	<!--
		At 20 px the letter needs a nudge: `items-center` centers the line box,
		not the glyph, and a capital has no descender. For Literata at 12px
		(inkAscent 8.41, fontDescent 4, fontAscent 14, measured through canvas
		`measureText`) the nudge (inkAscent + fontDescent - fontAscent) / 2 is
		-0.79px, rounded to 0.8.
	-->
	<span class={size === 'xs' ? '-translate-y-[0.8px]' : ''}>{initial}</span>
</span>
