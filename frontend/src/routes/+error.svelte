<script lang="ts">
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import notFoundArt from '$brand/lockups/rezepte-mascot-not-found.webp?url';
	import errorArt from '$brand/lockups/rezepte-mascot-error.webp?url';
	import Button from '#lib/components/ui/Button.svelte';
	import { session } from '#lib/auth.svelte.js';
	import { m } from '#lib/paraglide/messages.js';

	// A 404 is a link that leads nowhere; anything else (the server down, a
	// load that threw) is our fault, and trying again may well help. An
	// unknown path while the server is down is the latter (+layout.ts).
	const unreachable = $derived(Boolean(page.data.unreachable));
	const notFound = $derived(page.status === 404 && !unreachable);
	const title = $derived(notFound ? m.error_not_found_title() : m.error_failed_title());
	const label = $derived(unreachable ? m.error_offline() : m.error_code({ status: page.status }));
</script>

<svelte:head><title>{title} · {m.app_name()}</title></svelte:head>

<!-- The error page carries no bottom nav (`navPlace` in `#lib/shell.svelte`),
     so it offers its own way back. Without a session the page renders bare
     (+layout.svelte) and centers itself in the window. -->
<main
	class="mx-auto grid max-w-4xl items-center justify-items-center gap-10 px-6 py-10 text-center md:grid-cols-[300px_25rem] md:justify-center md:justify-items-start md:gap-14 md:py-16 md:text-left {session.user
		? ''
		: 'min-h-dvh content-center'}"
>
	<!-- The painting lies as a card on a small stack of pages: it drops in, the
	     pages fan out under it, and after an error it shakes once. -->
	<div class="card relative w-60 md:w-full" class:shake={!notFound}>
		<span class="page page-back" aria-hidden="true"></span>
		<span class="page page-front" aria-hidden="true"></span>
		<div class="pic relative rounded-3xl border border-border bg-surface p-2.5 shadow-lift">
			<img
				src={notFound ? notFoundArt : errorArt}
				alt=""
				width="320"
				height="320"
				class="block aspect-square w-full rounded-xl"
			/>
		</div>
	</div>

	<div class="text flex flex-col items-center md:items-start">
		<span
			class="inline-flex h-7 items-center gap-1.5 rounded-pill bg-accent px-3 text-body-sm font-semibold text-accent-foreground"
		>
			{#if unreachable}<span class="dot size-1.5 rounded-full bg-current"></span>{/if}
			{label}
		</span>
		<h1 class="mt-4 font-display text-display-sm font-semibold text-balance md:text-display-lg">
			{title}
		</h1>
		<p class="mt-3 max-w-sm text-body text-balance text-text-muted">
			{notFound ? m.error_not_found_text() : m.error_failed_text()}
		</p>
		<div class="mt-8 flex flex-wrap justify-center gap-3 md:justify-start">
			{#if !notFound}
				<Button onclick={() => location.reload()}>{m.error_retry()}</Button>
			{/if}
			<!-- With the server unreachable, the recipes would only fail again. -->
			{#if !unreachable}
				<Button variant={notFound ? 'primary' : 'secondary'} href={resolve('/')}>
					{m.error_back_to_recipes()}
				</Button>
			{/if}
		</div>
	</div>
</main>

<style>
	.page {
		position: absolute;
		inset: 0;
		border-radius: var(--radius-3xl);
		border: 1px solid var(--color-border);
		background: var(--color-surface);
		box-shadow: var(--shadow-lift);
	}
	.page-back {
		transform: rotate(7deg) translate(10px, 6px);
	}
	.page-front {
		transform: rotate(-4deg) translate(-6px, 4px);
	}
	.pic {
		transform: rotate(-2deg);
	}

	@media (prefers-reduced-motion: no-preference) {
		.card {
			animation: drop 900ms var(--ease-spring) both;
		}
		.page {
			animation: fan 900ms 200ms var(--ease-spring) backwards;
		}
		.shake .pic {
			animation: shake 620ms 700ms ease-in-out both;
		}
		.text > :global(*) {
			animation: rise 560ms var(--ease-spring) both;
		}
		.text > :global(:nth-child(2)) {
			animation-delay: 120ms;
		}
		.text > :global(:nth-child(3)) {
			animation-delay: 220ms;
		}
		.text > :global(:nth-child(4)) {
			animation-delay: 320ms;
		}
		.dot {
			animation: blink 1.6s ease-in-out infinite;
		}
	}

	@keyframes drop {
		from {
			opacity: 0;
			transform: translateY(-40px) rotate(-8deg) scale(0.96);
		}
	}
	@keyframes fan {
		from {
			transform: none;
		}
	}
	@keyframes shake {
		0%,
		100% {
			transform: rotate(-2deg);
		}
		20% {
			transform: rotate(-5deg) translateX(-4px);
		}
		40% {
			transform: rotate(1.5deg) translateX(4px);
		}
		60% {
			transform: rotate(-4deg) translateX(-3px);
		}
		80% {
			transform: rotate(-1deg) translateX(2px);
		}
	}
	@keyframes rise {
		from {
			opacity: 0;
			transform: translateY(18px);
		}
	}
	@keyframes blink {
		50% {
			opacity: 0.25;
		}
	}
</style>
