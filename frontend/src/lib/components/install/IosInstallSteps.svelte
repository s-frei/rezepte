<script lang="ts">
	import Share from '@lucide/svelte/icons/share';
	import SquarePlus from '@lucide/svelte/icons/square-plus';
	import { m } from '$lib/paraglide/messages';

	// Each sentence is translated whole and split where Safari's button goes,
	// so each language keeps its own word order (as SignInCard does).
	const share = $derived(m.install_ios_step_share({ button: '\u0000' }).split('\u0000'));
	const confirm = $derived(m.install_ios_step_confirm({ button: '\u0000' }).split('\u0000'));
	const step = 'flex items-center gap-3 text-body';
	const number =
		'flex size-[30px] shrink-0 items-center justify-center rounded-full bg-accent text-caption font-semibold text-accent-foreground';
	const icon = 'inline size-[1.1em] -translate-y-px align-middle text-primary';
</script>

<!--
	Safari's three taps to the home screen, each naming the button as Safari
	labels it and drawing the icon Safari shows beside it, so the words can be
	matched to the screen. Shared by the settings card and the iPhone sheet.
-->
<ol class="space-y-3">
	<li class={step}>
		<span class={number} aria-hidden="true">1</span>
		<span>
			{share[0]}<Share class="{icon} mr-1" aria-hidden="true" /><strong class="font-semibold"
				>{m.install_ios_share()}</strong
			>{share[1]}
		</span>
	</li>
	<li class={step}>
		<span class={number} aria-hidden="true">2</span>
		<span>
			<strong class="font-semibold">{m.install_ios_step_add()}</strong><SquarePlus
				class="{icon} ml-1"
				aria-hidden="true"
			/>
		</span>
	</li>
	<li class={step}>
		<span class={number} aria-hidden="true">3</span>
		<span>
			{confirm[0]}<strong class="font-semibold">{m.install_ios_add()}</strong>{confirm[1]}
		</span>
	</li>
</ol>
