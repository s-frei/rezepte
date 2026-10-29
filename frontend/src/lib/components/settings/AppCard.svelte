<script lang="ts">
	import AppWindow from '@lucide/svelte/icons/app-window';
	import ArrowUpRight from '@lucide/svelte/icons/arrow-up-right';
	import LayoutGrid from '@lucide/svelte/icons/layout-grid';
	import Lock from '@lucide/svelte/icons/lock';
	import Share from '@lucide/svelte/icons/share';
	import { session } from '#lib/auth.svelte.js';
	import IosInstallSteps from '#lib/components/install/IosInstallSteps.svelte';
	import Button from '#lib/components/ui/Button.svelte';
	import { USER_GUIDE_URL } from '#lib/docs.js';
	import { appCardState } from '#lib/install.js';
	import { offer, secure, standalone, startInstall } from '#lib/install.svelte.js';
	import { m } from '#lib/paraglide/messages.js';

	const cardState = $derived(
		appCardState(offer(), { secure, standalone, owner: session.user?.role === 'superadmin' })
	);
	// The sentence is translated whole and split where the bold part goes.
	const share = $derived(m.install_benefit_share({ target: '\u0000' }).split('\u0000'));
	const circle = 'flex size-[30px] shrink-0 items-center justify-center rounded-full';
</script>

<!--
	Installing Rezepte on this device, headed by the real app icon, so the
	card shows what will land on the home screen. With a captured prompt it
	lists what the app gives and offers the browser's dialog; on an iPhone it
	shows Safari's steps instead, since there is nothing to press. The owner
	on plain HTTP learns why there is no install, as only they can fix it; a
	member there, the installed app and a browser that cannot install get no
	card, since it would have nothing to offer.
-->
{#if cardState}
	<section class="rounded-2xl bg-surface p-6 md:p-7" aria-labelledby="settings-app">
		<div class="mb-5 flex items-center gap-4">
			<img src="/icon-192.png" alt="" class="size-15 shrink-0 rounded-lg shadow-card" />
			<div class="min-w-0">
				<h2 id="settings-app" class="font-display text-heading font-medium">
					{m.install_card_title()}
				</h2>
				{#if cardState !== 'https'}
					<p class="mt-0.5 text-caption text-text-muted">
						{cardState === 'ios' ? m.install_card_caption_ios() : m.install_card_caption()}
					</p>
				{/if}
			</div>
		</div>

		{#if cardState === 'prompt'}
			<ul class="space-y-3 text-body">
				<li class="flex items-center gap-3">
					<span class="{circle} bg-accent text-accent-foreground" aria-hidden="true">
						<LayoutGrid class="size-4" />
					</span>
					{m.install_benefit_icon()}
				</li>
				<li class="flex items-center gap-3">
					<span class="{circle} bg-accent text-accent-foreground" aria-hidden="true">
						<AppWindow class="size-4" />
					</span>
					{m.install_benefit_window()}
				</li>
				<li class="flex items-center gap-3">
					<span class="{circle} bg-accent text-accent-foreground" aria-hidden="true">
						<Share class="size-4" />
					</span>
					<span>
						{share[0]}<strong class="font-semibold">{m.install_benefit_share_target()}</strong
						>{share[1]}
					</span>
				</li>
			</ul>
			<div class="mt-5 flex justify-end">
				<Button onclick={() => void startInstall()}>{m.install_button()}</Button>
			</div>
		{:else if cardState === 'ios'}
			<IosInstallSteps />
		{:else}
			<div class="flex gap-3">
				<span class="{circle} bg-background text-text-muted" aria-hidden="true">
					<Lock class="size-4" />
				</span>
				<div class="min-w-0 text-body">
					<p>{m.install_https_text()}</p>
					<p class="mt-1.5 text-caption text-text-muted">{m.install_https_owner_only()}</p>
					<!-- The guide lives on GitHub Pages, outside the SPA router, so
					     `resolve()` does not apply (as in HelpCard). -->
					<!-- eslint-disable svelte/no-navigation-without-resolve -->
					<a
						href="{USER_GUIDE_URL}operations/reverse-proxy/"
						target="_blank"
						rel="noopener external"
						class="mt-3 inline-flex items-center gap-1 text-body-sm font-semibold text-primary hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
					>
						{m.install_https_link()}
						<ArrowUpRight class="size-4" aria-hidden="true" />
						<span class="sr-only">({m.settings_help_new_tab()})</span>
					</a>
					<!-- eslint-enable svelte/no-navigation-without-resolve -->
				</div>
			</div>
		{/if}
	</section>
{/if}
