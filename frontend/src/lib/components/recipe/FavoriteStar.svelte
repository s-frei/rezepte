<script lang="ts">
	import StarIcon from '$lib/components/icons/StarIcon.svelte';
	import { toast } from 'svelte-sonner';
	import { setFavorite } from '$lib/api/recipes';
	import { m } from '$lib/paraglide/messages';

	let {
		id,
		active = $bindable(false)
	}: {
		id: string;
		active?: boolean;
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

<!-- 44px on a phone, the floor for an action there, since the recipe page
     is where a phone sets the star; 36px from md up, where it also sits over
     every card's photo. A toggle: the name stays, `aria-pressed` carries the
     state. -->
<button
	type="button"
	onclick={toggle}
	aria-pressed={active}
	aria-label={m.recipe_favorite()}
	class="inline-flex size-11 items-center justify-center rounded-pill bg-surface shadow-card transition hover:brightness-95 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary md:size-9"
>
	<StarIcon
		bind:this={star}
		class="size-5 {active ? 'fill-primary text-primary' : 'text-text-muted'}"
	/>
</button>
