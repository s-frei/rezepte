<script lang="ts">
	import KeyRound from '@lucide/svelte/icons/key-round';
	import Languages from '@lucide/svelte/icons/languages';
	import LogIn from '@lucide/svelte/icons/log-in';
	import SunMoon from '@lucide/svelte/icons/sun-moon';
	import { onDestroy, onMount } from 'svelte';
	import { toast } from 'svelte-sonner';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import {
		me,
		removeOwnAvatar,
		resendConfirmation,
		setOwnAvatar,
		updateOwnProfile
	} from '#lib/api/auth.js';
	import { ApiError, isSignedOut } from '#lib/api/client.js';
	import { getOidc, getOwnIdentity, unlinkOwnIdentity, type OidcInfo } from '#lib/api/oidc.js';
	import { getSettings } from '#lib/api/settings.js';
	import { listPeople, type PersonEntry } from '#lib/api/users.js';
	import { session } from '#lib/auth.svelte.js';
	import AboutCard from '#lib/components/settings/AboutCard.svelte';
	import AppCard from '#lib/components/settings/AppCard.svelte';
	import ColorPicker from '#lib/components/settings/ColorPicker.svelte';
	import EmailDialog from '#lib/components/settings/EmailDialog.svelte';
	import EmailRow from '#lib/components/settings/EmailRow.svelte';
	import HelpCard from '#lib/components/settings/HelpCard.svelte';
	import LanguageControl from '#lib/components/settings/LanguageControl.svelte';
	import PasswordDialog from '#lib/components/settings/PasswordDialog.svelte';
	import ProfileHead from '#lib/components/settings/ProfileHead.svelte';
	import SettingsGroup from '#lib/components/settings/SettingsGroup.svelte';
	import SettingsLayout from '#lib/components/settings/SettingsLayout.svelte';
	import SettingsRow from '#lib/components/settings/SettingsRow.svelte';
	import ThemeControl from '#lib/components/settings/ThemeControl.svelte';
	import Button from '#lib/components/ui/Button.svelte';
	import { m } from '#lib/paraglide/messages.js';
	import { formatDate } from '#lib/recipe/format.js';
	import { roleLabel } from '#lib/roles.js';
	import { USER_COLORS, type UserColor } from '#lib/user/color.js';

	let color = $state<UserColor>(session.user?.color ?? USER_COLORS[0]);
	let people = $state<PersonEntry[]>([]);
	// Whether the instance offers a sign-in provider; the sign-in row only
	// exists when it does, and a failed lookup leaves it out.
	let oidc = $state<OidcInfo | null>(null);
	// When the own account was connected to the provider; null while it is not.
	let linkedAt = $state<string | null>(null);
	let identityLoaded = $state(false);
	let unlinking = $state(false);
	let emailOpen = $state(false);
	// Whether Rezepte sends mail: the email row offers a send only then. Null
	// until known, or when the settings could not be read: the row then says
	// nothing about mail rather than the wrong thing.
	let mailOn = $state<boolean | null>(null);
	let passwordOpen = $state(false);

	const user = $derived(session.user);
	const hasPassword = $derived(!!user?.hasPassword);

	onMount(() => {
		// Advisory: the list only names who else wears a color, so a failed
		// load leaves the line saying nobody does rather than the picker locked.
		listPeople().then(
			(list) => (people = list),
			() => (people = [])
		);
		getSettings().then(
			(settings) => (mailOn = settings.mailEnabled),
			() => (mailOn = null)
		);
		getOidc().then(
			(info) => {
				oidc = info;
				if (info.enabled) void loadIdentity(info.name);
			},
			() => (oidc = null)
		);
	});

	async function loadIdentity(name: string) {
		// The provider flow comes back to /settings?oidc=...: say how it went
		// once, then drop the query so a reload does not say it again.
		const result = page.url.searchParams.get('oidc');
		if (result === 'linked') toast.success(m.oidc_linked({ name }));
		else if (result === 'taken') toast.error(m.oidc_taken({ name }));
		else if (result === 'failed') toast.error(m.oidc_failed({ name }));
		if (result) {
			void goto(resolve('/settings'), { replaceState: true });
		}
		try {
			linkedAt = (await getOwnIdentity())?.linkedAt ?? null;
		} catch {
			// Unknown reads as not connected: connecting an identity the
			// account already has is harmless, the service treats it as done.
			linkedAt = null;
		} finally {
			identityLoaded = true;
		}
	}

	async function disconnect() {
		unlinking = true;
		try {
			await unlinkOwnIdentity();
			linkedAt = null;
		} catch (e) {
			// Without a password the service refuses (409): the account would
			// be locked out. The button is off then too; this is the race.
			if (e instanceof ApiError && e.status === 409) {
				toast.error(m.settings_signin_need_password());
			} else if (!isSignedOut(e)) {
				toast.error(m.users_update_error());
			}
		} finally {
			unlinking = false;
		}
	}

	async function rename(name: string) {
		// Straight into the shared session, so the top bar, the recipe cards
		// and the head all change at once.
		session.user = await updateOwnProfile({ displayName: name });
		toast.success(m.settings_profile_saved());
	}

	async function saveEmail(email: string) {
		const saved = await updateOwnProfile({ email });
		session.user = saved;
		return saved;
	}

	/** The row rereads the account after a send, so its bar restarts. */
	async function reloadUser() {
		session.user = await me().catch(() => session.user);
	}

	/**
	 * Picking a color is the save: there is nothing else to fill in, so a
	 * swatch that needed a separate button would read as not having worked.
	 *
	 * `RadioGroup.Root` is a WAI-ARIA radio group, so arrow keys *move*
	 * selection - a run across the palette calls `pick` once per key. The
	 * save is trailing-edge debounced so that run turns into one request for
	 * the color the person landed on. `saveSeq` names the most recent pick,
	 * and a save whose answer comes back after a newer pick has fired is
	 * dropped instead of overwriting `session.user` with a stale color.
	 */
	let saveTimer: ReturnType<typeof setTimeout> | undefined;
	let saveSeq = 0;

	function pick(picked: UserColor) {
		color = picked;
		if (saveTimer !== undefined) {
			clearTimeout(saveTimer);
		}
		const seq = ++saveSeq;
		saveTimer = setTimeout(() => {
			saveTimer = undefined;
			void saveColor(picked, seq);
		}, 400);
	}

	async function saveColor(picked: UserColor, seq: number) {
		try {
			const saved = await updateOwnProfile({ color: picked });
			if (seq !== saveSeq) return;
			session.user = saved;
			toast.success(m.settings_profile_saved());
		} catch (failure) {
			if (seq !== saveSeq) return;
			// Back to what the server still has, so the picker never shows a
			// color that was never stored.
			color = session.user?.color ?? color;
			if (!isSignedOut(failure)) {
				toast.error(m.settings_profile_error());
			}
		}
	}

	onDestroy(() => {
		// A navigation mid-run must not fire a save into a torn-down page.
		if (saveTimer !== undefined) {
			clearTimeout(saveTimer);
		}
	});
