<script lang="ts">
	import type { ApiToken } from '$lib/api/tokens';
	import LifetimeBar from '$lib/components/ui/LifetimeBar.svelte';
	import PersonMark from '$lib/components/ui/PersonMark.svelte';
	import Stamp from '$lib/components/ui/Stamp.svelte';
	import { m } from '$lib/paraglide/messages';
	import { formatDate } from '$lib/recipe/format';
	import { accessLevels, isExpired } from '$lib/settings/token-access';
	import { userColorEdge } from '$lib/user/color';

	let {
		token,
		showIssuer,
		onrevoke
	}: { token: ApiToken; showIssuer: boolean; onrevoke: (token: ApiToken) => void } = $props();

	const expired = $derived(isExpired(token));

	const levelWords = {
		none: m.tokens_access_none,
		read: m.tokens_access_read,
		write: m.tokens_access_write,
		delete: m.tokens_access_delete
	};

	// One plaque per area, each a ladder of as many steps as the area has -
	// the same ladders the create dialog offers. A new area is a new plaque.
	const plaques = $derived.by(() => {
		const levels = accessLevels(token.scopes);
		return (
			[
				[m.tokens_access_recipes(), levels.recipes, ['none', 'read', 'write', 'delete']],
				[m.tokens_access_users(), levels.users, ['none', 'read', 'write']]
			] as const
		).map(([area, level, steps]) => ({
			area,
			level: levelWords[level](),
			filled: (steps as readonly string[]).indexOf(level),
			steps: steps.length - 1
		}));
	});
</script>

<!--
	A key tag on the household's key board: the left edge is the issuer's
	color, the bar below the name is how much of the token's life has passed,
	and each plaque is one area's access ladder. An expired tag is faded and
	stamped across its spent bar; the stamp is decoration, the same fact is
	spoken by the hidden line next to the bar.
-->
<li
	class="relative grid min-w-0 grid-cols-[minmax(0,1fr)_auto] gap-x-3 rounded-l-md rounded-r-2xl border border-l-[6px] border-border bg-surface-elevated py-4 pr-4 pl-8 md:pr-5 md:pl-10 {userColorEdge(
		token.ownerColor
	)}"
>
	<span
		aria-hidden="true"
		class="absolute top-6 left-2.5 size-3 rounded-full bg-background shadow-inner md:left-3.5"
	></span>

	<!-- On a phone "Revoke" moves below the plaques, so the name keeps the
		 tag's full width instead of breaking inside its words. -->
	<div class="col-span-2 min-w-0 md:col-span-1 {expired ? 'opacity-60' : ''}">
		<h3 class="font-display text-card leading-snug font-medium hyphens-auto">{token.name}</h3>
		<p class="mt-1 flex flex-wrap items-center gap-x-4 gap-y-1">
			<span class="font-mono text-micro text-text-muted">{token.prefix}…</span>
			{#if showIssuer}
				<span class="flex items-center gap-1.5 text-caption font-semibold">
					<PersonMark
						size="xs"
						person={{
							id: token.ownerId,
							displayName: token.ownerDisplayName,
							color: token.ownerColor
						}}
					/>
					{token.ownerDisplayName}
				</span>
			{/if}
		</p>
	</div>
	<button
		type="button"
		aria-label={m.tokens_revoke_aria({ name: token.name })}
		onclick={() => onrevoke(token)}
		class="order-last col-span-2 mt-2 -mr-2 -mb-2 inline-flex h-11 shrink-0 items-center justify-self-end rounded-pill px-3 text-caption font-semibold whitespace-nowrap text-text-muted transition hover:bg-destructive-soft hover:text-destructive focus-visible:text-destructive focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary md:order-none md:col-span-1 md:col-start-2 md:row-start-1 md:-mt-1 md:mb-0 md:h-8"
	>
		{m.tokens_revoke()}
	</button>

	<div class="relative col-span-2 mt-3">
		<LifetimeBar item={token} faded={expired} />
		<p class="sr-only">
			{m.tokens_issued_on({ date: formatDate(token.createdAt) })}.
			{#if token.expiresAt === undefined}
				{m.tokens_no_expiry()}.
			{:else if expired}
				{m.tokens_expired_on({ date: formatDate(token.expiresAt) })}.
			{:else}
				{m.tokens_valid_to({ date: formatDate(token.expiresAt) })}.
			{/if}
		</p>
		<p
			title={m.tokens_last_used_hint()}
			class="mt-2 flex items-center gap-1.5 text-micro text-text-muted {expired
				? 'opacity-60'
				: ''}"
		>
			<span aria-hidden="true" class="size-1.5 rounded-full bg-primary"></span>
			{token.lastUsedAt === undefined
				? m.tokens_never_used()
				: m.tokens_last_used({ date: formatDate(token.lastUsedAt) })}
		</p>
		{#if expired && token.expiresAt !== undefined}
			<Stamp class="absolute -top-3.5 right-0 z-10 md:right-1.5">
				{m.tokens_expired()}
				<span class="block font-sans text-micro font-semibold">
					{formatDate(token.expiresAt)}
				</span>
			</Stamp>
		{/if}
	</div>

	<!--
		Each plaque keeps the same three tracks whatever it holds - the area
		in the flexible rest, then a ladder track as wide as three steps and a
		level track as wide as the longest word - so ladders and words line up
		down the list and nothing moves while it scrolls. On a wide screen the
		plaques sit side by side and wrap as areas are added.
	-->
	<ul class="col-span-2 mt-4 grid gap-2 md:flex md:flex-wrap {expired ? 'opacity-60' : ''}">
		{#each plaques as plaque (plaque.area)}
			<li
				class="grid grid-cols-[minmax(0,1fr)_2.6em_6em] items-center gap-x-2.5 rounded-pill border border-border bg-surface px-3 py-1.5 text-caption md:inline-flex md:gap-2"
			>
				<span class="hyphens-auto text-text-muted">{plaque.area}</span>
				<span aria-hidden="true" class="flex gap-0.5">
					{#each Array.from({ length: plaque.steps }, (_, i) => i) as step (step)}
						<span
							class="size-2.5 rounded-full border-[1.5px] {step < plaque.filled
								? 'border-primary bg-primary'
								: 'border-handle'}"
						></span>
					{/each}
				</span>
				<span>{plaque.level}</span>
			</li>
		{/each}
	</ul>
</li>
