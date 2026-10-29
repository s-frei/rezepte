<script lang="ts">
	import { onMount } from 'svelte';
	import { toast } from 'svelte-sonner';
	import { isSignedOut } from '#lib/api/client.js';
	import { clearMailSettings, getMailSettings, type MailSettings } from '#lib/api/mail.js';
	import { getSettings, updateSettings } from '#lib/api/settings.js';
	import Button from '#lib/components/ui/Button.svelte';
	import ConfirmDialog from '#lib/components/ui/ConfirmDialog.svelte';
	import Select from '#lib/components/ui/Select.svelte';
	import Skeleton from '#lib/components/ui/Skeleton.svelte';
	import { m } from '#lib/paraglide/messages.js';
	import { getLocale } from '#lib/paraglide/runtime.js';
	import Letterhead, { type LetterheadLine } from './Letterhead.svelte';
	import MailSettingsDialog from './MailSettingsDialog.svelte';
	import TestMailRow, { type TestMailResult } from './TestMailRow.svelte';

	let current = $state<MailSettings | null>(null);
	let loadFailed = $state(false);
	let editOpen = $state(false);
	let turnOffOpen = $state(false);
	let result = $state<TestMailResult | null>(null);
	// The daily new recipes mail's hour; null until read, and when it cannot
	// be, which hides only its select.
	let digestHour = $state<number | null>(null);

	onMount(async () => {
		void getSettings()
			.then((s) => (digestHour = s.recipeDigestHour))
			.catch(() => null);
		try {
			current = await getMailSettings();
		} catch (error) {
			loadFailed = !isSignedOut(error);
		}
	});

	// Each hour as the locale writes a time: 17:00, 5:00 PM.
	const hourOptions = $derived.by(() => {
		const format = new Intl.DateTimeFormat(getLocale(), { hour: 'numeric', minute: '2-digit' });
		return Array.from({ length: 24 }, (_, hour) => ({
			value: String(hour),
			label: format.format(new Date(2000, 0, 1, hour))
		}));
	});

	// Applies on pick; a failed write snaps back to what was saved.
	async function saveHour(hour: number) {
		const before = digestHour;
		digestHour = hour;
		try {
			digestHour = (await updateSettings({ recipeDigestHour: hour })).recipeDigestHour;
			toast.success(m.settings_mail_digest_saved());
		} catch (error) {
			digestHour = before;
			if (!isSignedOut(error)) toast.error(m.settings_mail_save_error());
		}
	}

	// A letterhead: the same three lines whether set up or not, so the card
	// keeps its shape when mail is turned on and only the lines fill in. Each
	// value is a main part and a smaller line under it.
	const blank = $derived(!current || current.source === 'none');
	const lines = $derived<LetterheadLine[]>(
		current && !blank
			? [
					{
						key: m.settings_mail_line_sender(),
						main: current.fromName || current.from,
						sub: current.fromName ? current.from : '',
						address: true
					},
					{
						key: m.settings_mail_line_via(),
						main: `${current.host}:${current.port}`,
						sub:
							current.security === 'none'
								? m.settings_mail_security_none()
								: current.security.toUpperCase()
					},
					{
						key: m.settings_mail_line_signin(),
						main: current.username || '—',
						sub: current.passwordSet ? m.settings_mail_line_password_set() : '',
						address: true
					}
				]
			: [
					m.settings_mail_line_sender(),
					m.settings_mail_line_via(),
					m.settings_mail_line_signin()
				].map((key) => ({
					key,
					main: '—',
					sub: ''
				}))
	);

	async function turnOff() {
		try {
			await clearMailSettings();
			current = await getMailSettings();
			result = null;
			toast.success(m.settings_mail_turned_off());
		} catch (error) {
			if (!isSignedOut(error)) toast.error(m.settings_mail_save_error());
		}
	}

	function saved(next: MailSettings) {
		current = next;
		result = null;
	}
</script>

<section class="rounded-2xl bg-surface p-6 md:p-7" aria-labelledby="settings-mail">
	<h2 id="settings-mail" class="font-display text-heading font-medium">
		{m.settings_nav_mail()}
	</h2>
	<p class="mt-1 mb-4 max-w-[60ch] text-caption text-text-muted">{m.settings_mail_intro()}</p>
	{#if loadFailed}
		<p class="text-body-sm text-text-muted">{m.settings_mail_load_error()}</p>
	{:else if !current}
		<Skeleton class="mt-6 h-32 rounded-lg" />
	{:else}
		{#if current.publicUrlMissing}
			<p class="max-w-[60ch] text-body-sm text-text-muted">
				{m.settings_mail_public_url_missing()}
			</p>
		{:else}
			<Letterhead
				{lines}
				{blank}
				stamp={{
					label:
						current.source === 'settings'
							? m.settings_mail_stamp_on()
							: current.source === 'env'
								? m.settings_mail_stamp_env()
								: m.settings_mail_stamp_off(),
					on: current.source === 'settings'
				}}
			/>
			{#if current.source === 'env'}
				<p class="mt-3 text-caption text-text-muted">{m.settings_mail_env_hint()}</p>
			{:else if current.source === 'none'}
				<p class="mt-3 text-body-sm text-text-muted">{m.settings_mail_off_hint()}</p>
			{/if}
			{#if current.source === 'none'}
				<div class="mt-4">
					<Button onclick={() => (editOpen = true)}>{m.settings_mail_set_up()}</Button>
				</div>
			{:else if current.source === 'settings'}
				<div class="mt-4 flex flex-wrap items-center justify-between gap-3">
					<Button variant="secondary" onclick={() => (editOpen = true)}
						>{m.settings_mail_edit()}</Button
					>
					<Button variant="ghost" onclick={() => (turnOffOpen = true)}
						>{m.settings_mail_turn_off()}</Button
					>
				</div>
			{/if}
			{#if current.source !== 'none'}
				<div class="mt-5 border-t border-border pt-5"><TestMailRow bind:result /></div>
				{#if digestHour !== null}
					<div class="mt-5 border-t border-border pt-5">
						<p class="mb-1.5 text-caption font-semibold">{m.settings_mail_digest_label()}</p>
						<Select
							value={String(digestHour)}
							options={hourOptions}
							label={m.settings_mail_digest_label()}
							onchange={(next) => void saveHour(Number(next))}
							class="w-36 bg-surface-elevated"
						/>
						<p class="mt-1.5 text-caption text-text-muted">{m.settings_mail_digest_hint()}</p>
					</div>
				{/if}
			{/if}
		{/if}
	{/if}
</section>

{#if current}
	<MailSettingsDialog bind:open={editOpen} {current} onsaved={saved} />
{/if}
<!-- Turning off also drops the stored password, which cannot be read back. -->
<ConfirmDialog
	bind:open={turnOffOpen}
	title={m.settings_mail_turn_off_title()}
	text={m.settings_mail_turn_off_text()}
	confirmLabel={m.settings_mail_turn_off()}
	destructive
	onconfirm={() => void turnOff()}
/>
