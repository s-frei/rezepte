<script lang="ts">
	import type { ApiToken } from '$lib/api/tokens';
	import { m } from '$lib/paraglide/messages';
	import TokenRow from './TokenRow.svelte';

	let { tokens, onrevoke }: { tokens: ApiToken[]; onrevoke: (token: ApiToken) => void } = $props();
</script>

<!-- A CSS grid, not a table: the same UserTable/UserRow pattern as the users
	 list. Rows stack on a phone instead of forcing a wide, horizontally
	 scrolling table - see UserRow.svelte for the column-collapsing technique. -->
<div
	aria-hidden="true"
	class="hidden grid-cols-[1.4fr_1fr_90px_160px_110px] gap-3 border-b border-border pb-2 text-micro font-semibold tracking-[0.08em] text-text-muted uppercase md:grid"
>
	<span>{m.tokens_col_name()}</span>
	<span>{m.tokens_col_scopes()}</span>
	<span>{m.tokens_col_owner()}</span>
	<span>{m.tokens_col_activity()}</span>
	<span class="text-right">{m.tokens_col_actions()}</span>
</div>
<ul aria-label={m.settings_nav_api()}>
	{#each tokens as token (token.id)}
		<TokenRow {token} {onrevoke} />
	{/each}
</ul>
