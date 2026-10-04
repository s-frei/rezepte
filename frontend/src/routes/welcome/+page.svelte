<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { getPasswordReset, inspectSetupLink, redeemSetupLink } from '$lib/api/auth';
	import { ApiError } from '$lib/api/client';
	import { getOidc, type OidcInfo } from '$lib/api/oidc';
	import AuthScene from '$lib/components/auth/AuthScene.svelte';
	import ProviderButton from '$lib/components/auth/ProviderButton.svelte';
	import { session } from '$lib/auth.svelte';
	import PasswordStrength from '$lib/components/settings/PasswordStrength.svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import Input from '$lib/components/ui/Input.svelte';
	import { withoutErrors } from '$lib/form-errors';
	import { m } from '$lib/paraglide/messages';
	import {
		PASSWORD_MIN,
		passwordErrorsFromApi,
		validateNewPassword,
		type PasswordErrors
	} from '$lib/settings/password';

	// Read once on mount and kept only in memory from then on: the address bar
	// is stripped of the fragment immediately (below), whether or not the
	// form is ever finished, so an abandoned tab leaves no live token behind
	// in the address bar or in history. See auth.SetupPath on the service
	// side for why the token was in the fragment at all - it never reaches a
	// server or proxy log or a Referer header.
	// $state rather than a plain variable because the provider button renders
	// it into its form; it still never reaches the address bar again.
	let token = $state('');
	let oidc = $state<OidcInfo | null>(null);
	let resetAvailable = $state(false);
	// Set when a provider sign-in came back here. The service redirects
	// without the fragment, so the token is gone and the person has to open
	// the link again.
	const oidcReturn = $derived(page.url.searchParams.get('oidc'));
	let username = $state('');
	let displayName = $state('');
	let purpose = $state<'setup' | 'reset'>('setup');
	const reset = $derived(purpose === 'reset');
	let loading = $state(true);
	let invalid = $state(false);
	// A non-404 inspect failure (offline, 503, ...): the token is still good,
	// only the lookup failed, so this offers a retry rather than declaring
	// the link dead.
	let loadError = $state(false);
	// Whether an invite was ever successfully resolved - what the <title>
	// and the heading key off, so neither shows "Welcome, " with nothing
	// after it while loading or on an error.
	const ready = $derived(!loading && !invalid && !loadError);
	// Never "Welcome,  · Rezepte" with nobody's name in it - the tab title
	// falls back to the plain app name until an invite has actually loaded.
	const pageTitle = $derived(
		ready
			? `${reset ? m.welcome_reset_title({ name: displayName }) : m.welcome_title({ name: displayName })} · ${m.app_name()}`
			: m.app_name()
	);

	let password = $state('');
	let repeat = $state('');
	let errors = $state<PasswordErrors>({});
	let submitting = $state(false);
	let submitError = $state<string | null>(null);

	async function loadInvite() {
		loading = true;
		invalid = false;
		loadError = false;
		try {
			const info = await inspectSetupLink(token);
			username = info.username;
			displayName = info.displayName;
			purpose = info.purpose;
		} catch (e) {
			if (e instanceof ApiError && e.status === 404) {
				// Unknown, used or expired - one message for all three, so
				// this page learns nothing a prober could not already tell
				// from the response.
				invalid = true;
			} else {
				loadError = true;
			}
		} finally {
			loading = false;
		}
	}

	onMount(async () => {
		token = window.location.hash.slice(1);
		window.history.replaceState(null, '', '/welcome' + window.location.search);
		// The button is an extra, so a failed lookup simply leaves it out.
		const info = getOidc().catch(() => null);
		void info.then((i) => (oidc = i));
		getPasswordReset().then(
			(r) => (resetAvailable = r.available),
			() => {}
		);
		if (!token) {
			// Coming back from the provider: wait for its name, so the page
			// does not flash "this link no longer works" first.
			if (oidcReturn) {
				await info;
			}
			invalid = true;
			loading = false;
			return;
		}
		void loadInvite();
	});

	async function submit(event: SubmitEvent) {
		event.preventDefault();
		submitError = null;
		errors = validateNewPassword(password, repeat);
		if (Object.keys(errors).length > 0) {
			return;
		}
		submitting = true;
		try {
			session.user = await redeemSetupLink(token, password);
			await goto(resolve('/'));
		} catch (e) {
			if (e instanceof ApiError && e.status === 404) {
				// Used by another tab, or simply expired, while this one sat open.
				invalid = true;
			} else if (e instanceof ApiError && e.status === 422) {
				errors = passwordErrorsFromApi(e.errors);
				if (Object.keys(errors).length === 0) {
					submitError = m.login_error_generic();
				}
			} else {
				submitError = m.login_error_generic();
			}
		} finally {
			submitting = false;
		}
	}
