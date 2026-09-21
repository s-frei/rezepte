<script lang="ts">
	import { Dialog } from 'bits-ui';
	import { toast } from 'svelte-sonner';
	import { ApiError, isSignedOut } from '$lib/api/client';
	import {
		createToken,
		type CreatedApiToken,
		type TokenExpiry,
		type TokenScope
	} from '$lib/api/tokens';
	import BaseDialog from '$lib/components/ui/BaseDialog.svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import Input from '$lib/components/ui/Input.svelte';
	import SegmentedControl from '$lib/components/ui/SegmentedControl.svelte';
	import Select from '$lib/components/ui/Select.svelte';
	import { m } from '$lib/paraglide/messages';

	let {
		open = $bindable(false),
		oncreated
	}: { open?: boolean; oncreated: (token: CreatedApiToken) => void } = $props();

	/** One row of the scope picker: none, read, or read plus write. */
	type Access = 'none' | 'read' | 'write';

	let name = $state('');
	let recipes = $state<Access>('read');
	let users = $state<Access>('none');
	let expiry = $state('90');
	let errors = $state<{ name?: string; scopes?: string }>({});
	let saving = $state(false);

	const accessOptions: { value: Access; label: string }[] = [
		{ value: 'none', label: m.tokens_access_none() },
		{ value: 'read', label: m.tokens_access_read() },
		{ value: 'write', label: m.tokens_access_write() }
	];

	const expiryOptions = [
		{ value: '30', label: m.tokens_expiry_30() },
		{ value: '90', label: m.tokens_expiry_90() },
		{ value: '365', label: m.tokens_expiry_365() },
		{ value: 'never', label: m.tokens_expiry_never() }
	];

	// Write always carries its read scope, so the server compares plain lists
	// instead of applying an implication rule.
	function scopesOf(area: 'recipes' | 'users', access: Access): TokenScope[] {
		if (access === 'none') return [];
		const read = `${area}:read` as TokenScope;
		return access === 'read' ? [read] : [read, `${area}:write` as TokenScope];
	}

	const scopes = $derived([...scopesOf('recipes', recipes), ...scopesOf('users', users)]);

	// A fresh form every time the dialog opens.
	$effect(() => {
		if (open) {
			name = '';
			recipes = 'read';
			users = 'none';
			expiry = '90';
			errors = {};
		}
	});

	async function submit(event: SubmitEvent) {
		event.preventDefault();
		errors = {};
		if (name.trim() === '') {
			errors.name = m.tokens_validation_name();
		}
		if (scopes.length === 0) {
			errors.scopes = m.tokens_validation_scopes();
		}
		if (Object.keys(errors).length > 0) {
			return;
		}
		saving = true;
		try {
			const expiresInDays: TokenExpiry =
				expiry === 'never' ? null : (Number(expiry) as 30 | 90 | 365);
			const created = await createToken({ name: name.trim(), scopes, expiresInDays });
			toast.success(m.tokens_created({ name: created.name }));
			open = false;
			oncreated(created);
		} catch (error) {
			if (error instanceof ApiError && error.status === 422) {
				errors.name = error.detail ?? m.tokens_create_error();
			} else if (!isSignedOut(error)) {
				// 401 already redirects to the login page (see $lib/api/client).
				toast.error(m.tokens_create_error());
			}
		} finally {
			saving = false;
		}
	}
</script>

<BaseDialog bind:open>
	<Dialog.Title class="font-display text-heading font-medium">
		{m.tokens_create_title()}
	</Dialog.Title>
	<Dialog.Description class="sr-only">{m.tokens_create_description()}</Dialog.Description>
	<form onsubmit={submit} class="mt-5 space-y-4">
		<Input
			id="new-token-name"
			label={m.tokens_field_name()}
			hint={m.tokens_field_name_hint()}
			autocomplete="off"
			bind:value={name}
			error={errors.name ?? null}
		/>
		<div class="space-y-3">
			<div class="flex flex-col gap-1.5">
				<span class="text-body-sm font-semibold">{m.tokens_access_recipes()}</span>
				<SegmentedControl
					bind:value={recipes}
					options={accessOptions}
					label={m.tokens_access_recipes()}
				/>
			</div>
			<div class="flex flex-col gap-1.5">
				<span class="text-body-sm font-semibold">{m.tokens_access_users()}</span>
				<SegmentedControl
					bind:value={users}
					options={accessOptions}
					label={m.tokens_access_users()}
				/>
			</div>
			{#if errors.scopes}
				<p class="text-caption text-destructive">{errors.scopes}</p>
			{/if}
		</div>
		<div class="flex flex-col gap-1.5">
			<span class="text-body-sm font-semibold">{m.tokens_field_expiry()}</span>
			<Select
				bind:value={expiry}
				options={expiryOptions}
				label={m.tokens_field_expiry()}
				class="w-full bg-surface-elevated"
			/>
		</div>
		<div class="flex justify-end gap-3 pt-2">
			<Button variant="ghost" onclick={() => (open = false)}>{m.common_cancel()}</Button>
			<Button type="submit" disabled={saving}>{m.tokens_create_submit()}</Button>
		</div>
	</form>
</BaseDialog>
