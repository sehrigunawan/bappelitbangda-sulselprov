<script lang="ts">
	import { resolve } from '$app/paths';
	import type { components } from '#lib/api/schema.d.ts';
	import RoleBadge from '#lib/admin/RoleBadge.svelte';
	import { formatDateTimeWita } from '#lib/format/date.ts';
	import { canManageUsers as checkCanManageUsers } from '#lib/admin/permissions.ts';
	type SessionState = components['schemas']['SessionState'];
	type UserPage = components['schemas']['UserPage'];

	interface Props {
		data: {
			session?: SessionState;
			userPage?: UserPage;
			canManage?: boolean;
		};
	}

	let { data }: Props = $props();

	const canManageUsers = $derived(
		data.canManage !== undefined
			? data.canManage
			: data.session
				? checkCanManageUsers(data.session)
				: true
	);

	const userPage = $derived(data.userPage);
	const items = $derived(userPage?.items ?? []);
	const currentPage = $derived(userPage?.page ?? 1);
	const total = $derived(userPage?.total ?? 0);
	const pageSize = $derived(userPage?.pageSize ?? 10);
	const totalPages = $derived(Math.ceil(total / pageSize) || 1);
</script>

{#if !canManageUsers}
	<div class="forbidden-notice">
		<p>Anda tidak memiliki akses ke halaman ini.</p>
	</div>
{:else}
	<div class="users-page">
		<div class="page-header">
			<h2>Daftar Pengguna</h2>
			<a href={resolve('/admin/(panel)/pengguna/baru')} class="add-button">Tambah Pengguna</a>
		</div>

		{#if items.length === 0}
			<p>Belum ada pengguna.</p>
		{:else}
			<table class="users-table">
				<thead>
					<tr>
						<th>Nama pengguna</th>
						<th>Nama lengkap</th>
						<th>Peran</th>
						<th>Status</th>
						<th>Autentikator</th>
						<th>Terakhir masuk</th>
					</tr>
				</thead>
				<tbody>
					{#each items as user (user.id)}
						<tr>
							<td>
								<a href={resolve('/admin/(panel)/pengguna/[id]', { id: String(user.id) })}
									>{user.username}</a
								>
							</td>
							<td>{user.fullName}</td>
							<td>
								{#each user.roles as role (role)}
									<RoleBadge {role} />
								{/each}
							</td>
							<td>
								<span class={user.active ? 'status-active' : 'status-inactive'}>
									{user.active ? 'Aktif' : 'Nonaktif'}
								</span>
							</td>
							<td>
								{user.totpEnrolled ? 'Terdaftar' : 'Belum terdaftar'}
							</td>
							<td>
								{user.lastLoginAt ? formatDateTimeWita(user.lastLoginAt) : '—'}
							</td>
						</tr>
					{/each}
				</tbody>
			</table>

			{#if total > 0}
				<div class="pagination">
					{#if currentPage <= 1}
						<span aria-disabled="true" class="page-link disabled">Sebelumnya</span>
					{:else}
						<a
							href={resolve(
								`/admin/(panel)/pengguna?halaman=${currentPage - 1}` as '/admin/(panel)/pengguna'
							)}
							class="page-link"
						>
							Sebelumnya
						</a>
					{/if}
					<span class="page-info">Halaman {currentPage} dari {totalPages}</span>
					{#if currentPage >= totalPages}
						<span aria-disabled="true" class="page-link disabled">Berikutnya</span>
					{:else}
						<a
							href={resolve(
								`/admin/(panel)/pengguna?halaman=${currentPage + 1}` as '/admin/(panel)/pengguna'
							)}
							class="page-link"
						>
							Berikutnya
						</a>
					{/if}
				</div>
			{/if}
		{/if}
	</div>
{/if}
