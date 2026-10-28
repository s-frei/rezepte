<script lang="ts">
	import Download from '@lucide/svelte/icons/download';
	import X from '@lucide/svelte/icons/x';
	import { MediaQuery } from 'svelte/reactivity';
	import { dismissBanner, install, offer, startInstall } from '$lib/install.svelte';
	import { m } from '$lib/paraglide/messages';

	// The same breakpoint as the bottom nav: a desktop never gets the banner,
	// it has the user menu's row and the settings card.
	const phone = new MediaQuery('(max-width: 767.98px) and (pointer: coarse)');
	const current = $derived(offer());
</script>

<!--
	A slim strip above the overview's running head, offering the install on a
	phone. The whole left part is the button, a thumb's target rather than a
	small link; × hides only this strip, for good on this device, while the
	You sheet's row and the settings card stay until the app is installed.
-->
{#if current && phone.current && !install.dismissed}
	<div class="mb-4 flex items-center rounded-xl bg-surface">
		<button
			type="button"
			onclick={() => void startInstall()}
			class="flex min-w-0 flex-1 items-center gap-3 rounded-xl py-2.5 pl-3 text-left focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-primary"
		>
			<span
				class="flex size-9 shrink-0 items-center justify-center rounded-full bg-accent text-accent-foreground"
				aria-hidden="true"
			>
				<Download class="size-4.5" />
			</span>
			<span class="min-w-0">
				<span class="block text-body-sm font-semibold">{m.install_banner_title()}</span>
				<span class="block text-caption text-text-muted">
					{current === 'ios' ? m.install_banner_ios() : m.install_banner_share()}
				</span>
			</span>
		</button>
		<button
			type="button"
			onclick={dismissBanner}
			aria-label={m.install_banner_dismiss()}
			title={m.install_banner_dismiss()}
			class="flex size-11 shrink-0 items-center justify-center rounded-xl text-text-muted transition hover:text-text focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-primary"
		>
			<X class="size-4.5" aria-hidden="true" />
		</button>
	</div>
{/if}
