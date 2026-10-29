<script lang="ts">
	import type { Component } from 'svelte';
	import type { LucideProps } from '@lucide/svelte';
	import Bell from '@lucide/svelte/icons/bell';
	import BellOff from '@lucide/svelte/icons/bell-off';
	import BellRing from '@lucide/svelte/icons/bell-ring';
	import Mail from '@lucide/svelte/icons/mail';
	import MailCheck from '@lucide/svelte/icons/mail-check';
	import MailWarning from '@lucide/svelte/icons/mail-warning';
	import MailX from '@lucide/svelte/icons/mail-x';
	import Send from '@lucide/svelte/icons/send';
	import { resolve } from '$app/paths';
	import { toast } from 'svelte-sonner';
	import { ApiError, isSignedOut } from '$lib/api/client';
	import {
		KIND_GROUP,
		setNotificationPrefs,
		type Channel,
		type Group,
		type Kind,
		type Prefs
	} from '$lib/api/notifications';
	import { session } from '$lib/auth.svelte';
	import { deviceState, enableDevice, sendPushTest } from '$lib/push.svelte';
	import StatusBox from '$lib/components/settings/StatusBox.svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import SegmentedControl from '$lib/components/ui/SegmentedControl.svelte';
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
	const isOwner = $derived(session.user?.role === 'superadmin');
	const device = $derived(deviceState());
	let testSent = $state(false);
	let testing = $state(false);

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
	const kindText: Record<Kind, { name: () => string; hint: () => string }> = {
		comment: {
			name: m.settings_notifications_kind_comment,
			hint: m.settings_notifications_kind_comment_hint
		},
		recipe: {
			name: m.settings_notifications_kind_recipe,
			hint: m.settings_notifications_kind_recipe_hint
		},
		tasty: {
			name: m.settings_notifications_kind_tasty,
			hint: m.settings_notifications_kind_tasty_hint
		},
		invite_accepted: {
			name: m.settings_notifications_kind_invite_accepted,
			hint: m.settings_notifications_kind_invite_accepted_hint
		},
		mail_failing: {
			name: m.settings_notifications_kind_mail_failing,
			hint: m.settings_notifications_kind_mail_failing_hint
		}
	};

	type Icons = { on: Component<LucideProps>; off: Component<LucideProps> };
	const channels: { channel: Channel; title: () => string; lead: () => string; icons: Icons }[] = [
		{
			channel: 'mail',
			title: m.settings_notifications_mail_title,
			lead: m.settings_notifications_mail_lead,
			icons: { on: Mail, off: MailX }
		},
		{
			channel: 'push',
			title: m.settings_notifications_push_title,
			lead: m.settings_notifications_push_lead,
			icons: { on: Bell, off: BellOff }
		}
	];

	// Greyed but operable: the preferences are the account's, so a later
	// confirmation or another device makes them count.
	const muted: Record<Channel, boolean> = $derived({
		mail: !mailEnabled || !verified,
		push: device !== 'enabled'
	});

	// Per channel, one block per group the account has a kind of there, in the server's order.
	function groupsOf(channel: Channel) {
		return (['household', 'admin'] as Group[])
			.map((group) => ({
				group,
				kinds: (Object.keys(KIND_GROUP) as Kind[]).filter(
					(kind) => KIND_GROUP[kind] === group && prefs?.kinds[kind]?.[channel] !== undefined
				)
			}))
			.filter(({ kinds }) => kinds.length > 0);
	}

	// Saves on flip; the controls show the server's answer, so a failed write snaps back.
	// A group's switch is its own: its kinds keep their choices.
	// Only the latest flip's answer lands: an older reply, or an older failure's
	// snap back, would undo a newer choice still on its way.
	let latest = 0;
	async function change(patch: Prefs) {
		if (!prefs) return;
		const request = ++latest;
		const before = prefs;
		prefs = {
			kinds: merge(prefs.kinds, patch.kinds),
			groups: merge(prefs.groups, patch.groups)
		};
		try {
			const saved = await setNotificationPrefs(patch);
			if (request === latest) prefs = saved;
		} catch (error) {
			if (request === latest) prefs = before;
			if (!isSignedOut(error)) toast.error(m.settings_notifications_save_error());
		}
	}

	function merge<K extends string>(
		into: Partial<Record<K, Partial<Record<Channel, boolean>>>>,
		patch: Partial<Record<K, Partial<Record<Channel, boolean>>>>
	) {
		const out = { ...into };
		for (const key of Object.keys(patch) as K[]) out[key] = { ...into[key], ...patch[key] };
		return out;
	}

	// No await before enableDevice: the permission prompt needs the click.
	function enable() {
		enableDevice().catch(() => {
			// Declined: the box now says it is blocked.
			if (deviceState() !== 'blocked') toast.error(m.settings_notifications_enable_error());
		});
	}

	async function sendTest() {
		// Cleared first, so a repeat's "Sent" is new text and is announced again.
		testSent = false;
		testing = true;
		try {
			await sendPushTest();
			testSent = true;
		} catch (error) {
			if (isSignedOut(error)) return;
			toast.error(
				error instanceof ApiError && error.status === 429
					? m.settings_notifications_test_wait()
					: m.settings_notifications_test_error()
			);
		} finally {
			testing = false;
		}
	}

	const blockClass = 'rounded-lg border border-border bg-surface-elevated';
