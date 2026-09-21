<script lang="ts">
	import type { ApiToken } from '$lib/api/tokens';
	import { m } from '$lib/paraglide/messages';

	let { token, onrevoke }: { token: ApiToken; onrevoke: (token: ApiToken) => void } = $props();

	const expired = $derived(
		token.expiresAt !== undefined && new Date(token.expiresAt) <= new Date()
	);

	function date(value: string | undefined, fallback: string): string {
		return value === undefined ? fallback : new Date(value).toLocaleDateString('de-DE');
	}
</script>

<!--
	Same collapsing technique as UserRow.svelte: on a phone the row is a single
	stacked column (name on its own line, everything else below it), and
	`md:contents` on the second wrapper dissolves it so scopes/owner/activity/
	actions become the grid's own columns 2-5. Scopes, owner and activity carry
	an inline label on mobile (`md:hidden`) since there's no header row there;
	activity keeps its two labels at every width because "Aktivität" alone
	doesn't say which line is which.
-->
<li
	class="grid grid-cols-[1fr_auto] items-start gap-3 border-b border-dashed border-border py-4 last:border-b-0 md:grid-cols-[1.4fr_1fr_90px_160px_110px]"
>
	<div class="col-span-2 min-w-0 md:col-span-1">
		<div class="flex flex-wrap items-center gap-2">
			<span
				title={token.name}
				class="truncate text-body-sm font-semibold {expired ? 'text-text-muted' : ''}"
			>
				{token.name}
			</span>
			{#if expired}
				<span class="rounded-pill bg-surface-elevated px-2 py-0.5 text-micro">
					{m.tokens_expired()}
				</span>
			{/if}
		</div>
		<p class="mt-0.5 truncate font-mono text-micro text-text-muted">
			{m.tokens_col_prefix()}: {token.prefix}…
		</p>
	</div>

	<div class="col-span-2 mt-3 flex flex-col gap-2 md:mt-0 md:contents">
		<div class="min-w-0 text-caption">
			<span class="text-micro font-semibold text-text-muted md:hidden">
				{m.tokens_col_scopes()}:
			</span>
			{token.scopes.join(', ')}
		</div>
		<div class="min-w-0 text-caption">
			<span class="text-micro font-semibold text-text-muted md:hidden">
				{m.tokens_col_owner()}:
			</span>
			{token.ownerName}
		</div>
		<div class="min-w-0 text-caption">
			<p title={m.tokens_last_used_hint()}>
				<span class="text-micro text-text-muted">{m.tokens_col_last_used()}:</span>
				{date(token.lastUsedAt, m.tokens_last_used_never())}
			</p>
			<p>
				<span class="text-micro text-text-muted">{m.tokens_col_expires()}:</span>
				{date(token.expiresAt, m.tokens_expires_never())}
			</p>
		</div>
		<div class="flex justify-end">
			<button
				type="button"
				aria-label={m.tokens_revoke_aria({ name: token.name })}
				onclick={() => onrevoke(token)}
				class="inline-flex h-11 items-center rounded-pill px-3 text-caption font-semibold whitespace-nowrap text-destructive transition hover:bg-destructive-soft focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary md:h-8"
			>
				{m.tokens_revoke()}
			</button>
		</div>
	</div>
</li>
