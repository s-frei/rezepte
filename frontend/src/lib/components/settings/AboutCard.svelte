<script lang="ts">
	import { onMount } from 'svelte';
	import ArrowUpRight from '@lucide/svelte/icons/arrow-up-right';
	import { fetchVersion } from '#lib/api/spec.js';
	import { CHANGELOG_URL, SOURCE_URL } from '#lib/docs.js';
	import { m } from '#lib/paraglide/messages.js';

	// Decoration on a card that stands without it, like the API card's
	// version: a failed fetch leaves the line out.
	let version = $state<string | undefined>(undefined);

	onMount(async () => {
		version = await fetchVersion().catch(() => undefined);
	});

	const link =
		'inline-flex min-h-10 max-w-full min-w-0 flex-auto items-center justify-center gap-2 rounded-[1.25rem] border border-border bg-surface-elevated px-4 py-2 text-center text-body-sm font-semibold wrap-break-word hyphens-auto transition hover:bg-background focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary';
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
	<p class="mt-1 max-w-[60ch] text-caption wrap-break-word hyphens-auto text-text-muted">
		{m.settings_about_text()}
	</p>
	<!-- eslint-disable svelte/no-navigation-without-resolve -->
	<!-- The links share the line and grow to fill it; with 200% text each
	     takes a line of its own and a long word hyphenates inside it. The
	     radius is half the 40px height, so one line is a pill and a wrapped
	     label a rounded box rather than a blob. -->
	<div class="mt-5 flex flex-wrap gap-3 sm:[&>a]:flex-none">
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
