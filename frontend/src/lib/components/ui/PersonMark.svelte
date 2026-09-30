<script lang="ts">
	import { avatarUrl } from '$lib/user/avatar';
	import { userColorClasses } from '$lib/user/color';

	type Size = 'xs' | 'sm' | 'md' | 'lg' | 'xl' | 'card' | 'profile';

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
		card: 'size-16 text-heading',
		profile: 'size-24 text-display-sm'
	};

	const initial = $derived(person.displayName.charAt(0).toUpperCase());
	const colors = $derived(
		neutral ? 'bg-accent text-accent-foreground' : userColorClasses(person.color)
	);

	const src = $derived(avatarUrl(person));
	// The id whose file failed to load. Keyed by id rather than a boolean,
	// so a new picture gets its own chance without an effect resetting it.
	let failed = $state<string | null>(null);
	const showImage = $derived(src !== null && failed !== person.avatarId);
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
	{#if showImage}
		<!-- Decorative: the name is always beside the mark or in its trigger's label. -->
		<!-- Absolute: the circle's bottom padding (the letter's centering) would stop an in-flow image short. -->
		<img
			{src}
			alt=""
			loading="lazy"
			decoding="async"
			class="absolute inset-0 size-full object-cover"
			onerror={() => (failed = person.avatarId ?? null)}
		/>
	{:else}
		<span class={size === 'xs' ? '-translate-y-[0.8px]' : ''}>{initial}</span>
	{/if}
</span>