</script>

<svelte:head><title>{pageTitle}</title></svelte:head>

<!-- Every state shares the login's scene and card; only the form states
     carry the line under the wordmark. -->
<AuthScene
	lead={ready
		? reset
			? m.welcome_reset_lead()
			: oidc?.enabled
				? m.welcome_lead_choose()
				: m.welcome_lead()
		: ''}
>
	{#if !loading}
		{#if !token && oidc?.enabled && (oidcReturn === 'failed' || oidcReturn === 'taken')}
			<div class="space-y-2 text-center lg:text-left">
				<p role="alert" class="text-body font-medium text-destructive">
					{oidcReturn === 'taken'
						? m.oidc_taken({ name: oidc.name })
						: m.oidc_failed({ name: oidc.name })}
				</p>
				<p class="text-body text-text-muted">{m.welcome_reopen()}</p>
			</div>
		{:else if invalid}
			<div class="space-y-2 text-center lg:text-left">
				<h1 class="font-display text-heading font-medium">{m.welcome_invalid()}</h1>
				<p class="text-body text-text-muted">
					{#if resetAvailable}
						{m.welcome_invalid_hint_reset_before()}<a
							href={resolve('/forgot-password')}
							class="font-semibold text-primary underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
							>{m.welcome_invalid_hint_reset_link()}</a
						>{m.welcome_invalid_hint_reset_after()}
					{:else}
						{m.welcome_invalid_hint()}
					{/if}
				</p>
			</div>
			<a
				href={resolve('/login')}
				class="self-center text-caption font-semibold text-text-muted underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
			>
				{m.forgot_back()}
			</a>
		{:else if loadError}
			<p class="text-center text-body text-text-muted lg:text-left">{m.login_error_generic()}</p>
			<Button variant="secondary" onclick={loadInvite}>{m.common_retry()}</Button>
		{:else}
			<h1 class="font-display text-heading font-medium">
				{reset
					? m.welcome_reset_title({ name: displayName })
					: m.welcome_title({ name: displayName })}
			</h1>
			{#if reset}
				<p class="text-body text-text-muted">{m.welcome_reset_text()}</p>
			{/if}
			{#if session.user && session.user.username !== username}
				<!-- Another account is signed in on this browser: the link is
				     still usable, but finishing it signs that account out here. -->
				<p role="status" class="text-body text-text-muted">
					{m.welcome_signed_in_as({ name: session.user.displayName, invited: displayName })}
				</p>
			{/if}
			<!-- A reset link only sets a password; the service refuses it in the
			     provider flow anyway. -->
			{#if oidc?.enabled && !reset}
				<ProviderButton intent="setup" name={oidc.name} setup={token} />
				<div class="flex items-center gap-3 text-caption text-text-muted">
					<span class="h-px flex-1 bg-border"></span>
					{m.login_or()}
					<span class="h-px flex-1 bg-border"></span>
				</div>
			{/if}
			<form onsubmit={submit} class="flex flex-col gap-5">
				<!-- Tells a password manager which account this is: without it the
				     new password is saved as a second, nameless entry. -->
				<input
					type="text"
					name="username"
					autocomplete="username"
					value={username}
					readonly
					hidden
				/>
				<div class="space-y-1.5">
					<Input
						id="welcome-password"
						label={m.settings_password_new()}
						type="password"
						autocomplete="new-password"
						required
						bind:value={password}
						oninput={() => (errors = withoutErrors(errors, ['next', 'repeat']))}
						error={errors.next ?? null}
						hint={m.settings_password_too_short({ min: PASSWORD_MIN })}
					/>
					<PasswordStrength {password} userInputs={[username, displayName]} />
				</div>
				<Input
					id="welcome-password-repeat"
					label={m.settings_password_repeat()}
					type="password"
					autocomplete="new-password"
					required
					bind:value={repeat}
					oninput={() => (errors = withoutErrors(errors, ['repeat']))}
					error={errors.repeat ?? null}
				/>
				{#if submitError}
					<p role="alert" class="text-caption font-medium text-destructive">{submitError}</p>
				{/if}
				<Button type="submit" size="lg" class="w-full" disabled={submitting}>
					{reset ? m.welcome_reset_submit() : m.welcome_submit()}
				</Button>
			</form>
		{/if}
	{/if}
</AuthScene>
