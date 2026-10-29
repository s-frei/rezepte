<script lang="ts">
	import OidcCard from '#lib/components/settings/OidcCard.svelte';
	import SettingsLayout from '#lib/components/settings/SettingsLayout.svelte';
	import { m } from '#lib/paraglide/messages.js';
</script>

<svelte:head><title>{m.settings_nav_sign_in()} · {m.app_name()}</title></svelte:head>

<SettingsLayout active="sign-in">
	<OidcCard />
</SettingsLayout>
