<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import type { Pathname } from '$app/types';
	import { Button, Label } from 'bits-ui';
	import { isAppPath, login, safeNext } from '$lib/api/auth';
	import { ApiError } from '$lib/api/client';
	import { getOidc, type OidcInfo } from '$lib/api/oidc';
	import { session } from '$lib/auth.svelte';
	import AuthScene from '$lib/components/auth/AuthScene.svelte';
	import ProviderButton from '$lib/components/auth/ProviderButton.svelte';
	import { m } from '$lib/paraglide/messages';

	let username = $state('');
	let password = $state('');
	let error = $state<string | null>(null);
	let submitting = $state(false);
	let oidc = $state<OidcInfo | null>(null);

	// Where a provider sign-in came back to this page, and why.
	const oidcNotice = $derived.by(() => {
		if (!oidc?.enabled) return null;
		switch (page.url.searchParams.get('oidc')) {
			case 'unlinked':
				return m.oidc_unlinked({ name: oidc.name });
			case 'failed':
				return m.oidc_failed({ name: oidc.name });
		}
		return null;
	});

	// The button is an extra, so a failed lookup simply leaves it out.
	onMount(async () => {
		try {
			oidc = await getOidc();
		} catch {
			oidc = null;
		}
	});

	function loginError(e: unknown): string {
		if (e instanceof ApiError && e.status === 401) return m.login_failed();
		if (e instanceof ApiError && e.status === 429) return m.login_throttled();
		return m.login_error_generic();
	}

	async function submit(event: SubmitEvent) {
		event.preventDefault();
		error = null;
		submitting = true;
		try {
			session.user = await login(username, password);
			const next = safeNext(page.url.searchParams.get('next'));
			// Targets outside the SPA (the Scalar docs page under /api/) are
			// served by the Go binary, so they need a real navigation - goto
			// would look the path up in the client router and 404.
			if (isAppPath(next)) {
				await goto(resolve(next as Pathname));
			} else {
				window.location.assign(next);
			}
		} catch (e) {
			error = loginError(e);
		} finally {
			submitting = false;
		}
	}
</script>

<svelte:head><title>{m.login_title()} · {m.app_name()}</title></svelte:head>

<!-- The card holds two forms side by side rather than one: the provider
     button is a form of its own (a real POST the browser follows to the
     provider), and forms cannot nest. -->
<AuthScene lead={m.login_welcome()}>
	<form onsubmit={submit} class="flex flex-col gap-5">
		<h1 class="sr-only font-display text-heading font-medium lg:not-sr-only">
			{m.login_title()}
		</h1>

		<div class="space-y-1.5">
			<Label.Root for="username" class="text-caption font-semibold">{m.login_username()}</Label.Root
			>
			<!-- The page has one job, so it starts in its first field. It has to
		     be the attribute: SvelteKit moves focus to <body> after every
		     navigation, the redirect here included, unless an element
		     carries `autofocus`. -->
			<!-- svelte-ignore a11y_autofocus -->
			<input
				id="username"
				name="username"
				type="text"
				autofocus
				autocomplete="username"
				autocapitalize="none"
				spellcheck="false"
				required
				bind:value={username}
				class="h-[50px] w-full rounded-md border border-border bg-surface-elevated px-4 text-body outline-none focus:border-primary aria-[invalid=true]:border-[1.5px] aria-[invalid=true]:border-destructive"
				aria-invalid={error !== null}
			/>
		</div>

		<div class="space-y-1.5">
			<Label.Root for="password" class="text-caption font-semibold">{m.login_password()}</Label.Root
			>
			<input
				id="password"
				name="password"
				type="password"
				autocomplete="current-password"
				required
				bind:value={password}
				class="h-[50px] w-full rounded-md border border-border bg-surface-elevated px-4 text-body outline-none focus:border-primary aria-[invalid=true]:border-[1.5px] aria-[invalid=true]:border-destructive"
				aria-invalid={error !== null}
			/>
		</div>

		{#if error}
			<p role="alert" class="text-caption font-medium text-destructive">{error}</p>
		{/if}

		<Button.Root
			type="submit"
			disabled={submitting}
			class="h-[50px] w-full rounded-pill bg-primary text-body font-semibold text-primary-foreground transition hover:brightness-95 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary active:scale-[.98] disabled:opacity-50"
		>
			{submitting ? m.login_submitting() : m.login_submit()}
		</Button.Root>
	</form>

	{#if oidc?.enabled}
		<div class="flex items-center gap-3 text-caption text-text-muted">
			<span class="h-px flex-1 bg-border"></span>
			{m.login_or()}
			<span class="h-px flex-1 bg-border"></span>
		</div>
		{#if oidcNotice}
			<p role="alert" class="text-caption font-medium text-destructive">{oidcNotice}</p>
		{/if}
		<ProviderButton
			intent="login"
			name={oidc.name}
			next={page.url.searchParams.get('next') ?? ''}
		/>
	{/if}
</AuthScene>