</script>

<svelte:head><title>{m.settings_title()} · {m.app_name()}</title></svelte:head>

<SettingsLayout active="profile">
	<!--
		One card, sorted by what people come to do: the head is how the
		household sees you (photo, name, color, each edited in place), then the
		account, then the choices that only change how the app looks to you.
		The head reads the color the picker holds rather than the session's,
		so the band follows a pick before its debounced save lands.
	-->
	{#if user}
		<section aria-label={m.settings_nav_profile()} class="overflow-hidden rounded-2xl bg-surface">
			<ProfileHead
				person={{ ...user, color }}
				roleLabel={roleLabel(user.role)}
				editable
				upload={setOwnAvatar}
				removeAvatar={removeOwnAvatar}
				onavatar={(avatarId) => {
					if (session.user) session.user = { ...session.user, avatarId };
				}}
				onrename={rename}
			>
				{#snippet colors()}
					<ColorPicker
						bind:value={() => color, pick}
						{people}
						selfId={user.id}
						label={m.settings_profile_color_label()}
						variant="head"
					/>
				{/snippet}
			</ProfileHead>

			<div class="space-y-8 px-5 pt-8 pb-3 md:px-7 md:pb-4">
				<SettingsGroup label={m.profile_group_account()}>
					<EmailRow
						email={user.email}
						emailVerified={user.emailVerified}
						emailConfirmation={user.emailConfirmation}
						emailSetupLink={user.emailSetupLink}
						{mailOn}
						resend={resendConfirmation}
						onchanged={reloadUser}
						none={m.profile_email_none()}
					>
						{#snippet control()}
							<Button
								variant="quiet"
								size="sm"
								label={m.profile_email_change()}
								onclick={() => (emailOpen = true)}
							>
								{m.profile_change()}
							</Button>
						{/snippet}
					</EmailRow>
					<SettingsRow icon={KeyRound} label={m.profile_password_label()}>
						{#snippet value()}
							{hasPassword ? m.profile_password_hint() : m.profile_password_none()}
						{/snippet}
						{#snippet control()}
							<Button
								variant="quiet"
								size="sm"
								label={hasPassword ? m.settings_password_title() : m.settings_password_set_title()}
								onclick={() => (passwordOpen = true)}
							>
								{hasPassword ? m.profile_change() : m.profile_password_set()}
							</Button>
						{/snippet}
					</SettingsRow>
					{#if oidc?.enabled && identityLoaded}
						{@const name = oidc.name}
						<SettingsRow icon={LogIn} label={m.settings_signin_title()}>
							{#snippet value()}
								{#if linkedAt}
									{m.settings_signin_linked({ name, date: formatDate(linkedAt) })}
									{#if !hasPassword}
										<span class="block">{m.settings_signin_need_password()}</span>
									{/if}
								{:else}
									{m.settings_signin_unlinked({ name })}
								{/if}
							{/snippet}
							{#snippet control()}
								{#if linkedAt}
									<Button
										variant="quiet"
										size="sm"
										label={m.profile_signin_disconnect_label({ name })}
										disabled={unlinking || !hasPassword}
										onclick={disconnect}
									>
										{m.settings_signin_disconnect()}
									</Button>
								{:else}
									<!-- A real form post: the browser has to follow the
									     service's 303 to the provider. -->
									<form method="POST" action="/api/v1/auth/oidc/start">
										<input type="hidden" name="intent" value="link" />
										<Button
											type="submit"
											variant="quiet"
											size="sm"
											label={m.profile_signin_connect_label({ name })}
										>
											{m.profile_signin_connect()}
										</Button>
									</form>
								{/if}
							{/snippet}
						</SettingsRow>
					{/if}
				</SettingsGroup>

				<SettingsGroup label={m.profile_group_preferences()}>
					<SettingsRow icon={Languages} label={m.settings_language_title()}>
						{#snippet value()}{m.profile_language_hint()}{/snippet}
						{#snippet control()}<LanguageControl />{/snippet}
					</SettingsRow>
					<SettingsRow icon={SunMoon} label={m.settings_theme_title()} stack>
						{#snippet control()}<ThemeControl size="sm" />{/snippet}
					</SettingsRow>
				</SettingsGroup>
			</div>
		</section>

		<EmailDialog
			bind:open={emailOpen}
			email={user.email}
			sentAt={user.emailConfirmation?.sentAt}
			{mailOn}
			save={saveEmail}
		/>
		<PasswordDialog bind:open={passwordOpen} />
	{/if}

	<AppCard />
	<HelpCard />
	<AboutCard />
</SettingsLayout>
