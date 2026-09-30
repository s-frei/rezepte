<script lang="ts" module>
	import Monitor from '@lucide/svelte/icons/monitor';
	import Moon from '@lucide/svelte/icons/moon';
	import Sun from '@lucide/svelte/icons/sun';
	import { m } from '$lib/paraglide/messages';
	import type { Theme } from '$lib/theme';

	/** The three choices in their order, shared with the desktop account menu. */
	export function themeOptions() {
		return [
			{ value: 'system' as Theme, label: m.settings_theme_system(), icon: Monitor },
			{ value: 'light' as Theme, label: m.settings_theme_light(), icon: Sun },
			{ value: 'dark' as Theme, label: m.settings_theme_dark(), icon: Moon }
		];
	}
</script>

<script lang="ts">
	import SegmentedControl from '$lib/components/ui/SegmentedControl.svelte';
	import { setTheme, theme } from '$lib/theme.svelte';

	/** The settings card explains "System"; the You sheet has no room for it. */
	let { hint = true }: { hint?: boolean } = $props();
</script>

{#if hint}
	<p class="mb-3 text-body-sm text-text-muted">{m.settings_theme_hint()}</p>
{/if}
<SegmentedControl
	value={theme.value}
	options={themeOptions()}
	label={m.settings_theme_label()}
	onchange={(next) => setTheme(next)}
/>
