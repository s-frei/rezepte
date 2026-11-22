<script lang="ts">
	import CalendarSync from '@lucide/svelte/icons/calendar-sync';
	import Ellipsis from '@lucide/svelte/icons/ellipsis';
	import House from '@lucide/svelte/icons/house';
	import LogOut from '@lucide/svelte/icons/log-out';
	import Pencil from '@lucide/svelte/icons/pencil';
	import Share2 from '@lucide/svelte/icons/share-2';
	import Trash2 from '@lucide/svelte/icons/trash-2';
	import { Dialog, DropdownMenu } from 'bits-ui';
	import BaseDialog from '#lib/components/ui/BaseDialog.svelte';
	import Button from '#lib/components/ui/Button.svelte';
	import ConfirmDialog from '#lib/components/ui/ConfirmDialog.svelte';
	import DialogActions from '#lib/components/ui/DialogActions.svelte';
	import { iconCircle } from '#lib/components/ui/IconButton.svelte';
	import Input from '#lib/components/ui/Input.svelte';
	import CalendarFeedDialog from './CalendarFeedDialog.svelte';
	import StaplesDialog from './StaplesDialog.svelte';
	import { m } from '#lib/paraglide/messages.js';

	/**
	 * A plan's ⋯. Every plan offers its staples ("Haben wir immer") and the
	 * calendar, one feed over all plans the reader is in. A named plan's, as a list's: everyone renames; the creator
	 * shares and deletes, a member leaves. The personal plan has only the calendar.
	 */
	let {
		planId,
		personal = false,
		name,
		owner,
		onrename,
		onshare,
		ondelete,
		onleave
	}: {
		planId: string;
		personal?: boolean;
		name: string;
		owner: boolean;
		/** Resolves to whether the new name was stored, which closes the dialog. */
		onrename: (name: string) => Promise<boolean>;
		onshare: () => void;
		ondelete: () => void;
		onleave: () => void;
	} = $props();

	let renameOpen = $state(false);
	let calendarOpen = $state(false);
	let staplesOpen = $state(false);
	let confirmOpen = $state(false);
	let draft = $state('');
	let saving = $state(false);

	function openRename() {
		draft = name;
		renameOpen = true;
	}

	async function rename(event: SubmitEvent) {
		event.preventDefault();
		const trimmed = draft.trim();
		if (!trimmed || saving) return;
		if (trimmed === name) {
			renameOpen = false;
			return;
		}
		saving = true;
		try {
			if (await onrename(trimmed)) renameOpen = false;
		} finally {
			saving = false;
		}
	}

	const item =
		'flex h-10 items-center gap-2 rounded-sm px-3 text-body-sm text-text transition hover:bg-background';
</script>

<DropdownMenu.Root>
	<DropdownMenu.Trigger
		aria-label={m.detail_menu()}
		class="{iconCircle} bg-surface text-text max-md:size-11"
	>
		<Ellipsis class="size-5" aria-hidden="true" />
	</DropdownMenu.Trigger>
	<DropdownMenu.Portal>
		<DropdownMenu.Content
			preventScroll={false}
			sideOffset={8}
			align="end"
			class="z-50 w-60 rounded-2xl border border-popover-border bg-popover p-2 shadow-dialog"
		>
			{#if !personal}
				<DropdownMenu.Item onSelect={openRename} class={item}>
					<Pencil class="size-4" aria-hidden="true" />
					{m.plan_rename()}
				</DropdownMenu.Item>
				{#if owner}
					<DropdownMenu.Item onSelect={onshare} class={item}>
						<Share2 class="size-4" aria-hidden="true" />
						{m.plan_share()}
					</DropdownMenu.Item>
				{/if}
			{/if}
			<DropdownMenu.Item onSelect={() => (staplesOpen = true)} class={item}>
				<House class="size-4" aria-hidden="true" />
				{m.plan_staple()}
			</DropdownMenu.Item>
			<DropdownMenu.Item onSelect={() => (calendarOpen = true)} class={item}>
				<CalendarSync class="size-4" aria-hidden="true" />
				{m.plan_calendar()}
			</DropdownMenu.Item>
			{#if !personal}
				<DropdownMenu.Separator class="mx-1 my-1.5 h-px bg-popover-border" />
				<DropdownMenu.Item
					onSelect={() => (confirmOpen = true)}
					class="flex h-10 items-center gap-2 rounded-sm px-3 text-body-sm text-destructive transition hover:bg-destructive-soft"
				>
					{#if owner}
						<Trash2 class="size-4" aria-hidden="true" />
						{m.plan_delete()}
					{:else}
						<LogOut class="size-4" aria-hidden="true" />
						{m.plan_leave()}
					{/if}
				</DropdownMenu.Item>
			{/if}
		</DropdownMenu.Content>
	</DropdownMenu.Portal>
</DropdownMenu.Root>

<StaplesDialog bind:open={staplesOpen} {planId} />

<CalendarFeedDialog bind:open={calendarOpen} />

<BaseDialog bind:open={renameOpen}>
	<Dialog.Title class="font-display text-heading font-medium">{m.plan_rename()}</Dialog.Title>
	<form onsubmit={rename} class="mt-4 space-y-5">
		<Input
			id="rename-plan-name"
			label={m.plan_name_label()}
			autocomplete="off"
			maxlength={80}
			bind:value={draft}
		/>
		<DialogActions>
			<Button variant="ghost" onclick={() => (renameOpen = false)}>{m.common_cancel()}</Button>
			<Button type="submit" busy={saving} disabled={!draft.trim()}>{m.common_save()}</Button>
		</DialogActions>
	</form>
</BaseDialog>

<ConfirmDialog
	bind:open={confirmOpen}
	title={owner ? m.plan_delete_confirm_title({ name }) : m.plan_leave_confirm_title({ name })}
	text={owner ? m.plan_delete_confirm_text() : m.plan_leave_confirm_text()}
	confirmLabel={owner ? m.common_delete() : m.plan_leave()}
	destructive
	onconfirm={owner ? ondelete : onleave}
/>
