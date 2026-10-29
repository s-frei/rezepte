<script lang="ts">
	import StarIcon from '$lib/components/icons/StarIcon.svelte';
	import { toast } from 'svelte-sonner';
	import { setFavorite } from '$lib/api/recipes';
	import { m } from '$lib/paraglide/messages';

	let {
		id,
		active = $bindable(false),
		card = false,
		photo = false
	}: {
		id: string;
		active?: boolean;
		/** The small disc in a card's top-right corner: 28px with a 16px star,
		 * so it shares the corner's center (see RecipeCard). */
		card?: boolean;
		/** On a phone card's photo: 24px with a 14px star, for the photo's
		 * 18px corner, and in the lightbox's translucent near-black. */
		photo?: boolean;
	} = $props();

	let busy = $state(false);
	let star = $state<StarIcon>();

	// Optimistic with rollback: the star must feel instant, and a failed
	// request must not leave a lie on screen. `preventDefault`/`stopPropagation`
	// matter wherever this sits inside (or, on the card, visually over) a link -
	// without them a click here would also navigate.
	async function toggle(event: MouseEvent) {
		event.preventDefault();
		event.stopPropagation();
		if (busy) {
			return;
		}
		const previous = active;
		active = !active;
		if (active) {
			star?.play();
		}
		busy = true;
		try {
			await setFavorite(id, active);
		} catch {
			active = previous;
			toast.error(m.recipe_favorite_error());
		} finally {
			busy = false;
		}
	}
</script>

<!-- 44px on a phone, the floor for an action there; 36px from md up. On a
     card the disc is smaller and its `::after` grows the target to 44px on
     a phone. On a photo the phone disc reads like a caption: the lightbox's
     near-black, the same in both themes, translucent over a blurred patch,
     and a set star in the lighter orange, which keeps 3:1 against it where
     the light theme's primary does not. A toggle: the name stays,
     `aria-pressed` carries the state. -->
<button
	type="button"
	onclick={toggle}
	aria-pressed={active}
	aria-label={m.recipe_favorite()}
	class="relative inline-flex items-center justify-center rounded-pill hover-tint transition focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary {!card
		? 'size-11 bg-surface shadow-card md:size-9'
		: photo
			? 'size-6 bg-lightbox/60 backdrop-blur-sm after:absolute after:-inset-2.5 md:size-7 md:bg-surface md:shadow-card md:backdrop-blur-none md:after:hidden'
			: 'size-7 bg-surface shadow-card after:absolute after:-inset-2 md:after:hidden'}"
>
	<StarIcon
		bind:this={star}
		class="{!card ? 'size-5' : photo ? 'size-3.5 md:size-4' : 'size-4'} {!photo
			? active
				? 'fill-primary text-primary'
				: 'text-text-muted'
			: active
				? 'fill-lightbox-primary text-lightbox-primary md:fill-primary md:text-primary'
				: 'text-lightbox-foreground md:text-text-muted'}"
	/>
</button>