</script>

{#snippet text(name: string, hint: string, strong: boolean)}
	<div class="min-w-0">
		<p class="text-body {strong ? 'font-semibold' : ''}">{name}</p>
		<p class="mt-0.5 text-caption text-text-muted">{hint}</p>
	</div>
{/snippet}

{#snippet mailBox()}
	{#if !mailEnabled}
		<StatusBox
			icon={MailX}
			tone="warn"
			title={address || m.settings_notifications_address_none()}
			hint={m.settings_notifications_mail_off()}
			action={isOwner ? setUpMail : undefined}
		/>
	{:else if !address}
		<StatusBox
			icon={MailWarning}
			tone="warn"
			title={m.settings_notifications_address_none()}
			hint={m.settings_notifications_address_none_hint()}
		>
			{#snippet action()}
				<Button size="sm" href={resolve('/settings')}>
					<Mail class="size-4" aria-hidden="true" />{m.settings_notifications_add_in_profile()}
				</Button>
			{/snippet}
		</StatusBox>
	{:else if !verified}
		<StatusBox
			icon={MailWarning}
			tone="warn"
			title={address}
			hint={m.settings_notifications_address_unconfirmed()}
		>
			{#snippet action()}
				<Button size="sm" href={resolve('/settings')}>
					<Mail class="size-4" aria-hidden="true" />{m.settings_notifications_confirm_in_profile()}
				</Button>
			{/snippet}
		</StatusBox>
	{:else}
		<StatusBox
			icon={MailCheck}
			tone="good"
			title={address}
			hint={m.settings_notifications_address_confirmed()}
		>
			{#snippet action()}
				<Button size="sm" variant="secondary" href={resolve('/settings')}>
					{m.settings_notifications_change_in_profile()}
				</Button>
			{/snippet}
		</StatusBox>
	{/if}
{/snippet}

{#snippet setUpMail()}
	<Button size="sm" variant="secondary" href={resolve('/settings/mail')}>
		{m.settings_notifications_set_up_mail()}
	</Button>
{/snippet}

{#snippet deviceBox()}
	{#if device === 'enabled'}
		<StatusBox
			icon={BellRing}
			tone="good"
			title={m.settings_notifications_device()}
			hint={m.settings_notifications_device_enabled()}
			note={testSent ? m.settings_notifications_test_sent() : undefined}
		>
			{#snippet action()}
				<Button size="sm" variant="secondary" disabled={testing} onclick={sendTest}>
					<Send class="size-4" aria-hidden="true" />{m.settings_notifications_send_test()}
				</Button>
			{/snippet}
		</StatusBox>
	{:else if device === 'off'}
		<StatusBox
			icon={BellOff}
			tone="neutral"
			title={m.settings_notifications_device()}
			hint={m.settings_notifications_device_off()}
		>
			{#snippet action()}
				<Button size="sm" onclick={enable}>
					<Bell class="size-4" aria-hidden="true" />{m.settings_notifications_enable_device()}
				</Button>
			{/snippet}
		</StatusBox>
	{:else}
		<StatusBox
			icon={BellOff}
			tone="warn"
			title={m.settings_notifications_device()}
			hint={device === 'blocked'
				? m.settings_notifications_device_blocked()
				: m.settings_notifications_device_unsupported()}
		/>
	{/if}
{/snippet}

<section class="rounded-2xl bg-surface p-6 md:p-7" aria-labelledby="settings-notifications">
	<h2 id="settings-notifications" class="mb-4 font-display text-heading font-medium">
		{m.settings_nav_notifications()}
	</h2>
	{#if prefs === null}
		<p class="mb-4 text-body-sm text-text-muted">{m.settings_notifications_load_error()}</p>
	{/if}
	{#each channels as { channel, title, lead, icons }, c (channel)}
		<!-- Mail and push each have a toggle per kind: the name says which. -->
		{@const named = (name: string) =>
			m.settings_notifications_control_label({ name, channel: title() })}
		<section aria-labelledby="settings-notifications-{channel}" class={c ? 'mt-7' : ''}>
			<h3 id="settings-notifications-{channel}" class="font-display text-card font-medium">
				{title()}
			</h3>
			<p class="mt-1 max-w-[60ch] text-body-sm text-text-muted">{lead()}</p>
			<div class="mt-4 space-y-3">
				{@render (channel === 'mail' ? mailBox : deviceBox)()}
				{#if prefs}
					<div class="space-y-3 {muted[channel] ? 'opacity-50' : ''}">
						{#each groupsOf(channel) as { group, kinds } (group)}
							{@const groupOn = prefs.groups[group]?.[channel] ?? true}
							<!-- The group's switch heads the block and its kinds sit inset
							     below it, so it reads as the switch for all of them. Off, it
							     holds them: greyed and inert but still focusable, each keeping
							     its own choice. -->
							<div class={blockClass}>
								<div class="flex items-start justify-between gap-4 px-4 py-3">
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
										checked={groupOn}
										label={named(groupText[group].name())}
										onchange={(on) =>
											void change({ kinds: {}, groups: { [group]: { [channel]: on } } })}
									/>
								</div>
								<div
									class="mx-3 mb-3 rounded-md border border-border bg-surface {groupOn
										? ''
										: 'opacity-50'}"
								>
									{#each kinds as kind, i (kind)}
										<div
											class="flex flex-col gap-2.5 px-4 py-3 md:flex-row md:items-center md:justify-between md:gap-4 {i
												? 'border-t border-dashed border-border'
												: ''}"
										>
											{@render text(kindText[kind].name(), kindText[kind].hint(), false)}
											<!-- Off left and On right, like a switch's track; each side
											     carries the channel's icon, since a row of switches would
											     not say which channel it is about. -->
											<SegmentedControl
												value={(prefs.kinds[kind]?.[channel] ?? true) ? 'on' : 'off'}
												options={[
													{ value: 'off', label: m.settings_notifications_off(), icon: icons.off },
													{ value: 'on', label: m.settings_notifications_on(), icon: icons.on }
												]}
												label={named(kindText[kind].name())}
												disabled={!groupOn}
												onchange={(v) =>
													void change({ kinds: { [kind]: { [channel]: v === 'on' } }, groups: {} })}
											/>
										</div>
									{/each}
								</div>
							</div>
						{/each}
					</div>
					{#if channel === 'mail'}
						<div class="{blockClass} flex items-start justify-between gap-4 px-4 py-3">
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
					{/if}
				{/if}
			</div>
		</section>
	{/each}
</section>
