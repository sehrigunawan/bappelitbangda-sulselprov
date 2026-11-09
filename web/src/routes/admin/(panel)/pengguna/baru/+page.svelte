<script lang="ts">
	import { resolve } from '$app/paths';
	import { panelClient } from '#lib/admin/panel-client.ts';
	import type { components } from '#lib/api/schema.d.ts';
	import { canManageUsers } from '#lib/admin/permissions.ts';

	type Role = components['schemas']['Role'];
	type UserCreated = components['schemas']['UserCreated'];

	interface Props {
		data: {
			createdResult?: UserCreated | null;
			session?: components['schemas']['SessionState'];
			canManage?: boolean;
		};
	}

	let { data }: Props = $props();

	const canManage = $derived(
		data.canManage !== undefined
			? data.canManage
			: data.session
				? canManageUsers(data.session)
				: true
	);
	let username = $state('');
	let fullName = $state('');
	let selectedRoles = $state<Role[]>(['ppid_officer']);
	let loading = $state(false);
	let errorMessage = $state('');
	let manualCreatedResult = $state<UserCreated | null>(null);
	let createdResult = $derived(manualCreatedResult ?? data.createdResult ?? null);

	const AVAILABLE_ROLES: { role: Role; label: string }[] = [
		{ role: 'super_admin', label: 'Super Admin' },
		{ role: 'humas_editor', label: 'Editor Humas' },
		{ role: 'document_operator', label: 'Operator Dokumen' },
		{ role: 'ppid_officer', label: 'Petugas PPID' },
		{ role: 'leadership', label: 'Pimpinan' }
	];

	function toggleRole(role: Role) {
		if (selectedRoles.includes(role)) {
			if (selectedRoles.length > 1) {
				selectedRoles = selectedRoles.filter((r) => r !== role);
			}
		} else {
			selectedRoles = [...selectedRoles, role];
		}
	}

	async function handleSubmit(e: SubmitEvent) {
		e.preventDefault();
		loading = true;
		errorMessage = '';

		try {
			const client = panelClient();
			const { data: res, response } = await client.POST('/api/v1/admin/auth/users', {
				body: {
					username,
					fullName,
					roles: selectedRoles
				}
			});

			if (response.ok && res) {
				manualCreatedResult = res;
				username = '';
				fullName = '';
			} else if (response.status === 409) {
				errorMessage = 'Nama pengguna sudah dipakai.';
			} else {
				errorMessage = 'Terjadi kesalahan. Coba lagi.';
			}
		} catch {
			errorMessage = 'Terjadi kesalahan. Coba lagi.';
		} finally {
			loading = false;
		}
	}
</script>

<div class="user-create-page">
	<h2>Tambah Pengguna</h2>

	{#if !canManage}
		<p>Anda tidak memiliki akses ke halaman ini.</p>
	{:else}
		{#if createdResult}
			<div class="success-banner">
				<p>
					Kata sandi sementara: {createdResult.temporaryPassword}. Berikan langsung kepada pengguna;
					kata sandi ini tidak akan ditampilkan lagi.
				</p>
				<a href={resolve('/admin/(panel)/pengguna')} class="btn-back">Kembali ke Daftar Pengguna</a>
			</div>
		{/if}

		{#if errorMessage}
			<div class="error-alert">{errorMessage}</div>
		{/if}

		<form onsubmit={handleSubmit}>
			<label>
				Nama pengguna
				<input
					type="text"
					name="username"
					pattern={'[a-z0-9._\\-]{3,32}'}
					required
					bind:value={username}
					disabled={loading}
				/>
			</label>

			<label>
				Nama lengkap
				<input
					type="text"
					name="fullName"
					maxlength="100"
					required
					bind:value={fullName}
					disabled={loading}
				/>
			</label>

			<fieldset class="roles-fieldset">
				<legend>Peran</legend>
				{#each AVAILABLE_ROLES as { role, label } (role)}
					<label class="role-checkbox">
						<input
							type="checkbox"
							value={role}
							checked={selectedRoles.includes(role)}
							onchange={() => toggleRole(role)}
							disabled={loading}
						/>
						{label}
					</label>
				{/each}
			</fieldset>

			<div class="form-actions">
				<a href={resolve('/admin/(panel)/pengguna')}>Batal</a>
				<button type="submit" disabled={loading || selectedRoles.length === 0}>
					{loading ? 'Memuat...' : 'Simpan'}
				</button>
			</div>
		</form>
	{/if}
</div>
