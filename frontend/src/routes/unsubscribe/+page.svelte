<script lang="ts">
	import CircleCheck from '@lucide/svelte/icons/circle-check';
	import { resolve } from '$app/paths';
	import { toast } from 'svelte-sonner';
	import { postUnsubscribe, type Kind, type Unsubscribe } from '$lib/api/notifications';
	import AuthScene from '$lib/components/auth/AuthScene.svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import { m } from '$lib/paraglide/messages';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	let info = $derived<Unsubscribe | null>(data.info);
	// What the last click switched off, so Undo turns back on only that;
	// null until then and after Undo.
	let done = $state<{ scope: 'kind' | 'group'; kinds: Kind[] } | null>(null);
	let busy = $state(false);

	// The kinds of the group that are on now.
	const groupOn = $derived(
		info ? (Object.keys(info.groupKinds) as Kind[]).filter((k) => info?.groupKinds[k]) : []
	);

	const MARK = '\u0000';
	const account = $derived(
		info ? m.unsubscribe_for({ username: MARK, email: info.email }).split(MARK) : []
	);

	const kindLabel = {
		comment: m.unsubscribe_kind_comment,
		recipe: m.unsubscribe_kind_recipe,
		invite_accepted: m.unsubscribe_kind_invite_accepted
	};
	const groupLabel = { household: m.unsubscribe_group_household, admin: m.unsubscribe_group_admin };
	const doneText = {
		comment: m.unsubscribe_done_comment,
		recipe: m.unsubscribe_done_recipe,
		invite_accepted: m.unsubscribe_done_invite_accepted,
		household: m.unsubscribe_done_household,
		admin: m.unsubscribe_done_admin
	};

	// Only what is still on is offered; the mail's own link comes first, as
	// the primary button. Nothing left means the link has nothing to switch off.
	const choices = $derived.by(() => {
		if (!info) return [];
		const kind = info.kindSubscribed
			? [{ scope: 'kind' as const, label: kindLabel[info.kind]() }]
			: [];
		const group =
			groupOn.length > 0 ? [{ scope: 'group' as const, label: groupLabel[info.group]() }] : [];
		return data.scope === 'group' ? [...group, ...kind] : [...kind, ...group];
	});

	async function unsubscribe(scope: 'kind' | 'group') {
		// What is on now, for Undo to turn back on and nothing more.
		const kinds = scope === 'group' ? groupOn : info ? [info.kind] : [];
		if (await send({ scope, subscribed: false })) done = { scope, kinds };
	}

	async function undo() {
		if (done && (await send({ ...done, subscribed: true }))) done = null;
	}

	async function send(change: { scope: 'kind' | 'group'; subscribed: boolean; kinds?: Kind[] }) {
		busy = true;
		try {
			info = await postUnsubscribe({ ...data.link, ...change });
			return true;
		} catch {
			toast.error(m.unsubscribe_error());
			return false;
		} finally {
			busy = false;
		}
	}

	const manageClass =
		'self-center text-body-sm font-semibold text-primary underline-offset-4 hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary lg:self-start';
</script>

<svelte:head
	><title
		>{done
			? m.unsubscribe_done()
			: info && choices.length === 0
				? m.unsubscribe_already()
				: m.unsubscribe_title()} · {m.app_name()}</title
	></svelte:head
>

{#snippet manage()}
	<a href={resolve('/settings/notifications')} class={manageClass}>{m.unsubscribe_manage()}</a>
{/snippet}

<AuthScene>
	{#if data.failure === 'gone'}
		<div class="space-y-2 text-center lg:text-left">
			<h1 class="font-display text-heading font-medium">{m.unsubscribe_gone()}</h1>
			<p class="text-body text-text-muted">{m.unsubscribe_gone_hint()}</p>
		</div>
		{@render manage()}
	{:else if data.failure === 'error' || !info}
		<div role="alert" class="space-y-2 text-center lg:text-left">
			<h1 class="font-display text-heading font-medium">{m.unsubscribe_title()}</h1>
			<p class="text-body text-text-muted">{m.unsubscribe_error()}</p>
		</div>
		{@render manage()}
	{:else if done}
		<h1 class="text-center font-display text-heading font-medium lg:text-left">
			{m.unsubscribe_done()}
		</h1>
		<p
			role="status"
			class="flex items-center gap-2.5 rounded-md bg-success-soft px-4 py-3 text-body font-semibold text-success-foreground"
		>
			<CircleCheck class="size-5 shrink-0" aria-hidden="true" />
			{doneText[done.scope === 'kind' ? info.kind : info.group]()}
		</p>
		<Button variant="secondary" size="lg" class="w-full" disabled={busy} onclick={undo}
			>{m.unsubscribe_undo()}</Button
		>
		<p class="text-center text-caption text-text-muted lg:text-left">{m.unsubscribe_done_hint()}</p>
		{@render manage()}
	{:else if choices.length === 0}
		<h1 class="text-center font-display text-heading font-medium lg:text-left">
			{m.unsubscribe_already()}
		</h1>
		<p
			role="status"
			class="flex items-center gap-2.5 rounded-md bg-success-soft px-4 py-3 text-body font-semibold text-success-foreground"
		>
			<CircleCheck class="size-5 shrink-0" aria-hidden="true" />
			{doneText[info.group]()}
		</p>
		<p class="text-center text-caption text-text-muted lg:text-left">{m.unsubscribe_done_hint()}</p>
		{@render manage()}
	{:else}
		<div class="space-y-2 text-center lg:text-left">
			<h1 class="font-display text-heading font-medium">{m.unsubscribe_title()}</h1>
			<p class="text-body wrap-anywhere text-text-muted">
				{account[0]}<strong class="font-semibold text-text">{info.username}</strong>{account[1]}
			</p>
		</div>
		<div class="flex flex-col gap-3">
			{#each choices as { scope, label }, i (scope)}
				<Button
					variant={i === 0 ? 'primary' : 'secondary'}
					size="lg"
					class="h-auto! min-h-12 w-full py-3 text-center text-balance"
					disabled={busy}
					onclick={() => unsubscribe(scope)}>{label}</Button
				>
			{/each}
		</div>
		<p class="text-center text-caption text-text-muted lg:text-left">{m.unsubscribe_security()}</p>
		{@render manage()}
	{/if}
</AuthScene>
