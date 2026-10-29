<script lang="ts">
	import Bell from '@lucide/svelte/icons/bell';
	import { m } from '$lib/paraglide/messages';
	import type { Timer } from '$lib/recipe/timers';
	import { timers } from '$lib/recipe/timers.svelte';

	let { timer, showRecipe }: { timer: Timer; showRecipe: boolean } = $props();

	const name = $derived(m.timer_name({ number: timer.step + 1, phrase: timer.phrase }));
</script>

<!-- `role="alert"`: a banner that appears is announced once, which is the
     end of the timer for a screen reader; the countdown itself never is. -->
<div
	role="alert"
	class="flex items-center gap-3 rounded-2xl bg-primary py-2 pr-2 pl-3.5 text-primary-foreground"
>
	<Bell class="size-6 shrink-0" aria-hidden="true" />
	<div class="min-w-0 flex-1 leading-tight">
		<p class="text-card font-semibold">{m.timer_done()}</p>
		<p data-timer-name class="truncate text-caption font-medium">{name}</p>
		<!-- Another recipe's title on a line of its own, so it never cuts the name. -->
		{#if showRecipe}
			<p class="truncate text-micro font-medium opacity-90">{timer.recipeTitle}</p>
		{/if}
	</div>
	<button
		type="button"
		onclick={() => timers.stop(timer.id)}
		class="h-11 shrink-0 rounded-pill bg-primary-foreground hover-tint px-5 text-body font-semibold text-primary transition focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary-foreground active:scale-[.98]"
	>
		{m.timer_off()}
	</button>
</div>
