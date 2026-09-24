<script lang="ts">
	import Star from '@lucide/svelte/icons/star';
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

<button
	type="button"
	onclick={toggle}
	aria-pressed={active}
	aria-label={active ? m.recipe_favorite_remove() : m.recipe_favorite_add()}
	class="inline-flex size-9 items-center justify-center rounded-pill bg-surface shadow-card transition hover:brightness-95 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
>
	<Star
		class="size-5 {active ? 'fill-primary text-primary' : 'text-text-muted'}"
		aria-hidden="true"
	/>
</button>
