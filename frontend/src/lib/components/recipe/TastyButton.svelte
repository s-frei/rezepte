<script lang="ts">
	import Heart from '@lucide/svelte/icons/heart';
	import HeartIcon from '#lib/components/icons/HeartIcon.svelte';
	import { toast } from 'svelte-sonner';
	import { setTasty } from '#lib/api/recipes.js';
	import { m } from '#lib/paraglide/messages.js';

	let {
		id,
		active = $bindable(false),
		count = $bindable(0),
		readonly = false,
		accent = false,
		size,
		onchange
	}: {
		id: string;
		active?: boolean;
		count?: number;
		/** A pill that shows the count and cannot be pressed: the author's own
		 * recipe, where the service refuses the mark, and the phone card. */
		readonly?: boolean;
		/** The read-only count in the sand of the accent rather than on the
		 * light surface: a phone photo card's footer, where a light pill
		 * would vanish into the light card in dark mode. */
		accent?: boolean;
		/** On a card. `compact`: the pill sized to the author initials in a
		 * card's footer: 28px like the phone's 24px circles, 32px like the 28px
		 * ones from md up (each circle carries a 2px ring). `card`: the 28px
		 * button on a card's photo, bottom-left, concentric with the photo's
		 * corner (see RecipeCard). Below `md` - a phone showing one card per
		 * row - its `::after` grows the target to 44px, the phone floor for an
		 * action. */
		size?: 'compact' | 'card';
		/** Called once a change has been saved, with the new state. */
		onchange?: (active: boolean) => void;
	} = $props();

	let busy = $state(false);
	let heart = $state<HeartIcon>();
	// `$props.id()` only runs as its own top-level declaration.
	const uid = $props.id();
	const countId = `${uid}-count`;

	const countLabel = $derived(
		count === 0
			? m.recipe_tasty_none()
			: count === 1
				? m.recipe_tasty_count_one()
				: m.recipe_tasty_count({ count })
	);

	// Optimistic with rollback, like FavoriteStar: the heart must feel
	// instant, and a failed request must not leave a lie on screen. The
	// count moves with it, since it is the one number everyone else sees.
	async function toggle(event: MouseEvent) {
		event.preventDefault();
		event.stopPropagation();
		if (busy) {
			return;
		}
		const previous = active;
		active = !active;
		count += active ? 1 : -1;
		if (active) {
			heart?.play();
		}
		busy = true;
		try {
			await setTasty(id, active);
			onchange?.(active);
		} catch {
			active = previous;
			count += previous ? 1 : -1;
			toast.error(m.recipe_tasty_error());
		} finally {
			busy = false;
		}
	}

	// On a card every pill follows its corner or the initials: the desktop
	// photo's button is 28px (14 + the 6px inset = the photo's 20px corner),
	// the compact pill as tall as the initials with their ring. The button is
	// 44px on the recipe page's phone layout, where it is the only place to
	// set the mark, and 36px there from md up.
	const pill = $derived(
		size === 'compact'
			? 'h-7 gap-1 px-2.5 text-micro desk:h-8 desk:gap-[5px] desk:px-[11px] desk:text-caption [&_svg]:size-4 desk:[&_svg]:size-[18px]'
			: size === 'card'
				? 'h-7 gap-1 px-[9px] text-caption [&_svg]:size-4'
				: 'h-11 gap-1.5 px-3 text-caption desk:h-9 desk:px-2.5 [&_svg]:size-5'
	);
</script>

{#if readonly}
	<!-- The sand pill lies flat in the card, without the shadow a button
	     floats on, so it does not invite a tap it cannot answer. -->
	<span
		title={countLabel}
		class="inline-flex items-center rounded-pill font-semibold {accent
			? 'bg-accent text-accent-foreground'
			: 'bg-surface text-text shadow-card'} {pill}"
	>
		<Heart class="fill-primary text-primary" aria-hidden="true" />
		<span aria-hidden="true">{count}</span>
		<span class="sr-only">{countLabel}</span>
	</span>
{:else}
	<!-- The count sits inside the button rather than beside it, so one tap
	     target carries both and the pill mirrors the star's circle. A toggle:
	     the name stays, `aria-pressed` carries the state. -->
	<button
		type="button"
		onclick={toggle}
		aria-pressed={active}
		aria-label={m.recipe_tasty()}
		aria-describedby={countId}
		title={countLabel}
		class="inline-flex items-center rounded-pill bg-surface hover-tint font-semibold text-text shadow-card transition focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary {size ===
		'card'
			? 'relative after:absolute after:-inset-2 md:after:hidden'
			: ''} {pill}"
	>
		<HeartIcon bind:this={heart} class={active ? 'fill-primary text-primary' : 'text-text-muted'} />
		{#if count > 0}
			<span aria-hidden="true">{count}</span>
		{/if}
		<span id={countId} class="sr-only">{countLabel}</span>
	</button>
{/if}
