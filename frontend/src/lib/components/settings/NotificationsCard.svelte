<script lang="ts">
	import { resolve } from '$app/paths';
	import { toast } from 'svelte-sonner';
	import { isSignedOut } from '$lib/api/client';
	import {
		KIND_GROUP,
		setNotificationPrefs,
		type Group,
		type Kind,
		type Prefs
	} from '$lib/api/notifications';
	import { session } from '$lib/auth.svelte';
	import Switch from '$lib/components/ui/Switch.svelte';
	import { m } from '$lib/paraglide/messages';

	let {
		prefs: loaded,
		mailEnabled
	}: {
		/** The kinds and groups the account may have; null when they could not be read. */
		prefs: Prefs | null;
		mailEnabled: boolean;
	} = $props();

	// Written over by every flip, reseeded by a new load.
	let prefs = $derived(loaded);

	const address = $derived(session.user?.email ?? '');
	const verified = $derived(Boolean(address) && Boolean(session.user?.emailVerified));
	const MARK = '\u0000';
	const lead = $derived(m.settings_notifications_lead({ address: MARK }).split(MARK));

	const groupText: Record<Group, { name: () => string; hint: () => string }> = {
		household: {
			name: m.settings_notifications_group_household,
			hint: m.settings_notifications_group_household_hint
		},
		admin: {
			name: m.settings_notifications_group_admin,
			hint: m.settings_notifications_group_admin_hint
		}
	};
	// Only the mail channel for now; a kind without text here is not shown.
	const kindText: Partial<Record<Kind, { name: () => string; hint: () => string }>> = {
		comment: {
			name: m.settings_notifications_kind_comment,
			hint: m.settings_notifications_kind_comment_hint
		},
		recipe: {
			name: m.settings_notifications_kind_recipe,
			hint: m.settings_notifications_kind_recipe_hint
		},
		invite_accepted: {
			name: m.settings_notifications_kind_invite_accepted,
			hint: m.settings_notifications_kind_invite_accepted_hint
		}
	};

	// One block per group the account has a kind of, in the server's order.
	const groups = $derived(
		(['household', 'admin'] as Group[])
			.map((group) => ({
				group,
				kinds: (Object.keys(KIND_GROUP) as Kind[]).flatMap((kind) => {
					const t = kindText[kind];
					return KIND_GROUP[kind] === group && t && prefs?.kinds[kind]?.mail !== undefined
						? [{ kind, t }]
						: [];
				})
			}))
			.filter(({ kinds }) => kinds.length > 0)
	);

	// Saves on flip; the switches show the server's answer, so a failed write snaps back.
	// A group's switch is its own: its kinds keep their choices.
	async function change(patch: Prefs) {
		if (!prefs) return;
		const before = prefs;
		prefs = {
			kinds: { ...prefs.kinds, ...patch.kinds },
			groups: { ...prefs.groups, ...patch.groups }
		};
		try {
			prefs = await setNotificationPrefs(patch);
		} catch (error) {
			prefs = before;
			if (!isSignedOut(error)) toast.error(m.settings_notifications_save_error());
		}
	}

	const linkClass =
		'font-semibold text-primary underline-offset-4 hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary';
	const blockClass = 'rounded-lg border border-border bg-surface-elevated';
	const rowClass = 'flex items-start justify-between gap-4 px-4 py-3';
</script>

{#snippet text(name: string, hint: string, strong: boolean)}
	<div class="min-w-0">
		<p class="text-body {strong ? 'font-semibold' : ''}">{name}</p>
		<p class="mt-0.5 text-caption text-text-muted">{hint}</p>
	</div>
{/snippet}

<section class="rounded-2xl bg-surface p-6 md:p-7" aria-labelledby="settings-notifications">
	<h2 id="settings-notifications" class="mb-4 font-display text-heading font-medium">
		{m.settings_nav_notifications()}
	</h2>
	{#if !verified}
		<p class="max-w-[60ch] text-body-sm text-text-muted">
			{m.settings_notifications_unverified()}
			<a href={resolve('/settings')} class={linkClass}
				>{m.settings_notifications_confirm_address()}</a
			>.
		</p>
	{:else}
		<p class="max-w-[60ch] text-body-sm wrap-anywhere text-text-muted">
			{lead[0]}<strong class="font-semibold text-text">{address}</strong>{lead[1]}
			<a href={resolve('/settings')} class={linkClass}
				>{m.settings_notifications_change_address()}</a
			>.
		</p>
	{/if}
	<!-- Unconfirmed too: confirming takes a mail as well. -->
	{#if !mailEnabled}
		<p class="mt-3 max-w-[60ch] text-body-sm text-text-muted">
			{m.settings_notifications_mail_off()}
		</p>
	{/if}
	{#if verified}
		{#if prefs === null}
			<p class="mt-5 text-body-sm text-text-muted">{m.settings_notifications_load_error()}</p>
		{:else}
			<div class="mt-5 space-y-3">
				{#each groups as { group, kinds } (group)}
					<!-- The group's switch heads the card and its kinds sit inset below
					     it, so it reads as the switch for all of them. -->
					<div class={blockClass}>
						<div class={rowClass}>
							<div class="min-w-0">
								<p class="flex flex-wrap items-center gap-2 text-body font-semibold">
									{groupText[group].name()}
									{#if group === 'admin'}
										<span
											class="rounded-pill bg-accent px-2 text-label font-bold text-accent-foreground uppercase"
											>{m.settings_notifications_admin()}</span
										>
									{/if}
								</p>
								<p class="mt-0.5 text-caption text-text-muted">{groupText[group].hint()}</p>
							</div>
							<Switch
								checked={prefs.groups[group]?.mail ?? false}
								label={groupText[group].name()}
								onchange={(on) => void change({ kinds: {}, groups: { [group]: { mail: on } } })}
							/>
						</div>
						<div class="mx-3 mb-3 rounded-md border border-border bg-surface">
							{#each kinds as { kind, t }, i (kind)}
								<div class="{rowClass} {i ? 'border-t border-dashed border-border' : ''}">
									{@render text(t.name(), t.hint(), false)}
									<Switch
										checked={prefs.kinds[kind]?.mail ?? false}
										label={t.name()}
										onchange={(on) => void change({ kinds: { [kind]: { mail: on } }, groups: {} })}
									/>
								</div>
							{/each}
						</div>
					</div>
				{/each}
				<div class="{blockClass} {rowClass}">
					{@render text(
						m.settings_notifications_security(),
						m.settings_notifications_security_hint(),
						true
					)}
					<span
						class="shrink-0 rounded-pill border border-border bg-surface px-2.5 py-0.5 text-label font-bold text-text-muted uppercase"
						>{m.settings_notifications_always()}</span
					>
				</div>
			</div>
		{/if}
	{/if}
</section>
