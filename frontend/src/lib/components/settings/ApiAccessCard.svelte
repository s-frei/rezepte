<script lang="ts">
	import { onMount } from 'svelte';
	import ArrowUpRight from '@lucide/svelte/icons/arrow-up-right';
	import { toast } from 'svelte-sonner';
	import { fetchOperationCount } from '$lib/api/spec';
	import CopyIcon from '$lib/components/icons/CopyIcon.svelte';
	import Skeleton from '$lib/components/ui/Skeleton.svelte';
	import { MCP_GUIDE_URL } from '$lib/docs';
	import { m } from '$lib/paraglide/messages';
	import { mcpUrl } from '$lib/settings/mcp';
	import type { TokenSupply } from '$lib/settings/token-access';

	let {
		isAdmin,
		loading = false,
		supply
	}: {
		isAdmin: boolean;
		/** True while the admin's token list is still on its way. */
		loading?: boolean;
		/** Undefined for a member, while loading, and when the list failed to load. */
		supply?: TokenSupply;
	} = $props();

	// Both addresses live under the origin that served this page, whatever the
	// reverse proxy in front of it is called, so they are read off the browser
	// rather than carried as configured values.
	const origin = typeof window === 'undefined' ? '' : window.location.origin;
	const apiUrl = `${origin}/api/v1`;
	const mcpAddress = mcpUrl(origin);

	// Decoration on a step that reads fine without it, so a failed fetch drops
	// the figure silently instead of turning the card into an error.
	let operations = $state<number | undefined>(undefined);
	onMount(() => {
		fetchOperationCount().then(
			(count) => (operations = count),
			() => {}
		);
	});

	const tokenDone = $derived(isAdmin && supply?.kind === 'valid');
	const tokenWanted = $derived(
		isAdmin && !loading && (supply === undefined || supply.kind !== 'valid')
	);

	async function copy(text: string, confirmation: string) {
		await navigator.clipboard.writeText(text);
		toast.success(confirmation);
		copyIcons[text]?.play();
	}

	// One icon per address, keyed by it, so each answers its own copy.
	let copyIcons = $state<Record<string, CopyIcon>>({});
</script>

