<script lang="ts">
	import { Dialog } from 'bits-ui';
	import Copy from '@lucide/svelte/icons/copy';
	import CircleCheck from '@lucide/svelte/icons/circle-check';
	import Mail from '@lucide/svelte/icons/mail';
	import { toast } from 'svelte-sonner';
	import { setupLinkUrl, type SetupLinkInfo } from '$lib/api/users';
	import BaseDialog from '$lib/components/ui/BaseDialog.svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import { m } from '$lib/paraglide/messages';
	import { formatDate } from '$lib/recipe/format';

	let {
		open = $bindable(false),
		link,
		displayName
	}: {
		open?: boolean;
		link: SetupLinkInfo | null;
		displayName: string;
	} = $props();

	// mailedTo names the address on a failed send too; mailError tells them apart.
	const sent = $derived(!!link?.mailedTo && !link.mailError);

	const url = $derived(link ? setupLinkUrl(link) : '');
	// The people row's formatter, so the date reads the same in both places.
	const expiry = $derived(link ? formatDate(link.expiresAt) : '');

	// navigator.clipboard is absent on plain http (a LAN instance with no
	// TLS) and writeText can also reject (permission denied); either way the
	// text is still right there, selected, to copy by hand.
	async function copy() {
		try {
			if (!navigator.clipboard) throw new Error('clipboard API unavailable');
			await navigator.clipboard.writeText(url);
			toast.success(m.setup_link_copied());
		} catch {
			toast.error(m.setup_link_copy_error());
		}
	}
</script>

<!-- Follows TokenRevealDialog's shape: a title, a hint, a read-only value
     block with its own copy button. Unlike that dialog the link is not shown
     once only - it stays valid until used, revoked or replaced - so this
     stays dismissible (Escape, outside click), where the token reveal is not. -->
<BaseDialog bind:open>
	<Dialog.Title class="font-display text-heading font-medium">
		{m.setup_link_dialog_title({ name: displayName })}
	</Dialog.Title>
	<!-- Mailed, the link below is only the fallback, so the success block
	     takes the hint's place and names the expiry itself. -->
	{#if sent}
		<div
			role="status"
			class="mt-4 flex items-start gap-3 rounded-lg bg-success-soft px-4 py-3.5 text-success-foreground"
		>
			<CircleCheck aria-hidden="true" class="mt-0.5 size-5 shrink-0" />
			<div class="min-w-0">
				<Dialog.Description class="text-body font-semibold break-words">
					{m.setup_link_sent({ address: link?.mailedTo ?? '' })}
				</Dialog.Description>
				<p class="text-caption">{m.setup_link_sent_detail({ date: expiry })}</p>
			</div>
		</div>
		<p class="mt-4 text-caption text-text-muted">{m.setup_link_sent_fallback()}</p>
	{:else if link?.mailError}
		<!-- The failure block says to pass the link on; the hint would say it twice. -->
		<div
			role="alert"
			class="mt-4 flex items-start gap-3 rounded-lg bg-destructive-soft px-4 py-3.5 text-destructive"
		>
			<Mail aria-hidden="true" class="mt-0.5 size-5 shrink-0" />
			<div class="min-w-0">
				<Dialog.Description class="text-body font-semibold break-words">
					{m.setup_link_send_failed({ address: link.mailedTo ?? '' })}
				</Dialog.Description>
				<p class="text-caption">{m.setup_link_send_failed_detail()}</p>
			</div>
		</div>
	{:else}
		<Dialog.Description class="mt-2 text-body text-text-muted">
			{m.setup_link_dialog_hint()}
		</Dialog.Description>
	{/if}
	<div class="{sent ? 'mt-2' : 'mt-5'} flex items-start gap-2 rounded-md bg-surface-elevated p-3">
		<!-- A textarea, not a single-line input: the URL is longer than the
		     dialog is wide, and wrapping it beats a field that scrolls its
		     content out of view - the same reason TokenRevealDialog's token
		     sits in a wrapped <code> block. readonly rather than disabled, so
		     it still gets an accessible name and can be selected by hand. -->
		<textarea
			readonly
			rows="4"
			value={url}
			aria-label={m.setup_link_dialog_title({ name: displayName })}
			class="[field-sizing:content] min-w-0 flex-1 resize-none bg-transparent font-mono text-body-sm break-all outline-none"
			onclick={(event) => event.currentTarget.select()}></textarea>
		<Button variant="ghost" onclick={() => void copy()}>
			<Copy aria-hidden="true" class="size-4" />
			{m.tokens_reveal_copy()}
		</Button>
	</div>
	{#if !sent}
		<p class="mt-3 text-caption text-text-muted">
			{m.setup_link_dialog_expiry({ date: expiry })}
		</p>
	{/if}
</BaseDialog>
