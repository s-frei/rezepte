<script lang="ts">
	import { onMount } from 'svelte';
	import ArrowUpRight from 'lucide-svelte/icons/arrow-up-right';
	import Copy from 'lucide-svelte/icons/copy';
	import { toast } from 'svelte-sonner';
	import { fetchApiSummary, fetchVersion } from '$lib/api/spec';
	import Skeleton from '$lib/components/ui/Skeleton.svelte';
	import { m } from '$lib/paraglide/messages';
	import type { ApiSummary } from '$lib/settings/api-summary';

	let { showTokenHint = false }: { showTokenHint?: boolean } = $props();

	let summary = $state<ApiSummary | undefined>(undefined);
	let version = $state<string | undefined>(undefined);
	let loading = $state(true);

	// The API lives under the origin that served this page, whatever the
	// reverse proxy in front of it is called, so the card reads it off the
	// browser rather than carrying a configured value.
	const baseUrl = $derived(
		typeof window === 'undefined' ? '/api/v1' : `${window.location.origin}/api/v1`
	);

	// Both figures are decoration on a card that stands without them: the base
	// URL and the two links are the payload. A failed fetch therefore drops its
	// own figure silently - no toast, no error state - which is also why this
	// settles both instead of awaiting them in sequence.
	async function load() {
		const [spec, healthz] = await Promise.allSettled([fetchApiSummary(), fetchVersion()]);
		if (spec.status === 'fulfilled') {
			summary = spec.value;
		}
		if (healthz.status === 'fulfilled') {
			version = healthz.value;
		}
		loading = false;
	}

	onMount(load);

	async function copyBaseUrl() {
		await navigator.clipboard.writeText(baseUrl);
		toast.success(m.api_base_url_copied());
	}
</script>

<section class="rounded-2xl bg-surface p-6 md:p-7" aria-labelledby="settings-api">
	<h2 id="settings-api" class="font-display text-heading font-medium">
		{m.api_section_title()}
	</h2>
	<p class="mt-1 max-w-[60ch] text-caption text-text-muted">{m.api_section_intro()}</p>

	<!-- The one framed surface on the card: this is the string somebody copies
		 into a client configuration, and everything else here is context. -->
	<div
		class="mt-5 flex items-center gap-2 rounded-lg border border-border bg-surface-elevated py-2 pr-2 pl-4"
	>
		<p class="min-w-0 flex-1 truncate font-mono text-body-sm" title={baseUrl}>{baseUrl}</p>
		<button
			type="button"
			aria-label={m.api_base_url_copy()}
			onclick={() => void copyBaseUrl()}
			class="inline-flex size-9 shrink-0 items-center justify-center rounded-pill text-text-muted transition hover:bg-surface hover:text-text focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
		>
			<Copy aria-hidden="true" class="size-4" />
		</button>
	</div>

	<!-- The version belongs to the instance behind that URL, not to the spec,
		 so it stays with the URL instead of joining the counted figures - and
		 it reads as the identifier it is, not as a third number. -->
	{#if version !== undefined}
		<p class="mt-2 pl-4 text-caption text-text-muted">
			{m.api_stat_version()}
			<span class="font-mono">{version}</span>
		</p>
	{/if}

	{#if loading}
		<Skeleton class="mt-5 h-10 w-44 rounded-sm" />
	{:else if summary !== undefined}
		<!-- Held apart by whitespace alone: two plain facts, not two cards.
			 `dt` precedes its `dd` as a description list requires, and
			 `flex-col-reverse` is what puts the value above its label. -->
		<dl class="mt-5 flex flex-wrap gap-x-10 gap-y-4">
			<div class="flex flex-col-reverse">
				<dt class="text-caption text-text-muted">{m.api_stat_operations()}</dt>
				<dd class="font-display text-display-sm font-medium">{summary.operations}</dd>
			</div>
			<div class="flex flex-col-reverse">
				<dt class="text-caption text-text-muted">{m.api_stat_areas()}</dt>
				<dd class="font-display text-display-sm font-medium">{summary.areas}</dd>
			</div>
		</dl>
	{/if}

	<!-- Both targets are served by the Go binary, not by the SPA router, so
		 they are plain anchors that open in their own tab. `resolve()` would be
		 wrong here - it maps SvelteKit routes, and neither path is one - which
		 is why the rule guarding SPA links is lifted for these two only. -->
	<!-- eslint-disable svelte/no-navigation-without-resolve -->
	<div class="mt-6 flex flex-wrap items-center justify-end gap-3">
		<a
			href="/api/v1/openapi.json"
			target="_blank"
			rel="noreferrer"
			class="inline-flex h-10 items-center justify-center rounded-pill px-3 font-mono text-body-sm text-text-muted underline decoration-border underline-offset-4 transition hover:text-text hover:decoration-text-muted focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
		>
			{m.api_spec_link()}
		</a>
		<a
			href="/api/v1/docs"
			target="_blank"
			rel="noreferrer"
			class="inline-flex h-10 items-center justify-center gap-2 rounded-pill bg-primary px-5 text-body-sm font-semibold text-primary-foreground transition hover:brightness-95 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary active:scale-[.98]"
		>
			{m.api_docs_link()}
			<ArrowUpRight aria-hidden="true" class="size-4" />
		</a>
	</div>
	<!-- eslint-enable svelte/no-navigation-without-resolve -->

	{#if showTokenHint}
		<p class="mt-5 max-w-[60ch] border-t border-border pt-5 text-caption text-text-muted">
			{m.api_member_hint()}
		</p>
	{/if}
</section>
