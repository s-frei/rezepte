<script lang="ts">
	import CircleCheck from '@lucide/svelte/icons/circle-check';
	import { onMount } from 'svelte';
	import { resolve } from '$app/paths';
	import { confirmEmail, me } from '$lib/api/auth';
	import { ApiError } from '$lib/api/client';
	import { session } from '$lib/auth.svelte';
	import AuthScene from '$lib/components/auth/AuthScene.svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import { m } from '$lib/paraglide/messages';

	let phase = $state<'loading' | 'done' | 'invalid' | 'error'>('loading');
	let address = $state('');
	const MARK = '\u0000';
	// Neutral, since whoever is signed in may not own the address.
	const doneText = m.confirm_done_text({ address: MARK }).split(MARK);
	const home = $derived(session.user ? resolve('/') : resolve('/login'));

	// The token is read from the fragment and the address bar cleared at
	// once, like /welcome: it never reaches a server log or a Referer header.
	onMount(async () => {
		const token = window.location.hash.slice(1);
		window.history.replaceState(null, '', '/confirm-email');
		if (!token) {
			phase = 'invalid';
			return;
		}
		try {
			address = (await confirmEmail(token)).address;
			phase = 'done';
			// A signed-in profile shows the confirmed line right away.
			if (session.user) session.user = await me().catch(() => session.user);
		} catch (e) {
			phase = e instanceof ApiError && e.status === 404 ? 'invalid' : 'error';
		}
	});
</script>

<!-- Named only once it is true, like /welcome's title. -->
<svelte:head
	><title>{phase === 'done' ? `${m.confirm_done()} · ${m.app_name()}` : m.app_name()}</title
	></svelte:head
>

<AuthScene lead={phase === 'done' ? m.confirm_lead() : ''}>
	{#if phase === 'loading'}
		<p role="status" class="text-center text-body text-text-muted lg:text-left">
			{m.confirm_loading()}
		</p>
	{:else if phase === 'done'}
		<div role="status">
			<h1
				class="flex items-center gap-2.5 rounded-md bg-success-soft px-4 py-3 text-body font-semibold text-success-foreground"
			>
				<CircleCheck class="size-5 shrink-0" aria-hidden="true" />
				{m.confirm_done()}
			</h1>
		</div>
		<p class="text-body text-text-muted">
			{doneText[0]}<strong class="font-semibold text-text">{address}</strong>{doneText[1]}
		</p>
		<Button size="lg" class="w-full" href={home}>{m.confirm_continue()}</Button>
	{:else if phase === 'invalid'}
		<div class="space-y-2 text-center lg:text-left">
			<h1 class="font-display text-heading font-medium">{m.welcome_invalid()}</h1>
			<p class="text-body text-text-muted">
				{session.user ? m.confirm_invalid_hint() : m.confirm_invalid_hint_signed_out()}
			</p>
		</div>
		<!-- A new link is sent from the profile, so that is where this leads. -->
		{#if session.user}
			<Button size="lg" class="w-full" href={resolve('/settings')}>{m.confirm_to_profile()}</Button>
		{:else}
			<Button size="lg" class="w-full" href={resolve('/login')}>{m.login_submit()}</Button>
		{/if}
	{:else if phase === 'error'}
		<div role="alert" class="space-y-2 text-center lg:text-left">
			<h1 class="font-display text-heading font-medium">{m.confirm_error_title()}</h1>
			<p class="text-body text-text-muted">{m.confirm_error()}</p>
		</div>
		<Button size="lg" class="w-full" href={home}>{m.confirm_continue()}</Button>
	{/if}
</AuthScene>
