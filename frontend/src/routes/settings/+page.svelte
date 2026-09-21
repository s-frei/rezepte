<script lang="ts">
	import { session } from '$lib/auth.svelte';
	import PasswordForm from '$lib/components/settings/PasswordForm.svelte';
	import SettingsLayout from '$lib/components/settings/SettingsLayout.svelte';
	import ThemeControl from '$lib/components/settings/ThemeControl.svelte';
	import { m } from '$lib/paraglide/messages';
	import { roleLabel } from '$lib/roles';

	const card = 'rounded-2xl bg-surface p-6 md:p-7';
	const title = 'mb-4 font-display text-heading font-medium';
	const initial = $derived(session.user?.username.charAt(0).toUpperCase() ?? '');
</script>

<svelte:head><title>{m.settings_title()} · {m.app_name()}</title></svelte:head>

<SettingsLayout active="profile">
	<section class={card} aria-labelledby="settings-profile">
		<h2 id="settings-profile" class={title}>{m.settings_nav_profile()}</h2>
		<div class="flex items-center gap-4">
			<span
				aria-hidden="true"
				class="flex size-12 items-center justify-center rounded-full bg-accent font-display text-heading font-semibold text-accent-foreground"
			>
				{initial}
			</span>
			<div>
				<p class="text-body font-semibold">{session.user?.username}</p>
				<p class="text-caption text-text-muted">{roleLabel(session.user?.role)}</p>
			</div>
		</div>
	</section>

	<section class={card} aria-labelledby="settings-password">
		<h2 id="settings-password" class={title}>{m.settings_password_title()}</h2>
		<PasswordForm />
	</section>

	<section class={card} aria-labelledby="settings-theme">
		<h2 id="settings-theme" class={title}>{m.settings_theme_title()}</h2>
		<ThemeControl />
	</section>
</SettingsLayout>
