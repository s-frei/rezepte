<script lang="ts">
	import Heart from '@lucide/svelte/icons/heart';
	import { toast } from 'svelte-sonner';
	import { setTasty } from '$lib/api/recipes';
	import { m } from '$lib/paraglide/messages';

	let {
		id,
		active = $bindable(false),
		count = $bindable(0),
		readonly = false,
		fluid = false,
		onchange
	}: {
		id: string;
		active?: boolean;
		count?: number;
		/** A pill that shows the count and cannot be pressed: the author's own
		 * recipe, where the service refuses the mark, and the phone card. */
		readonly?: boolean;
		/** A read-only pill that sizes itself to the card it sits on (the
		 * nearest size container): 24px on the narrowest phone card, growing
		 * with the card to the desktop button's 36px. */
		fluid?: boolean;
		/** Called once a change has been saved, with the new state. */
		onchange?: (active: boolean) => void;
	} = $props();

	let busy = $state(false);
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

	// A display grows with its card, a button keeps the size a finger or a
	// pointer needs. The fluid pill reaches 24px at a 150px card and 36px at
	// 220px, linearly in between (24 + (w - 150) * 12/70), so the count never
	// shrinks while the card grows - also not where the phone layout turns
	// into the desktop one. Gap, padding, type and icon scale off the same
	// height. The button is 44px on a phone, where the recipe page is the only
	// place to set the mark, and 36px from md up.
	const pill = $derived(
		fluid
			? '[--pill:clamp(24px,calc(17.143cqw_-_1.714px),36px)] h-(--pill) gap-[calc(var(--pill)*0.16)] px-[calc(var(--pill)*0.32)] text-[length:calc(var(--pill)/12_+_10px)] [&_svg]:size-[calc(var(--pill)*0.56)]'
			: 'h-11 gap-1.5 px-3 text-caption md:h-9 md:px-2.5 [&_svg]:size-5'
	);
</script>

{#if readonly}
	<!-- The fluid count sits on a phone card's photo, where a button
	     look would invite a tap it cannot answer: it reads as a caption on the
	     photo instead - the lightbox's near-black, the same in both themes,
	     translucent over a blurred patch and without the shadow a button
	     floats on. -->
	<span
		title={countLabel}
		class="inline-flex items-center rounded-pill font-semibold {fluid
			? 'bg-lightbox/60 text-lightbox-foreground backdrop-blur-sm'
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
		class="inline-flex items-center rounded-pill bg-surface font-semibold text-text shadow-card transition hover:brightness-95 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary {pill}"
	>
		<Heart class={active ? 'fill-primary text-primary' : 'text-text-muted'} aria-hidden="true" />
		{#if count > 0}
			<span aria-hidden="true">{count}</span>
		{/if}
		<span id={countId} class="sr-only">{countLabel}</span>
	</button>
{/if}
