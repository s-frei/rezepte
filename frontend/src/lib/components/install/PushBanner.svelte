<script lang="ts">
	import BellRing from '@lucide/svelte/icons/bell-ring';
	import X from '@lucide/svelte/icons/x';
	import { toast } from 'svelte-sonner';
	import { cubicOut } from 'svelte/easing';
	import { prefersReducedMotion } from 'svelte/motion';
	import { slide } from 'svelte/transition';
	import { standalone } from '#lib/install.svelte.js';
	import { m } from '#lib/paraglide/messages.js';
	import { deviceState, enableDevice, pushUndecided } from '#lib/push.svelte.js';
	import { load, save } from '#lib/storage.js';

	const STORAGE_KEY = 'rezepte-push-banner-dismissed';

	// Kept in memory as well, so × works where storage throws.
	let dismissed = $state(load(STORAGE_KEY) === '1');

	function dismiss() {
		dismissed = true;
		save(STORAGE_KEY, '1');
	}

	// No await before enableDevice: the permission prompt needs the tap.
	// Allowed or declined, the permission is no longer 'default' and the
	// note goes; only a failed subscribe leaves it, with a toast.
	function enable() {
		enableDevice().catch(() => {
			if (deviceState() !== 'blocked') toast.error(m.settings_notifications_enable_error());
		});
	}

	// As the install banner: only × folds the note away with motion.
	function collapse(node: Element) {
		const duration = dismissed && !prefersReducedMotion.current ? 240 : 0;
		const fold = slide(node, { duration, easing: cubicOut });
		return { ...fold, css: (t: number, u: number) => `${fold.css?.(t, u)}; opacity: ${t}` };
	}
</script>

<!--
	The installed app's counterpart to the install banner, in the same
	cookbook note: whoever installed Rezepte learns it can notify them, and
	turns that on with one tap. Only inside the installed app (where the
	install banner never shows), only while this device was never asked, and
	× hides it for good on this device; Settings → Notifications stays.
-->
{#if standalone && pushUndecided() && !dismissed}
	<div out:collapse class="mb-5 rounded-2xl bg-surface px-4 pt-3 pb-4 shadow-card">
		<div class="flex items-center gap-2.5">
			<span class="font-display text-body-sm font-medium text-primary italic">
				{m.push_banner_label()}
			</span>
			<span
				class="min-w-4 flex-1 -translate-y-0.5 border-b-2 border-dotted border-handle"
				aria-hidden="true"
			></span>
			<!-- 44px target, pulled out with negative margins so the look stays. -->
			<button
				type="button"
				onclick={dismiss}
				aria-label={m.push_banner_dismiss()}
				title={m.push_banner_dismiss()}
				class="-my-1 -mr-3 flex size-11 shrink-0 items-center justify-center text-text-muted transition hover:text-text focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-primary"
			>
				<X class="size-4" aria-hidden="true" />
			</button>
		</div>
		<!-- The bell sits beside the title, not beside the text: next to a
		     52px icon the German copy broke on every word at 320px. -->
		<div class="md:flex md:items-center md:gap-6">
			<div class="mt-1 md:flex-1">
				<div class="flex items-center gap-3">
					<span
						class="flex size-10 shrink-0 items-center justify-center rounded-full bg-accent text-accent-foreground"
					>
						<BellRing class="size-5" aria-hidden="true" />
					</span>
					<p class="font-display text-heading font-medium">{m.push_banner_title()}</p>
				</div>
				<p class="mt-2 text-body-sm font-medium">{m.push_banner_lead()}</p>
				<p class="text-caption text-text-muted">{m.push_banner_detail()}</p>
			</div>
			<button
				type="button"
				onclick={enable}
				class="mt-4 inline-flex h-11 w-full items-center justify-center rounded-pill bg-primary hover-tint text-body-sm font-semibold text-primary-foreground transition focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary active:scale-[.98] md:mt-1 md:w-auto md:shrink-0 md:px-6"
			>
				{m.push_banner_button()}
			</button>
		</div>
	</div>
{/if}
