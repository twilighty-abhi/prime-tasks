import {describe, it, expect, vi} from 'vitest'
import {mount} from '@vue/test-utils'

// The real ones reach into the auth store for the date display settings.
vi.mock('@/helpers/time/formatDate', () => ({
	formatDateLong: (date: string) => `long ${date}`,
	formatDateShort: (date: string) => `short ${date}`,
	formatISO: (date: string) => date,
}))

import TaskClaimNotice from './TaskClaimNotice.vue'
import type {TaskClaim} from '@/client/generated'

function claimExpiringIn(ms: number, botName = 'Review Bot'): TaskClaim {
	return {
		task_id: 1,
		bot_id: 23,
		claimed_at: new Date(Date.now() - 60_000).toISOString(),
		expires_at: new Date(Date.now() + ms).toISOString(),
		bot: {
			id: 23,
			username: 'bot-reviewer',
			name: botName,
		},
	}
}

function mountNotice(claim: TaskClaim) {
	return mount(TaskClaimNotice, {
		props: {claim},
		global: {
			directives: {
				tooltip: {},
				cy: {},
			},
			stubs: {
				Icon: true,
				'i18n-t': {template: '<span><slot name="bot" /> until <slot name="until" /></span>'},
			},
		},
	})
}

describe('TaskClaimNotice', () => {
	it('names the bot holding the claim and when it expires', () => {
		const claim = claimExpiringIn(2 * 60 * 60 * 1000)
		const wrapper = mountNotice(claim)

		const notice = wrapper.find('.task-claim')
		expect(notice.exists()).toBe(true)
		expect(notice.attributes('role')).toBe('status')
		expect(wrapper.find('.task-claim__bot').text()).toBe('Review Bot')
		expect(wrapper.find('.task-claim__bot').attributes('title')).toBe('bot-reviewer')
		const time = wrapper.find('time')
		expect(time.attributes('datetime')).toBe(claim.expires_at)
		expect(time.text()).toBe(`short ${claim.expires_at}`)
	})

	it('falls back to the username when the bot has no name', () => {
		const wrapper = mountNotice(claimExpiringIn(60 * 60 * 1000, ''))

		expect(wrapper.find('.task-claim__bot').text()).toBe('bot-reviewer')
	})

	it('shows nothing once the lease has run out', () => {
		const wrapper = mountNotice(claimExpiringIn(-60_000))

		expect(wrapper.find('.task-claim').exists()).toBe(false)
	})
})
