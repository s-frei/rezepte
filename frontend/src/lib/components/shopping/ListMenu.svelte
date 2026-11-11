<script lang="ts">
	import DialogActions from '#lib/components/ui/DialogActions.svelte';
	import Copy from '@lucide/svelte/icons/copy';
	import Ellipsis from '@lucide/svelte/icons/ellipsis';
	import ListPlus from '@lucide/svelte/icons/list-plus';
	import LogOut from '@lucide/svelte/icons/log-out';
	import Pencil from '@lucide/svelte/icons/pencil';
	import Share2 from '@lucide/svelte/icons/share-2';
	import Trash2 from '@lucide/svelte/icons/trash-2';
	import { Dialog, DropdownMenu } from 'bits-ui';
	import BaseDialog from '#lib/components/ui/BaseDialog.svelte';
	import Button from '#lib/components/ui/Button.svelte';
	import ConfirmDialog from '#lib/components/ui/ConfirmDialog.svelte';
	import Input from '#lib/components/ui/Input.svelte';
	import { m } from '#lib/paraglide/messages.js';

	let {
		name,
		owner,
		onrename,
		onshare,
		onaddtext,
		oncopy,
		ondelete,
		onleave
	}: {
		name: string;
		/** The creator may share and delete; a member may leave instead. */
		owner: boolean;
		onshare: () => void;
		/** Resolves to whether the new name was stored, which closes the dialog. */
		onrename: (name: string) => Promise<boolean>;
		/** Opens the dialog that adds pasted text to the list. */
		onaddtext: () => void;
		oncopy: () => void;
		ondelete: () => void;
		onleave: () => void;
	} = $props();

	let renameOpen = $state(false);
	let deleteOpen = $state(false);
	let leaveOpen = $state(false);
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
		class="inline-flex size-10 shrink-0 items-center justify-center rounded-full border border-border bg-surface hover-tint text-text transition focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary active:scale-[.98]"
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
			<DropdownMenu.Item onSelect={openRename} class={item}>
				<Pencil class="size-4" aria-hidden="true" />
				{m.list_rename()}
			</DropdownMenu.Item>
			{#if owner}
				<DropdownMenu.Item onSelect={onshare} class={item}>
					<Share2 class="size-4" aria-hidden="true" />
					{m.list_share()}
				</DropdownMenu.Item>
			{/if}
			<DropdownMenu.Item onSelect={onaddtext} class={item}>
				<ListPlus class="size-4" aria-hidden="true" />
				{m.list_add_text()}
			</DropdownMenu.Item>
			<DropdownMenu.Item onSelect={oncopy} class={item}>
				<Copy class="size-4" aria-hidden="true" />
				{m.list_copy_text()}
			</DropdownMenu.Item>
			<DropdownMenu.Separator class="mx-1 my-1.5 h-px bg-popover-border" />
			<DropdownMenu.Item
				onSelect={() => (owner ? (deleteOpen = true) : (leaveOpen = true))}
				class="flex h-10 items-center gap-2 rounded-sm px-3 text-body-sm text-destructive transition hover:bg-destructive-soft"
			>
				{#if owner}
					<Trash2 class="size-4" aria-hidden="true" />
					{m.list_delete()}
				{:else}
					<LogOut class="size-4" aria-hidden="true" />
					{m.list_leave()}
				{/if}
			</DropdownMenu.Item>
		</DropdownMenu.Content>
	</DropdownMenu.Portal>
</DropdownMenu.Root>

<BaseDialog bind:open={renameOpen}>
	<Dialog.Title class="font-display text-heading font-medium">{m.list_rename()}</Dialog.Title>
	<form onsubmit={rename} class="mt-4 space-y-5">
		<Input
			id="rename-list-name"
			label={m.lists_new_name_label()}
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
	bind:open={deleteOpen}
	title={m.list_delete_confirm_title({ name })}
	text={m.list_delete_confirm_text()}
	confirmLabel={m.common_delete()}
	destructive
	onconfirm={ondelete}
/>

<ConfirmDialog
	bind:open={leaveOpen}
	title={m.list_leave_confirm_title({ name })}
	text={m.list_leave_confirm_text()}
	confirmLabel={m.list_leave()}
	destructive
	onconfirm={onleave}
/>
