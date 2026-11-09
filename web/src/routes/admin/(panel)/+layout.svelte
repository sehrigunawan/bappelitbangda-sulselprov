<script lang="ts">
	import type { Snippet } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { panelClient } from '#lib/admin/panel-client.ts';
	import type { components } from '#lib/api/schema.d.ts';

	type SessionState = components['schemas']['SessionState'];

	interface Props {
		data: {
			session?: SessionState;
		};
		children?: Snippet;
	}

	let { data, children }: Props = $props();

	const session = $derived(data.session);
	const canManageUsers = $derived(
		session?.permissions?.some((p) => p.area === 'users' && p.level === 'manage') ?? false
	);

	async function handleLogout() {
		try {
			const client = panelClient();
			await client.DELETE('/api/v1/admin/auth/session');
		} finally {
			await goto(resolve('/admin/(masuk)/masuk'));
		}
	}
</script>

<div class="admin-shell">
	<header class="admin-header">
		<div class="brand">
			<a href={resolve('/admin/(panel)')}>Panel Admin</a>
		</div>

		<nav class="admin-nav">
			<a href={resolve('/admin/(panel)')}>Beranda</a>
			<a href={resolve('/admin/(panel)/akun')}>Akun Saya</a>
			{#if canManageUsers}
				<a href={resolve('/admin/(panel)/pengguna')}>Pengguna</a>
			{/if}
		</nav>

		<div class="user-menu">
			{#if session?.user?.fullName}
				<span class="user-name">{session.user.fullName}</span>
			{/if}
			<button type="button" class="logout-button" onclick={handleLogout}> Keluar </button>
		</div>
	</header>

	<main class="admin-content">
		{#if children}
			{@render children()}
		{/if}
	</main>
</div>
