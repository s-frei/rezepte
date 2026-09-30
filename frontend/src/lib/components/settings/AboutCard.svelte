<script lang="ts">
	import { onMount } from 'svelte';
	import ArrowUpRight from '@lucide/svelte/icons/arrow-up-right';
	import { fetchVersion } from '$lib/api/spec';
	import { CHANGELOG_URL, SOURCE_URL } from '$lib/docs';
	import { m } from '$lib/paraglide/messages';

	// Decoration on a card that stands without it, like the API card's
	// version: a failed fetch leaves the line out.
	let version = $state<string | undefined>(undefined);

	onMount(async () => {
		version = await fetchVersion().catch(() => undefined);
	});

	const link =
		'inline-flex h-10 items-center justify-center gap-2 rounded-pill border border-border bg-surface-elevated px-4 text-body-sm font-semibold transition hover:bg-background focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary';
</script>

<!--
	The book's colophon: which Rezepte this is and where it comes from. Both
	targets are outside the SPA router, so `resolve()` does not apply and the
	navigation rule is lifted for these anchors only.
-->
<section class="rounded-2xl bg-surface p-6 md:p-7" aria-labelledby="settings-about">
	<div class="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1">
		<h2 id="settings-about" class="font-display text-heading font-medium">
			{m.settings_about_title()}
		</h2>
		{#if version !== undefined}
			<p class="text-caption text-text-muted">
				{m.settings_about_version({ version })}
			</p>
		{/if}
	</div>
	<p class="mt-1 max-w-[60ch] text-caption text-text-muted">{m.settings_about_text()}</p>
	<!-- eslint-disable svelte/no-navigation-without-resolve -->
	<div class="mt-5 flex flex-wrap gap-3">
		<a href={SOURCE_URL} target="_blank" rel="noopener external" class={link}>
			{m.settings_about_source()}
			<ArrowUpRight aria-hidden="true" class="size-4 text-text-muted" />
		</a>
		<a href={CHANGELOG_URL} target="_blank" rel="noopener external" class={link}>
			{m.settings_about_changelog()}
			<ArrowUpRight aria-hidden="true" class="size-4 text-text-muted" />
		</a>
	</div>
	<!-- eslint-enable svelte/no-navigation-without-resolve -->
</section>
