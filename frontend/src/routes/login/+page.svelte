<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import type { Pathname } from '$app/types';
	import { Button, Label } from 'bits-ui';
	import { isAppPath, login, safeNext } from '$lib/api/auth';
	import { ApiError } from '$lib/api/client';
	import { session } from '$lib/auth.svelte';
	import kitchen from '$brand/lockups/rezepte-kitchen.webp?url';
	import wordmark from '$brand/lockups/rezepte-wordmark.svg?url';
	import { m } from '$lib/paraglide/messages';

	let username = $state('');
	let password = $state('');
	let error = $state<string | null>(null);
	let submitting = $state(false);

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

<!-- Two layouts from one tree. From lg the kitchen scene covers the window and
     the form lies on its empty right third as a card. Below lg the page opens
     like a cookbook: the scene as a plate on top, the fold's shadow, then a
     text page with the wordmark, a printer's ornament and the form. -->
<main class="relative flex min-h-screen flex-col bg-surface lg:block lg:bg-transparent">
	<img
		src={kitchen}
		alt=""
		class="h-[250px] w-full object-cover object-[25%_100%] sm:h-[320px] lg:absolute lg:inset-0 lg:h-full lg:object-[52%_100%]"
	/>
	<div
		class="relative flex flex-1 flex-col items-center gap-3.5 px-6 py-8 before:absolute before:inset-x-0 before:top-0 before:h-[22px] before:bg-linear-to-b before:from-overlay/50 before:to-transparent lg:ml-auto lg:min-h-screen lg:w-[460px] lg:items-stretch lg:justify-center lg:gap-5 lg:px-12 lg:py-14 lg:before:hidden dark:before:from-lightbox/70"
	>
		<!-- The wordmark as a mask, so on the text page it can take the theme's
		     ink; on the painting it keeps its own. Quoted: Vite inlines the small
		     SVG as a data URI, whose parentheses break a bare url(). -->
		<span
			role="img"
			aria-label={m.app_name()}
			style:mask-image={`url("${wordmark}")`}
			class="block aspect-[386.8/94.6] w-[200px] bg-accent-foreground mask-contain mask-center mask-no-repeat forced-color-adjust-none lg:w-[250px] lg:self-center lg:bg-scene-ink"
		></span>
		<div
			aria-hidden="true"
			class="flex w-[180px] items-center gap-3 text-body text-primary before:h-px before:flex-1 before:bg-border after:h-px after:flex-1 after:bg-border lg:hidden"
		>
			❦
		</div>
		<p
			class="mb-1.5 font-display text-body-lg text-text-muted italic lg:-mt-2.5 lg:mb-0 lg:text-center lg:font-sans lg:text-scene-muted lg:not-italic"
		>
			{m.login_welcome()}
		</p>
		<form
			onsubmit={submit}
			class="flex w-full max-w-[440px] flex-col gap-5 lg:rounded-2xl lg:bg-surface lg:p-6 lg:shadow-lift dark:lg:border dark:lg:border-border"
		>
			<h1 class="sr-only font-display text-heading font-medium lg:not-sr-only">
				{m.login_title()}
			</h1>

			<div class="space-y-1.5">
				<Label.Root for="username" class="text-caption font-semibold"
					>{m.login_username()}</Label.Root
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
				<Label.Root for="password" class="text-caption font-semibold"
					>{m.login_password()}</Label.Root
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
	</div>
</main>
