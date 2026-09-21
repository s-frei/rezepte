<script lang="ts">
	import { onDestroy, onMount } from 'svelte';
	import { toast } from 'svelte-sonner';
	import { listColorUsage, updateOwnProfile, type ColorUsage } from '$lib/api/auth';
	import { isSignedOut } from '$lib/api/client';
	import { session } from '$lib/auth.svelte';
	import ColorPicker from '$lib/components/settings/ColorPicker.svelte';
	import PasswordForm from '$lib/components/settings/PasswordForm.svelte';
	import ProfileForm from '$lib/components/settings/ProfileForm.svelte';
	import SettingsLayout from '$lib/components/settings/SettingsLayout.svelte';
	import ThemeControl from '$lib/components/settings/ThemeControl.svelte';
	import { m } from '$lib/paraglide/messages';
	import { roleLabel } from '$lib/roles';
	import { userColorClasses, USER_COLORS, type UserColor } from '$lib/user/color';

	const card = 'rounded-2xl bg-surface p-6 md:p-7';
	const title = 'mb-4 font-display text-heading font-medium';
	const initial = $derived(session.user?.displayName.charAt(0).toUpperCase() ?? '');

	let color = $state<UserColor>(session.user?.color ?? USER_COLORS[0]);
	let usage = $state<ColorUsage[]>([]);

	// Advisory: the counts only mark a colour somebody else already holds, so
	// a failed load leaves the palette unmarked rather than the picker locked.
	async function loadUsage() {
		try {
			usage = await listColorUsage();
		} catch {
			usage = [];
		}
	}

	onMount(() => {
		void loadUsage();
	});

	/**
	 * Picking a colour is the save. There is nothing else to fill in and no
	 * second field to wait for, so a swatch that needed a separate button
	 * would read as not having worked. The name below keeps its button
	 * because a name is only finished when the person stops typing.
	 *
	 * This runs through a function binding rather than an effect watching
	 * `color`: the write belongs to the act of picking, not to the value
	 * happening to differ from the session's.
	 *
	 * `RadioGroup.Root` is a WAI-ARIA radio group, so arrow keys *move*
	 * selection rather than just focus it - a run across the palette calls
	 * `pick` once per key. The save itself is trailing-edge debounced so
	 * that run turns into one request for the colour the person landed on,
	 * not one per key. `saveSeq` guards the response side of the same race:
	 * it names the most recent pick, and a save whose answer comes back
	 * after a newer pick has already fired is dropped instead of
	 * overwriting `session.user` with a stale colour.
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
			if (seq !== saveSeq) {
				// A later pick has already replaced this one; its own save owns
				// `session.user` and the toast from here on.
				return;
			}
			session.user = saved;
			toast.success(m.settings_profile_saved());
			// The old colour is free again and the new one is taken; both
			// marks are wrong until the counts come back.
			await loadUsage();
		} catch (failure) {
			if (seq !== saveSeq) {
				return;
			}
			// Back to what the server still has, so the picker never shows a
			// colour that was never stored.
			color = session.user?.color ?? color;
			if (!isSignedOut(failure)) {
				// 401 already redirects to the login page (see $lib/api/client).
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
	<section class={card} aria-labelledby="settings-profile">
		<h2 id="settings-profile" class={title}>{m.settings_nav_profile()}</h2>
		<!-- The avatar is the one place on this page where the chosen colour is
		     shown at size, and it is the same circle the recipe cards paint, so
		     a pick can be judged here instead of on the overview. -->
		<div class="flex items-center gap-4">
			<span
				aria-hidden="true"
				class="flex size-14 items-center justify-center rounded-full initial-centred font-display text-heading font-semibold {userColorClasses(
					session.user?.color
				)}"
			>
				{initial}
			</span>
			<div>
				<p class="text-body-lg font-semibold">{session.user?.displayName}</p>
				<p class="text-caption text-text-muted">{roleLabel(session.user?.role)}</p>
			</div>
		</div>

		<!--
			The order of this card is the avatar's doing. The avatar above is the
			only place the chosen colour appears at size, so the picker sits
			directly under it and a pick can be judged where it lands. The login
			name follows as the other thing about the account that simply is. The
			one editable field with a button comes last, so the button ends the
			card instead of splitting it in half - which is what made it read as
			misplaced when the name sat on top.
		-->
		<div class="mt-6 space-y-6 border-t border-border pt-6">
			<ColorPicker
				bind:value={() => color, pick}
				{usage}
				label={m.settings_profile_color_label()}
			/>

			<!--
				A fact, not a field: the login name cannot be changed here, and a
				disabled input would invite the click that proves it. A
				description list says the same thing in markup, and the label
				keeps the column the form below it sets.

				The value sits in the filled block this app already uses for a
				value you read rather than write (the token dialog), but sized to
				its content instead of to the column. That width is what carries
				the difference: a field always spans its column, so something
				that hugs three characters cannot be one, while bare text on the
				card had no form at all and read as a stray paragraph.
			-->
			<dl>
				<dt class="text-caption font-semibold">{m.settings_profile_username_label()}</dt>
				<dd class="mt-1.5">
					<span class="inline-block rounded-md bg-background px-3 py-1.5 text-body">
						{session.user?.username}
					</span>
					<span class="mt-1.5 block text-micro text-text-muted">
						{m.settings_profile_username_hint()}
					</span>
				</dd>
			</dl>

			<ProfileForm />
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
