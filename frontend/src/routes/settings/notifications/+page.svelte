<script lang="ts">
	import NotificationsCard from '$lib/components/settings/NotificationsCard.svelte';
	import SettingsLayout from '$lib/components/settings/SettingsLayout.svelte';
	import { m } from '$lib/paraglide/messages';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();
</script>

<svelte:head><title>{m.settings_nav_notifications()} · {m.app_name()}</title></svelte:head>

<SettingsLayout active="notifications">
	<NotificationsCard prefs={data.prefs} mailEnabled={data.mailEnabled} />
</SettingsLayout>
