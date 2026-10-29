<script lang="ts">
	import { resolve } from '$app/paths';
	import { m } from '#lib/paraglide/messages.js';
</script>

<svelte:head><title>{m.app_name()}</title></svelte:head>

<!-- The error page carries no bottom nav (`navPlace` in `#lib/shell.svelte`),
     so it offers its own way back. -->
<main class="mx-auto max-w-3xl p-8">
	<h1 class="font-display text-display-lg font-semibold">{m.app_name()}</h1>
	<p class="mt-2 text-body text-text-muted">{m.not_found()}</p>
	<a
		href={resolve('/')}
		class="mt-6 inline-block text-body-sm font-medium text-primary underline-offset-4 transition hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
		>{m.error_back_to_recipes()}</a
	>
</main>