{#snippet address(label: string, url: string, copyLabel: string, copied: string)}
	<li class="border-b border-border py-2.5 last:border-b-0">
		<div class="flex items-center justify-between gap-2">
			<span>{label}</span>
			<button
				type="button"
				aria-label={copyLabel}
				onclick={() => void copy(url, copied)}
				class="inline-flex h-8 shrink-0 items-center gap-1.5 rounded-pill border border-border bg-surface-elevated px-3 text-micro font-semibold text-text-muted transition hover:text-text focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
			>
				<CopyIcon bind:this={copyIcons[url]} class="size-3.5" />
				{m.access_copy()}
			</button>
		</div>
		<p class="mt-1.5 font-mono text-caption [overflow-wrap:anywhere]">
			<!-- A phone has room for about 35 monospace characters: a URL breaks
				 after a slash, never inside a segment, unless one alone is wider. -->
			{#each url.split(/(?<=\/)/) as segment, i (i)}{segment}<wbr />{/each}
		</p>
	</li>
{/snippet}

{#snippet token(highlight: boolean, divided: boolean)}
	<!-- The same token in both recipes, so the same status; only where it is
		 the one way in does a missing token stand out. -->
	<li
		class="flex items-baseline justify-between gap-2 py-2.5 {divided
			? 'border-b border-border'
			: ''}"
	>
		<span class={highlight ? 'font-semibold text-primary' : ''}>{m.access_token_one()}</span>
		<div class="text-micro {highlight ? 'font-semibold text-primary' : 'text-text-muted'}">
			{#if !isAdmin}
				{m.access_token_from_admin()}
			{:else if loading}
				<Skeleton class="h-3 w-14 rounded-sm" />
			{:else if supply?.kind === 'missing'}
				{m.access_token_missing()}
			{:else if supply?.kind === 'expired'}
				{m.access_token_expired()}
			{:else if supply?.kind === 'valid'}
				{m.access_token_valid({ count: supply.count })}
			{/if}
		</div>
	</li>
{/snippet}

{#snippet step(n: number, title: string, done: boolean)}
	<span
		aria-hidden="true"
		class="min-w-5 font-display text-heading-lg leading-none {done
			? 'text-handle'
			: 'text-primary'}">{n}</span
	>
	<b class="font-semibold {done ? 'text-text-muted' : ''}">{title}</b>
{/snippet}

<!-- Served by the Go binary, not by the SPA router, so the links below are
	 plain anchors that open in their own tab; `resolve()` maps SvelteKit
	 routes, and none of these paths is one. -->
<!-- eslint-disable svelte/no-navigation-without-resolve -->
{#snippet external(href: string, label: string)}
	<a
		{href}
		target="_blank"
		rel="noreferrer"
		class="inline-flex items-center gap-0.5 font-semibold text-primary underline-offset-4 hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
	>
		{label}<ArrowUpRight aria-hidden="true" class="size-3.5" />
	</a>
{/snippet}
<!-- eslint-enable svelte/no-navigation-without-resolve -->

<!-- A cookbook page with two recipes: each way in has its own ingredients and
	 its own method, side by side from `lg` and one after the other below it:
	 next to the settings sidebar a tablet leaves each column too narrow for
	 its headings and addresses. -->
<section class="rounded-2xl bg-surface p-6 md:p-8" aria-labelledby="settings-access">
	<h2 id="settings-access" class="font-display text-heading font-medium">{m.access_title()}</h2>
	<p class="mt-1 max-w-[60ch] text-caption text-text-muted">{m.access_intro()}</p>

	<div class="mt-6 grid gap-6 lg:grid-cols-2 lg:gap-0">
		<section
			aria-labelledby="access-mcp"
			class="border-t border-border pt-6 lg:border-t-0 lg:border-r lg:pt-0 lg:pr-7"
		>
			<h3 id="access-mcp" class="font-display text-card font-medium">{m.access_mcp_title()}</h3>
			<!-- Both recipes list their ingredients in one order - the address,
				 then what signs in - so the rows line up side by side. -->
			<ul class="mt-3 text-body-sm">
				{@render address(m.access_mcp_address(), mcpAddress, m.mcp_url_copy(), m.mcp_url_copied())}
				{@render token(tokenWanted, false)}
			</ul>
			<ol class="mt-5 space-y-4 text-body-sm">
				<li class="grid grid-cols-[auto_1fr] items-baseline gap-x-3">
					{#if isAdmin}
						{@render step(1, m.access_step_create_title(), tokenDone)}
						<p class="col-start-2 mt-0.5 text-text-muted">
							{#if tokenDone}
								{m.access_step_create_done()}
							{:else if supply?.kind === 'expired'}
								{m.access_step_create_expired()}
							{:else}
								{m.access_step_create_text()}
							{/if}
						</p>
					{:else}
						{@render step(1, m.access_step_get_title(), false)}
						<p class="col-start-2 mt-0.5 text-text-muted">{m.access_step_get_text()}</p>
					{/if}
				</li>
				<li class="grid grid-cols-[auto_1fr] items-baseline gap-x-3">
					{@render step(2, m.access_step_enter_title(), false)}
					<p class="col-start-2 mt-0.5 text-text-muted">
						{m.access_step_enter_text()}
						{@render external(MCP_GUIDE_URL, m.mcp_guide_link())}
					</p>
				</li>
			</ol>
		</section>

		<section
			aria-labelledby="access-api"
			class="border-t border-border pt-6 lg:border-t-0 lg:pt-0 lg:pl-7"
		>
			<h3 id="access-api" class="font-display text-card font-medium">{m.access_api_title()}</h3>
			<ul class="mt-3 text-body-sm">
				{@render address(
					m.access_api_address(),
					apiUrl,
					m.api_base_url_copy(),
					m.api_base_url_copied()
				)}
				{@render token(false, true)}
				<li class="py-1 text-caption text-text-muted italic">{m.access_or()}</li>
				<li class="py-2.5">{m.access_credentials()}</li>
			</ul>
			<ol class="mt-5 space-y-4 text-body-sm">
				<li class="grid grid-cols-[auto_1fr] items-baseline gap-x-3">
					{@render step(1, m.access_step_sign_in_title(), false)}
					<p class="col-start-2 mt-0.5 text-text-muted">{m.access_step_sign_in_text()}</p>
				</li>
				<li class="grid grid-cols-[auto_1fr] items-baseline gap-x-3">
					{@render step(2, m.access_step_start_title(), false)}
					<p class="col-start-2 mt-0.5 text-text-muted">
						{operations === undefined
							? m.access_step_start_text()
							: m.access_step_start_count({ count: operations })}
					</p>
					<p class="col-start-2 mt-1 flex flex-wrap gap-x-4">
						{@render external('/api/v1/docs', m.api_docs_link())}
						{@render external('/api/v1/openapi.json', m.api_spec_link())}
					</p>
				</li>
			</ol>
		</section>
	</div>
</section>
