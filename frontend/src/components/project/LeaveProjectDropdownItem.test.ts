import {flushPromises, mount} from '@vue/test-utils'
import {QueryClient, VueQueryPlugin} from '@tanstack/vue-query'
import {beforeEach, describe, expect, it, vi} from 'vitest'

const sdk = vi.hoisted(() => ({projectUsersList: vi.fn()}))
vi.mock('@/client/generated', () => sdk)
vi.mock('@/stores/auth', () => ({
	useAuthStore: () => ({info: {id: 2, username: 'me'}}),
}))

import LeaveProjectDropdownItem from './LeaveProjectDropdownItem.vue'

async function mountItem(props: {projectId: number, ownerId?: number}) {
	const wrapper = mount(LeaveProjectDropdownItem, {
		props,
		global: {
			plugins: [[VueQueryPlugin, {queryClient: new QueryClient({defaultOptions: {queries: {retry: false}}})}]],
			stubs: {DropdownItem: {template: '<a class="leave"><slot /></a>'}},
			directives: {cy: () => {}},
			mocks: {$t: (key: string) => key},
		},
	})
	await flushPromises()
	return wrapper
}

describe('LeaveProjectDropdownItem', () => {
	beforeEach(() => vi.resetAllMocks())

	it('shows for a user with a direct share', async () => {
		sdk.projectUsersList.mockResolvedValue({data: {items: [{username: 'someone'}, {username: 'me'}], total_pages: 1}})
		const wrapper = await mountItem({projectId: 7, ownerId: 1})
		expect(wrapper.find('.leave').exists()).toBe(true)
	})

	it('hides when the access is not a direct share', async () => {
		sdk.projectUsersList.mockResolvedValue({data: {items: [{username: 'someone'}], total_pages: 1}})
		const wrapper = await mountItem({projectId: 7, ownerId: 1})
		expect(wrapper.find('.leave').exists()).toBe(false)
	})

	it('hides for the owner without asking the server', async () => {
		const wrapper = await mountItem({projectId: 7, ownerId: 2})
		expect(wrapper.find('.leave').exists()).toBe(false)
		expect(sdk.projectUsersList).not.toHaveBeenCalled()
	})
})
