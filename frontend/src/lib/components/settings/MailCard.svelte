<script lang="ts">
	import { onMount } from 'svelte';
	import { toast } from 'svelte-sonner';
	import { isSignedOut } from '$lib/api/client';
	import { clearMailSettings, getMailSettings, type MailSettings } from '$lib/api/mail';
	import Button from '$lib/components/ui/Button.svelte';
	import ConfirmDialog from '$lib/components/ui/ConfirmDialog.svelte';
	import Skeleton from '$lib/components/ui/Skeleton.svelte';
	import { m } from '$lib/paraglide/messages';
	import MailSettingsDialog from './MailSettingsDialog.svelte';
	import TestMailRow, { type TestMailResult } from './TestMailRow.svelte';

	let current = $state<MailSettings | null>(null);
	let loadFailed = $state(false);
	let editOpen = $state(false);
	let turnOffOpen = $state(false);
	let result = $state<TestMailResult | null>(null);

	onMount(async () => {
		try {
			current = await getMailSettings();
		} catch (error) {
			loadFailed = !isSignedOut(error);
		}
	});

	// A letterhead: the same three lines whether set up or not, so the card
	// keeps its shape when mail is turned on and only the lines fill in. Each
	// value is a main part and a smaller line under it.
	const blank = $derived(!current || current.source === 'none');
	const lines = $derived<{ key: string; main: string; sub: string }[]>(
		current && !blank
			? [
					{
						key: m.settings_mail_line_sender(),
						main: current.fromName || current.from,
						sub: current.fromName ? current.from : ''
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
						sub: current.passwordSet ? m.settings_mail_line_password_set() : ''
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

	// One layout for every row: if any value does not fit beside its key and
	// leader, all of them move under their leaders. Measured in the one-line
	// layout each time, so widening the card brings it back.
	function fitRows(dl: HTMLElement) {
		void lines; // measure again when the values change
		const measure = () => {
			dl.removeAttribute('data-stacked');
			const overflows = [...dl.children].some((row) => row.scrollWidth > row.clientWidth);
			dl.toggleAttribute('data-stacked', overflows);
		};
		measure();
		const observer = new ResizeObserver(measure);
		observer.observe(dl);
		return () => observer.disconnect();
	}

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
	<h2 id="settings-mail" class="mb-4 font-display text-heading font-medium">
		{m.settings_nav_mail()}
	</h2>
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
			<div class="relative mt-6 rounded-lg border border-border bg-surface-elevated px-5 py-4">
				<span
					class="absolute -top-2.5 right-4 inline-flex items-center gap-1.5 rounded-pill border border-border bg-surface px-2.5 py-0.5 text-label font-bold text-text-muted uppercase"
				>
					{#if current.source === 'settings'}
						<span aria-hidden="true" class="size-1.5 rounded-full bg-success"></span>
						<span class="text-text">{m.settings_mail_stamp_on()}</span>
					{:else}
						{current.source === 'env' ? m.settings_mail_stamp_env() : m.settings_mail_stamp_off()}
					{/if}
				</span>
				<dl {@attach fitRows} class="group">
					{#each lines as { key, main, sub } (key)}
						<div class="flex items-baseline gap-x-2.5 py-1 group-data-stacked:flex-wrap">
							<dt
								class="shrink-0 font-display text-body-lg whitespace-nowrap italic {blank
									? 'text-text-muted'
									: 'text-primary'}"
							>
								{key}
							</dt>
							<span
								aria-hidden="true"
								class="min-w-6 flex-1 -translate-y-1 border-b-2 border-dotted border-border"
							></span>
							<dd
								class="shrink-0 text-right text-body whitespace-nowrap group-data-stacked:basis-full group-data-stacked:wrap-anywhere group-data-stacked:whitespace-normal {blank
									? 'text-text-muted'
									: ''}"
							>
								<span class="block">{main}</span>
								{#if sub}
									<span class="block text-caption text-text-muted">{sub}</span>
								{/if}
							</dd>
						</div>
					{/each}
				</dl>
			</div>
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
