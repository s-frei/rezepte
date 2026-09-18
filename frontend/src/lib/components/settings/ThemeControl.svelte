<script lang="ts">
	import Monitor from 'lucide-svelte/icons/monitor';
	import Moon from 'lucide-svelte/icons/moon';
	import Sun from 'lucide-svelte/icons/sun';
	import SegmentedControl from '$lib/components/ui/SegmentedControl.svelte';
	import { m } from '$lib/paraglide/messages';
	import type { Theme } from '$lib/theme';
	import { setTheme, theme } from '$lib/theme.svelte';

	const options = [
		{ value: 'system' as Theme, label: m.settings_theme_system(), icon: Monitor },
		{ value: 'light' as Theme, label: m.settings_theme_light(), icon: Sun },
		{ value: 'dark' as Theme, label: m.settings_theme_dark(), icon: Moon }
	];
</script>

<p class="mb-3 text-body-sm text-text-muted">{m.settings_theme_hint()}</p>
<SegmentedControl
	value={theme.value}
	{options}
	label={m.settings_theme_label()}
	onchange={(next) => setTheme(next)}
/>
