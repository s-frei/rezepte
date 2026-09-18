<script lang="ts">
	import { Dialog } from 'bits-ui';
	import { toast } from 'svelte-sonner';
	import { fade, scale } from 'svelte/transition';
	import { ApiError, isSignedOut } from '$lib/api/client';
	import { updateUser, type UserAccount } from '$lib/api/users';
	import Button from '$lib/components/ui/Button.svelte';
	import Input from '$lib/components/ui/Input.svelte';
	import { m } from '$lib/paraglide/messages';
	import { passwordErrorsFromApi, validateNewPassword } from '$lib/settings/password';

	// Takes a nullable user and stays mounted: wrapping the dialog in an
	// `{#if user}` tore it down the moment the page dropped its target, so the
	// close transition never played. The page keeps the target until the next
	// open, which also keeps the title in place while the dialog animates out.
	let { open = $bindable(false), user }: { open?: boolean; user: UserAccount | null } = $props();

	let password = $state('');
	let error = $state<string | null>(null);
	let saving = $state(false);

	// A fresh field every time the dialog opens.
	$effect(() => {
		if (open) {
			password = '';
			error = null;
		}
	});

	async function submit(event: SubmitEvent) {
		event.preventDefault();
		if (!user) {
			return;
		}
		error = validateNewPassword(password, password).next ?? null;
		if (error) {
			return;
		}
		saving = true;
		try {
			await updateUser(user.id, { password });
			toast.success(m.users_password_reset());
			open = false;
		} catch (failure) {
			if (failure instanceof ApiError && failure.status === 422) {
				error = passwordErrorsFromApi(failure.errors).next ?? null;
				if (!error) toast.error(failure.detail ?? m.users_update_error());
			} else if (!isSignedOut(failure)) {
				// 401 already redirects to the login page (see $lib/api/client).
				toast.error(m.users_update_error());
			}
		} finally {
			saving = false;
		}
	}
</script>

<Dialog.Root bind:open>
	<Dialog.Portal>
		<Dialog.Overlay forceMount>
			{#snippet child({ props, open: isOpen })}
				{#if isOpen}
					<div
						{...props}
						class="fixed inset-0 z-40 bg-overlay"
						transition:fade={{ duration: 200 }}
					></div>
				{/if}
			{/snippet}
		</Dialog.Overlay>
		<Dialog.Content forceMount preventScroll={false}>
			{#snippet child({ props, open: isOpen })}
				{#if isOpen}
					<div
						{...props}
						class="fixed top-1/2 left-1/2 z-50 w-[calc(100%-2.5rem)] max-w-[440px] -translate-x-1/2 -translate-y-1/2 rounded-3xl bg-surface p-7 shadow-dialog"
						transition:scale={{ duration: 200, start: 0.95 }}
					>
						<Dialog.Title class="font-display text-heading font-medium">
							{m.users_reset_title({ username: user?.username ?? '' })}
						</Dialog.Title>
						<Dialog.Description class="mt-2 text-body text-text-muted">
							{m.users_reset_description()}
						</Dialog.Description>
						<form onsubmit={submit} class="mt-5 space-y-4">
							<Input
								id="reset-password"
								label={m.settings_password_new()}
								type="password"
								autocomplete="new-password"
								bind:value={password}
								{error}
							/>
							<div class="flex justify-end gap-3 pt-2">
								<Button variant="ghost" onclick={() => (open = false)}>{m.common_cancel()}</Button>
								<Button type="submit" disabled={saving}>{m.users_reset_submit()}</Button>
							</div>
						</form>
					</div>
				{/if}
			{/snippet}
		</Dialog.Content>
	</Dialog.Portal>
</Dialog.Root>
