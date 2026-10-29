<script lang="ts">
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
	A cookbook note above the overview's running head, offering the install on
	a phone: the "Tip" label and its dotted leader echo that running head right
	below, then the app icon, a lead sentence with its detail, and the button.
	× hides only this note, for good on this device, while the You sheet's row
	and the settings card stay until the app is installed. The copy is the same
	on every platform; on an iPhone the button opens Safari's steps.
-->
{#if current && phone.current && !install.dismissed}
	<div class="mb-5 rounded-2xl bg-surface px-4 pt-3 pb-4 shadow-card">
		<div class="flex items-center gap-2.5">
			<span class="font-display text-body-sm font-medium text-primary italic">
				{m.install_banner_label()}
			</span>
			<span
				class="min-w-4 flex-1 -translate-y-0.5 border-b-2 border-dotted border-handle"
				aria-hidden="true"
			></span>
			<!-- 44px target, pulled out with negative margins so the look stays. -->
			<button
				type="button"
				onclick={dismissBanner}
				aria-label={m.install_banner_dismiss()}
				title={m.install_banner_dismiss()}
				class="-my-1 -mr-3 flex size-11 shrink-0 items-center justify-center text-text-muted transition hover:text-text focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-primary"
			>
				<X class="size-4" aria-hidden="true" />
			</button>
		</div>
		<div class="mt-1 flex items-center gap-3.5">
			<img src="/icon-192.png" alt="" class="size-13 shrink-0 rounded-xl shadow-card" />
			<div class="min-w-0">
				<p class="font-display text-heading font-medium">{m.install_banner_title()}</p>
				<p class="mt-0.5 text-body-sm font-medium">{m.install_banner_lead()}</p>
				<p class="text-caption text-text-muted">{m.install_banner_detail()}</p>
			</div>
		</div>
		<button
			type="button"
			onclick={() => void startInstall()}
			class="mt-4 inline-flex h-11 w-full items-center justify-center rounded-pill bg-primary hover-tint text-body-sm font-semibold text-primary-foreground transition focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary active:scale-[.98]"
		>
			{m.install_button()}
		</button>
	</div>
{/if}
