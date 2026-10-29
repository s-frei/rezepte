<script lang="ts">
	import { onMount } from 'svelte';
	import { cubicOut } from 'svelte/easing';
	import { prefersReducedMotion } from 'svelte/motion';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import type { Pathname } from '$app/types';
	import { isAppPath, login, safeNext } from '$lib/api/auth';
	import { ApiError } from '$lib/api/client';
	import type { OidcInfo } from '$lib/api/oidc';
	import { session } from '$lib/auth.svelte';
	import AuthScene from '$lib/components/auth/AuthScene.svelte';
	import ProviderButton from '$lib/components/auth/ProviderButton.svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import Input from '$lib/components/ui/Input.svelte';
	import { m } from '$lib/paraglide/messages';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	let username = $state('');
	let password = $state('');
	let error = $state<string | null>(null);
	// Which fields a refused sign-in marked; typing into one clears it, and
	// the message goes with the last of them.
	let wrong = $state({ username: false, password: false });
	let submitting = $state(false);
	let oidc = $state<OidcInfo | null>(null);
	let resetAvailable = $state(false);
	let scene: AuthScene;

	// Usually settled already (the load waits a moment for them), so they
	// are in place before the first paint.
	onMount(() => {
		void data.oidc.then((info) => (oidc = info));
		void data.resetAvailable.then((available) => (resetAvailable = available));
	});

	// Carries the typed name along in navigation state, so the next page
	// starts filled in without the name landing in the URL, the history or
	// a server log. A modified click (new tab) opens the page empty.
	function openForgot(event: MouseEvent) {
		const name = username.trim();
		if (
			!name ||
			event.button !== 0 ||
			event.metaKey ||
			event.ctrlKey ||
			event.shiftKey ||
			event.altKey
		)
			return;
		event.preventDefault();
		void goto(resolve('/forgot-password'), { state: { login: name } });
	}

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

	function loginError(e: unknown): string {
		if (e instanceof ApiError && e.status === 401) return m.login_failed();
		if (e instanceof ApiError && e.status === 429) return m.login_throttled();
		return m.login_error_generic();
	}

	function clear(field: 'username' | 'password') {
		if (!wrong[field]) return;
		wrong[field] = false;
		if (!wrong.username && !wrong.password) error = null;
	}

	async function submit(event: SubmitEvent) {
		event.preventDefault();
		// The button stays pressable while busy (`aria-busy`), so a second
		// press or Enter lands here.
		if (submitting) return;
		error = null;
		wrong = { username: false, password: false };
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
			const refused = e instanceof ApiError && e.status === 401;
			wrong = { username: refused, password: refused };
			scene.shake();
			if (refused) {
				document.querySelector<HTMLInputElement>('#password')?.select();
			}
		} finally {
			submitting = false;
		}
	}

	// The message opens its line below the fields while the card shakes,
	// rather than pushing the button down in one step; the negative margin
	// takes back the form's gap until it is open.
	function open(node: HTMLElement) {
		const height = node.offsetHeight;
		return {
			delay: prefersReducedMotion.current ? 0 : 160,
			duration: prefersReducedMotion.current ? 0 : 360,
			easing: cubicOut,
			css: (t: number) =>
				`height: ${t * height}px; margin-bottom: ${(t - 1) * 1.25}rem; opacity: ${t}; translate: 0 ${(t - 1) * 6}px; overflow: hidden;`
		};
	}
</script>

<svelte:head><title>{m.login_title()} · {m.app_name()}</title></svelte:head>

<!-- The card holds two forms side by side rather than one: the provider
     button is a form of its own (a real POST the browser follows to the
     provider), and forms cannot nest. -->
<AuthScene bind:this={scene} lead={m.login_welcome()}>
	<form onsubmit={submit} class="flex flex-col gap-5">
		<h1 class="sr-only font-display text-heading font-medium lg:not-sr-only">
			{m.login_title()}
		</h1>

		<!-- The page has one job, so it starts in its first field. It has to
		     be the attribute: SvelteKit moves focus to <body> after every
		     navigation, the redirect here included, unless an element
		     carries `autofocus`. -->
		<Input
			id="username"
			name="username"
			label={m.login_username()}
			floating
			autofocus
			autocomplete="username"
			autocapitalize="none"
			spellcheck="false"
			bind:value={username}
			oninput={() => clear('username')}
			aria-invalid={wrong.username || undefined}
		/>
		<!-- One message under both fields, so each field only says it is wrong. -->
		<Input
			id="password"
			name="password"
			type="password"
			label={m.login_password()}
			floating
			autocomplete="current-password"
			bind:value={password}
			oninput={() => clear('password')}
			aria-invalid={wrong.password || undefined}
		/>

		{#if error}
			<p role="alert" in:open class="text-caption font-semibold text-destructive">
				{error}
			</p>
		{/if}

		<Button type="submit" size="lg" class="w-full" busy={submitting}>
			{submitting ? m.login_submitting() : m.login_submit()}
		</Button>

		<!-- Last in the DOM so Tab runs username, password, submit, link. -->
		{#if resetAvailable}
			<a
				href={resolve('/forgot-password')}
				onclick={openForgot}
				class="self-center text-caption font-semibold text-primary underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
			>
				{m.login_forgot()}
			</a>
		{/if}
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
