<script lang="ts">
	import type { RecipeCard } from '$lib/api/recipes';
	import SettingsLayout from '$lib/components/settings/SettingsLayout.svelte';
	import ExportCard from '$lib/components/settings/ExportCard.svelte';
	import ImportCard from '$lib/components/settings/ImportCard.svelte';
	import { m } from '$lib/paraglide/messages';

	let recipes = $state<RecipeCard[]>([]);
	// The import marks titles already here, so it waits for the full list.
	let ready = $state(false);
</script>

<svelte:head><title>{m.settings_nav_transfer()} · {m.app_name()}</title></svelte:head>

<SettingsLayout active="transfer">
	<ExportCard bind:recipes onready={() => (ready = true)} />
	<ImportCard existing={recipes} {ready} />
</SettingsLayout>
