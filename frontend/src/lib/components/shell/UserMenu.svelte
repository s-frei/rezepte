<script lang="ts">
	import LogOut from '@lucide/svelte/icons/log-out';
	import Settings from '@lucide/svelte/icons/settings';
	import SunMoon from '@lucide/svelte/icons/sun-moon';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { DropdownMenu } from 'bits-ui';
	import { session } from '$lib/auth.svelte';
	import { themeOptions } from '$lib/components/settings/ThemeControl.svelte';
	import { m } from '$lib/paraglide/messages';
	import { roleLabel } from '$lib/roles';
	import { signOut } from '$lib/sign-out';
	import { isTheme } from '$lib/theme';
	import { setTheme, theme } from '$lib/theme.svelte';
	import PersonMark from '$lib/components/ui/PersonMark.svelte';

	/** The desktop top bar's avatar menu; a phone reaches the same through the bottom nav's "You". */
	let { open = $bindable(false) }: { open?: boolean } = $props();

	const user = $derived(session.user);

	const row =
		'flex h-10 items-center gap-2.5 rounded-sm px-3 text-body-sm transition hover:bg-background';
</script>

<DropdownMenu.Root bind:open>
	<DropdownMenu.Trigger aria-label={m.nav_user_menu()} class="rounded-full">
		{#if user}<PersonMark person={user} size="lg" neutral />{/if}
	</DropdownMenu.Trigger>
	<DropdownMenu.Portal>
		<DropdownMenu.Content
			preventScroll={false}
			sideOffset={8}
			align="end"
			class="w-61 rounded-2xl border border-popover-border bg-popover p-2 shadow-dialog"
		>
			<!-- Headed by the person, as the You sheet is, so desktop and phone
			     read the same: who is signed in, then the rows with their icons. -->
			{#if user}
				<div class="flex items-center gap-2.5 px-3 pt-2 pb-2.5">
					<PersonMark person={user} size="md" />
					<div class="min-w-0">
						<p class="truncate font-display text-body font-medium">{user.displayName}</p>
						<p class="truncate text-caption text-text-muted">
							<!-- The sheet's "Owner, signed in as mia" did not fit the menu's width. -->
							{m.nav_user_menu_signed_in({ role: roleLabel(user.role), username: user.username })}
						</p>
					</div>
				</div>
				<DropdownMenu.Separator class="mx-3 mb-1.5 h-px bg-popover-border" />
			{/if}
			<DropdownMenu.Item onSelect={() => void goto(resolve('/settings'))} class="{row} text-text">
				<Settings class="size-4 shrink-0 text-text-muted" aria-hidden="true" />
				{m.nav_settings()}
			</DropdownMenu.Item>
			<!-- The theme as one row like the others: the word on the left, three
			     small icon segments on the right. Radio items rather than a
			     SegmentedControl, so the menu's own arrow keys reach them; they keep
			     the menu open, and the page behind it recolors while it does. The
			     row's own icon is the neutral sun-moon: the current scheme already
			     shows in the checked segment beside it. -->
			<DropdownMenu.RadioGroup
				value={theme.value}
				onValueChange={(next) => {
					if (isTheme(next)) {
						setTheme(next);
					}
				}}
				class="flex h-10 items-center gap-2.5 px-3"
			>
				<SunMoon class="size-4 shrink-0 text-text-muted" aria-hidden="true" />
				<DropdownMenu.GroupHeading class="text-body-sm text-text">
					{m.settings_theme_title()}
				</DropdownMenu.GroupHeading>
				<div class="ml-auto flex rounded-pill bg-background p-0.5">
					{#each themeOptions() as option (option.value)}
						{@const Icon = option.icon}
						<DropdownMenu.RadioItem
							value={option.value}
							closeOnSelect={false}
							aria-label={option.label}
							title={option.label}
							class="flex h-6 w-6.5 items-center justify-center rounded-pill text-text-muted transition outline-none hover:text-text focus-visible:outline-2 focus-visible:outline-primary data-highlighted:text-text data-[state=checked]:bg-surface data-[state=checked]:text-text data-[state=checked]:shadow-card"
						>
							<Icon class="size-3.5" aria-hidden="true" />
						</DropdownMenu.RadioItem>
					{/each}
				</div>
			</DropdownMenu.RadioGroup>
			<DropdownMenu.Separator class="mx-3 my-1.5 h-px bg-popover-border" />
			<DropdownMenu.Item
				onSelect={() => {
					open = false;
					void signOut();
				}}
				class="{row} text-destructive"
			>
				<LogOut class="size-4 shrink-0" aria-hidden="true" />
				{m.logout()}
			</DropdownMenu.Item>
		</DropdownMenu.Content>
	</DropdownMenu.Portal>
</DropdownMenu.Root>
