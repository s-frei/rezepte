<script lang="ts">
	import MailCard from '$lib/components/settings/MailCard.svelte';
	import SettingsLayout from '$lib/components/settings/SettingsLayout.svelte';
	import { m } from '$lib/paraglide/messages';
</script>

<svelte:head><title>{m.settings_nav_mail()} · {m.app_name()}</title></svelte:head>

<SettingsLayout active="mail">
	<MailCard />
</SettingsLayout>
