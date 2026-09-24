<script lang="ts">
	import ArrowUpRight from '@lucide/svelte/icons/arrow-up-right';
	import Copy from '@lucide/svelte/icons/copy';
	import { toast } from 'svelte-sonner';
	import { MCP_GUIDE_URL } from '$lib/docs';
	import { m } from '$lib/paraglide/messages';
	import { mcpUrl } from '$lib/settings/mcp';

	// Read off the browser for the same reason ApiReferenceCard does: the
	// instance is wherever the reverse proxy in front of it put it.
	const url = $derived(typeof window === 'undefined' ? '/mcp' : mcpUrl(window.location.origin));

	async function copyUrl() {
		await navigator.clipboard.writeText(url);
		toast.success(m.mcp_url_copied());
	}
</script>

<section class="rounded-2xl bg-surface p-6 md:p-7" aria-labelledby="settings-mcp">
	<h2 id="settings-mcp" class="font-display text-heading font-medium">{m.mcp_section_title()}</h2>
	<p class="mt-1 max-w-[60ch] text-caption text-text-muted">{m.mcp_section_intro()}</p>
	<div
		class="mt-5 flex items-center gap-2 rounded-lg border border-border bg-surface-elevated py-2 pr-2 pl-4"
	>
		<p class="min-w-0 flex-1 truncate font-mono text-body-sm" title={url}>{url}</p>
		<button
			type="button"
			aria-label={m.mcp_url_copy()}
			onclick={() => void copyUrl()}
			class="inline-flex size-9 shrink-0 items-center justify-center rounded-pill text-text-muted transition hover:bg-surface hover:text-text focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
		>
			<Copy aria-hidden="true" class="size-4" />
		</button>
	</div>
	<!-- Served by the Go binary, not the SPA router, so this is a plain anchor
		 that opens in its own tab, like ApiReferenceCard's two links. -->
	<!-- eslint-disable svelte/no-navigation-without-resolve -->
	<div class="mt-6 flex justify-end">
		<a
			href={MCP_GUIDE_URL}
			target="_blank"
			rel="noreferrer"
			class="inline-flex h-10 items-center justify-center gap-2 rounded-pill bg-primary px-5 text-body-sm font-semibold text-primary-foreground transition hover:brightness-95 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary active:scale-[.98]"
		>
			{m.mcp_guide_link()}
			<ArrowUpRight aria-hidden="true" class="size-4" />
		</a>
	</div>
	<!-- eslint-enable svelte/no-navigation-without-resolve -->
</section>
