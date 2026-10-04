<script lang="ts">
	import { onMount } from 'svelte';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { forgotPassword, getPasswordReset } from '$lib/api/auth';
	import AuthScene from '$lib/components/auth/AuthScene.svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import Input from '$lib/components/ui/Input.svelte';
	import { m } from '$lib/paraglide/messages';

	// Filled in from /login's navigation state, never from the URL.
	let login = $state(page.state.login ?? '');
	// Whether mail is on; null until known. Reached by URL with mail off,
	// the page says so instead of offering a form that mails nothing. A
	// failed lookup shows the form: the request still answers the same.
	let available = $state<boolean | null>(null);
	// The name the answer is about; null until the form was sent.
	let sentFor = $state<string | null>(null);
	let submitting = $state(false);
	let error = $state<string | null>(null);

	// The login sits mid-sentence in either language, so the sentence is cut
	// at a marker the catalog never contains, and the name goes in bold.
	const MARK = '\u0000';
	const sentText = $derived(m.forgot_sent_text({ login: MARK }).split(MARK));

	onMount(() => {
		getPasswordReset()
			.then((info) => (available = info.available))
			.catch(() => (available = true));
	});

	// The form is gone once it was sent, so focus goes to the answer.
	const focusOnMount = (node: HTMLElement) => node.focus();

	async function submit(event: SubmitEvent) {
		event.preventDefault();
		error = null;
		submitting = true;
		try {
			await forgotPassword(login.trim());
			sentFor = login.trim();
		} catch {
			error = m.forgot_error();
		} finally {
			submitting = false;
		}
	}
</script>

<svelte:head><title>{m.forgot_title()} · {m.app_name()}</title></svelte:head>

<AuthScene lead={m.forgot_lead()}>
	{#if available === false}
		<div class="space-y-2">
			<h1 class="font-display text-heading font-medium">{m.forgot_title()}</h1>
			<p class="text-body text-text-muted">{m.forgot_unavailable()}</p>
		</div>
	{:else if available && sentFor === null}
		<form onsubmit={submit} class="flex flex-col gap-5">
			<div class="space-y-2">
				<h1 class="font-display text-heading font-medium">{m.forgot_title()}</h1>
				<p class="text-body text-text-muted">{m.forgot_intro()}</p>
			</div>
			<Input
				id="forgot-login"
				label={m.forgot_login_label()}
				autocomplete="username"
				maxlength={254}
				required
				bind:value={login}
				oninput={() => (error = null)}
				{error}
			/>
			<Button type="submit" size="lg" class="w-full" disabled={submitting || !login.trim()}>
				{m.forgot_submit()}
			</Button>
		</form>
	{:else if sentFor !== null}
		<div class="space-y-3" role="status">
			<h1
				{@attach focusOnMount}
				tabindex="-1"
				class="font-display text-heading font-medium outline-none"
			>
				{m.forgot_sent_title()}
			</h1>
			<p class="text-body text-text-muted">
				{sentText[0]}<strong class="font-semibold text-text">{sentFor}</strong>{sentText[1]}
			</p>
			<p class="text-caption text-text-muted">{m.forgot_sent_hint()}</p>
		</div>
	{/if}
	{#if available !== null}
		<a
			href={resolve('/login')}
			class="self-center text-caption font-semibold text-text-muted underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
		>
			{m.forgot_back()}
		</a>
	{/if}
</AuthScene>
