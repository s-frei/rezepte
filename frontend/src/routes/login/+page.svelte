<script lang="ts">
	import { flushSync, onMount } from 'svelte';
	import { cubicOut } from 'svelte/easing';
	import { prefersReducedMotion } from 'svelte/motion';
	import { MediaQuery } from 'svelte/reactivity';
	import { afterNavigate, goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { login, safeNext, type User } from '#lib/api/auth.js';
	import { ApiError } from '#lib/api/client.js';
	import type { OidcInfo } from '#lib/api/oidc.js';
	import { markArrival } from '#lib/arrival.js';
	import { session } from '#lib/auth.svelte.js';
	import { wait } from '#lib/page-transition.js';
	import { announceSession } from '#lib/session-broadcast.js';
	import AuthScene from '#lib/components/auth/AuthScene.svelte';
	import HelloSplash, { grow } from '#lib/components/switch/HelloSplash.svelte';
	import ProviderButton from '#lib/components/auth/ProviderButton.svelte';
	import Button from '#lib/components/ui/Button.svelte';
	import Input from '#lib/components/ui/Input.svelte';
	import PersonMark from '#lib/components/ui/PersonMark.svelte';
	import { m } from '#lib/paraglide/messages.js';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	// Set by signOut for this one visit; empty after a reload. No avatarId:
	// this page is public, so the mark is the initial on its color.
	const signedOut = $derived(page.state.signedOut);

	// `?add=1`: signing in another person from "Who's cooking?". The name
	// comes along in the query to prefill (it is no secret on this device).
	const adding = $derived(page.url.searchParams.get('add') === '1');
	// Where the sign-in goes on to, in both modes: a deep link or push tap
	// that found the session gone, by way of "Who's cooking?" or not.
	const next = $derived(page.url.searchParams.get('next'));
	// Back to "Who's cooking?" is a step back when that is where it came
	// from, so its × still returns to the app rather than to this page.
	let fromSwitch = $state(false);
	afterNavigate(({ from }) => (fromSwitch = from?.route.id === '/switch'));

	function backToSwitch(event: MouseEvent) {
		if (!fromSwitch || event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey)
			return;
		event.preventDefault();
		history.back();
	}
	let username = $state(page.url.searchParams.get('username') ?? '');
	let password = $state('');
	let error = $state<string | null>(null);
	// Which fields a refused sign-in marked; typing into one clears it, and
	// the message goes with the last of them.
	let wrong = $state({ username: false, password: false });
	let submitting = $state(false);
	let oidc = $state<OidcInfo | null>(null);
	let resetAvailable = $state(false);
	let scene: AuthScene;

	// Below lg the page opens on a cover and the form waits in a sheet; it
	// starts up when the visit came for the form (adding a person, a name
	// filled in), without a focused field. From lg the card is all there is.
	const wide = new MediaQuery('width >= 64rem');
	let open = $state(
		page.url.searchParams.get('add') === '1' || page.url.searchParams.has('username')
	);

	// Inside the tap, synchronously: iOS raises the keyboard only for a focus
	// within the gesture, on a field already shown.
	function raise() {
		open = true;
		flushSync();
		document.getElementById(username ? 'password' : 'username')?.focus({ preventScroll: true });
	}

	// In the sheet, Enter in the name goes on to the password rather than
	// submitting; from lg it submits as it always did. Not while an IME is
	// composing, where Enter confirms the composition.
	function nextField(event: KeyboardEvent) {
		if (event.key !== 'Enter' || event.isComposing || wide.current) return;
		event.preventDefault();
		document.getElementById('password')?.focus();
	}

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
		let user: User;
		try {
			user = await login(username, password);
		} catch (e) {
			error = loginError(e);
			const refused = e instanceof ApiError && e.status === 401;
			wrong = { username: refused, password: refused };
			scene.shake();
			if (refused) {
				document.querySelector<HTMLInputElement>('#password')?.select();
			}
			submitting = false;
			return;
		}
		session.user = user;
		announceSession(user.id);
		const target = safeNext(next);
		// The switch's greeting, after the answer so a refused sign-in never
		// shows one; the app paints its ground again over the reload and
		// closes it into the avatar (#lib/arrival.ts).
		if (!prefersReducedMotion.current) await greet(user);
		await markArrival(user.color, target);
		// A full load, so nothing of the previous person survives; it also
		// reaches a target without a client route (the Scalar docs under
		// /api/, a path that no longer exists).
		window.location.assign(target);
	}

	// What grows into the greeting, as the switch's tapped card does: the
	// open sheet below lg, the card from lg.
	let greeting = $state<User | null>(null);
	let named = $state(false);
	let greeted = $state(false);

	async function greet(user: User) {
		const from = document.querySelector<HTMLElement>(
			wide.current ? '[data-auth-card]' : '[data-auth-page]'
		);
		from?.style.setProperty('view-transition-name', 'chosen-card');
		named = true;
		await grow(
			() => {
				from?.style.removeProperty('view-transition-name');
				// Held first (AuthScene's `still`), so the keyboard leaving and
				// the blur move nothing under the growing sheet.
				flushSync(() => (greeting = user));
				// The keyboard goes with the form it typed into.
				if (document.activeElement instanceof HTMLElement) document.activeElement.blur();
			},
			// From the sheet's corners (none once the plate has collapsed) or the card's.
			from ? getComputedStyle(from).borderRadius : undefined
		);
		named = false;
		await wait(900);
		greeted = true;
		await wait(150);
	}

	// The message opens its line below the fields while the card shakes,
	// rather than pushing the button down in one step; the negative margin
	// takes back the form's gap until it is open.
	function unfold(node: HTMLElement) {
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

<svelte:head
	><title>{adding ? m.login_add_title() : m.login_title()} · {m.app_name()}</title></svelte:head
>

<!-- The card holds two forms side by side rather than one: the provider
     button is a form of its own (a real POST the browser follows to the
     provider), and forms cannot nest. -->
{#snippet signedOutLine()}
	{#if signedOut}
		<p
			role="status"
			class="-mt-1 flex items-center justify-center gap-2 text-center text-caption font-semibold text-text-muted"
		>
			<PersonMark
				person={{
					id: '',
					avatarId: null,
					displayName: signedOut.displayName,
					color: signedOut.color
				}}
				size="xs"
			/>
			{m.login_signed_out({ name: signedOut.displayName })}
		</p>
	{/if}
{/snippet}

{#snippet provider()}
	{#if oidc?.enabled}
		{#if oidcNotice}
			<p role="alert" class="text-caption font-medium text-destructive">{oidcNotice}</p>
		{/if}
		<ProviderButton intent="login" name={oidc.name} next={next ?? ''} />
	{/if}
{/snippet}

<!-- Below lg the cover holds what needs no keyboard: who signed out, the
     button that raises the sheet and the provider's sign-in. -->
{#snippet cover()}
	{@render signedOutLine()}
	<Button size="lg" class="w-full" onclick={raise}>{m.login_submit()}</Button>
	{@render provider()}
{/snippet}

<AuthScene
	bind:this={scene}
	bind:open
	frame="cover"
	labelledby="login-title"
	{cover}
	lead={m.login_welcome()}
	still={greeting !== null}
>
	<form onsubmit={submit} class="flex flex-col gap-5">
		<h1 id="login-title" class="sr-only font-display text-heading font-medium lg:not-sr-only">
			{adding ? m.login_add_title() : m.login_title()}
		</h1>

		{#if wide.current}
			{@render signedOutLine()}
		{/if}

		<!-- From lg the page has one job, so it starts in its first field. It
		     has to be the attribute: SvelteKit moves focus to <body> after
		     every navigation, the redirect here included, unless an element
		     carries `autofocus`. Below lg the cover's tap focuses. -->
		<Input
			id="username"
			name="username"
			label={m.login_username()}
			floating
			autofocus={wide.current && !username}
			autocomplete="username"
			autocapitalize="none"
			spellcheck="false"
			enterkeyhint="next"
			onkeydown={nextField}
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
			autofocus={wide.current && !!username}
			autocomplete="current-password"
			enterkeyhint="go"
			bind:value={password}
			oninput={() => clear('password')}
			aria-invalid={wrong.password || undefined}
		/>

		{#if error}
			<p role="alert" in:unfold class="text-caption font-semibold text-destructive">
				{error}
			</p>
		{/if}

		<Button type="submit" size="lg" class="w-full" busy={submitting}>
			{submitting ? m.login_submitting() : m.login_submit()}
		</Button>

		<!-- Last in the DOM so Tab runs username, password, submit, link. -->
		{#if adding}
			<a
				href={next ? `${resolve('/switch')}?next=${encodeURIComponent(next)}` : resolve('/switch')}
				onclick={backToSwitch}
				class="relative self-center text-caption font-semibold text-primary underline after:absolute after:inset-x-0 after:top-1/2 after:h-11 after:-translate-y-1/2 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
			>
				{m.login_add_back()}
			</a>
		{/if}
		{#if resetAvailable}
			<a
				href={resolve('/forgot-password')}
				onclick={openForgot}
				class="relative self-center text-caption font-semibold text-primary underline after:absolute after:inset-x-0 after:top-1/2 after:h-11 after:-translate-y-1/2 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
			>
				{m.login_forgot()}
			</a>
		{/if}
	</form>

	{#if oidc?.enabled && wide.current}
		<div class="flex items-center gap-3 text-caption text-text-muted">
			<span class="h-px flex-1 bg-border"></span>
			{m.login_or()}
			<span class="h-px flex-1 bg-border"></span>
		</div>
		{@render provider()}
	{/if}
</AuthScene>

{#if greeting}
	<HelloSplash person={greeting} {named} {greeted} />
{/if}
