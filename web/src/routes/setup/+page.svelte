<script lang="ts">
	import { goto, invalidate } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { setup } from '#lib/api/auth.js';
	import { ApiError } from '#lib/api/client.js';
	import Button from '#lib/components/Button.svelte';
	import CardPage from '#lib/components/CardPage.svelte';
	import Field from '#lib/components/Field.svelte';

	let username = $state('');
	let password = $state('');
	let confirm = $state('');
	let error = $state('');
	let busy = $state(false);

	// The server checks these too; checking here gives the message sooner.
	const mismatch = $derived(confirm !== '' && confirm !== password);

	async function submit(event: SubmitEvent) {
		event.preventDefault();
		if (password !== confirm) {
			error = "The passwords don't match.";
			return;
		}
		busy = true;
		error = '';
		try {
			await setup(username, password);
			await goto(resolve('/(app)'), { invalidate: ['app:session'] });
		} catch (e) {
			if (e instanceof ApiError && e.code === 'setup_done') {
				// Finished in another tab or by someone else: move on to sign-in.
				await invalidate('app:session');
				return;
			}
			error = e instanceof ApiError ? e.message : 'Something went wrong.';
		} finally {
			busy = false;
		}
	}
</script>

<svelte:head>
	<title>Set up · watcht</title>
</svelte:head>

<CardPage
	title="Welcome to watcht"
	intro="Create the admin account. You can add the rest of the household afterwards."
>
	<form onsubmit={submit} class="flex flex-col gap-4">
		<Field
			label="Username"
			name="username"
			autocomplete="username"
			autocapitalize="none"
			spellcheck="false"
			required
			maxlength={64}
			pattern="[A-Za-z0-9._\-]+"
			title="Letters, digits, '.', '_' or '-'"
			hint="Letters, digits, dots, hyphens and underscores."
			bind:value={username}
		/>
		<Field
			label="Password"
			name="password"
			type="password"
			autocomplete="new-password"
			required
			minlength={8}
			hint="At least 8 characters."
			bind:value={password}
		/>
		<Field
			label="Confirm password"
			name="confirm"
			type="password"
			autocomplete="new-password"
			required
			error={mismatch ? "The passwords don't match." : undefined}
			bind:value={confirm}
		/>
		{#if error}
			<p
				role="alert"
				class="rounded-md border border-danger/40 bg-danger/10 px-3 py-2 text-sm text-danger first-letter:uppercase"
			>
				{error}
			</p>
		{/if}
		<Button type="submit" {busy} class="mt-1 w-full">
			{busy ? 'Creating…' : 'Create admin account'}
		</Button>
	</form>
</CardPage>
