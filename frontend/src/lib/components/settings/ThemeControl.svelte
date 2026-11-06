<script lang="ts" module>
	import Monitor from '@lucide/svelte/icons/monitor';
	import Moon from '@lucide/svelte/icons/moon';
	import Sun from '@lucide/svelte/icons/sun';
	import { m } from '#lib/paraglide/messages.js';
	import type { Theme } from '#lib/theme.js';

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
	import SegmentedControl from '#lib/components/ui/SegmentedControl.svelte';
	import { setTheme, theme } from '#lib/theme.svelte.js';

	let {
		size = 'md'
	}: {
		/** `sm` for a `SettingsRow`, whose controls share one height. */
		size?: 'md' | 'sm';
	} = $props();
</script>

<SegmentedControl
	value={theme.value}
	options={themeOptions()}
	label={m.settings_theme_label()}
	{size}
	onchange={(next) => setTheme(next)}
/>
