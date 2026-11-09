/**
 * Formats an ISO 8601 UTC timestamp into Indonesian WITA datetime string.
 * Example: '2026-10-15T06:05:00Z' -> 'Kamis, 15 Oktober 2026 14.05 WITA'
 */
export function formatDateTimeWita(iso: string): string {
	const date = new Date(iso);
	const dtf = new Intl.DateTimeFormat('id-ID', {
		timeZone: 'Asia/Makassar',
		weekday: 'long',
		day: 'numeric',
		month: 'long',
		year: 'numeric',
		hour: '2-digit',
		minute: '2-digit',
		hourCycle: 'h23'
	});

	const parts = dtf.formatToParts(date);
	let weekday = '';
	let day = '';
	let month = '';
	let year = '';
	let hour = '';
	let minute = '';

	for (const part of parts) {
		switch (part.type) {
			case 'weekday':
				weekday = part.value;
				break;
			case 'day':
				day = part.value;
				break;
			case 'month':
				month = part.value;
				break;
			case 'year':
				year = part.value;
				break;
			case 'hour':
				hour = part.value;
				break;
			case 'minute':
				minute = part.value;
				break;
		}
	}

	return `${weekday}, ${day} ${month} ${year} ${hour}.${minute} WITA`;
}
