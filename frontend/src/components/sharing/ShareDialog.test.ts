import {flushPromises, mount} from '@vue/test-utils'
import {QueryClient, VueQueryPlugin} from '@tanstack/vue-query'
import {createI18n} from 'vue-i18n'
import {ref} from 'vue'
import {beforeEach, describe, expect, it, vi} from 'vitest'

import enMessages from '@/i18n/lang/en.json'

const sdk = vi.hoisted(() => ({
	projectAccessList: vi.fn(),
	projectUsersCreate: vi.fn(),
	projectTeamsCreate: vi.fn(),
	projectUsersUpdate: vi.fn(),
	projectTeamsUpdate: vi.fn(),
	projectUsersDelete: vi.fn(),
	projectTeamsDelete: vi.fn(),
	projectUsersList: vi.fn(),
	projectTeamsList: vi.fn(),
	projectSharesLeave: vi.fn(),
}))
vi.mock('@/client/generated', () => sdk)
vi.mock('@/message', () => ({success: vi.fn(), error: vi.fn()}))
vi.mock('@/stores/auth', () => ({useAuthStore: () => ({info: {id: 1, username: 'me'}})}))
vi.mock('@/stores/config', () => ({useConfigStore: () => ({public_teams_enabled: false})}))
const searchResults = vi.hoisted(() => ({users: [] as unknown[], teams: [] as unknown[]}))
vi.mock('@/composables/useUserSearch', () => ({useUserSearch: () => ({users: ref(searchResults.users)})}))
vi.mock('@/composables/useTeams', () => ({useTeamSearch: () => ({teams: ref(searchResults.teams)})}))

import ShareDialog from './ShareDialog.vue'

const access = [
	{kind: 'user', source: 'owner', permission: 2, user: {id: 1, username: 'me', name: 'Me'}},
	{kind: 'user', source: 'direct', permission: 1, user: {id: 2, username: 'priya', name: 'Priya Raman'}},
	{kind: 'team', source: 'direct', permission: 0, team: {id: 5, name: 'Design'}},
	{kind: 'user', source: 'parent', permission: 2, user: {id: 1, username: 'me', name: 'Me'}, via_project_id: 9, via_project_title: 'Website'},
	{kind: 'user', source: 'parent', permission: 0, user: {id: 3, username: 'arjun', name: 'Arjun'}, via_project_id: 9, via_project_title: 'Website'},
]

async function mountDialog(userIsAdmin: boolean) {
	const i18n = createI18n({legacy: false, locale: 'en', messages: {en: enMessages}})
	const wrapper = mount(ShareDialog, {
		props: {projectId: 7, userIsAdmin},
		global: {
			plugins: [i18n, [VueQueryPlugin, {queryClient: new QueryClient({defaultOptions: {queries: {retry: false}}})}]],
			stubs: {
				User: {props: ['user'], template: '<span class="user">{{ user.name }}</span>'},
				XButton: {template: '<button type="button" v-bind="$attrs"><slot /></button>'},
				Loading: true,
			},
			directives: {cy: () => {}},
		},
	})
	await flushPromises()
	return wrapper
}

describe('ShareDialog', () => {
	beforeEach(() => {
		vi.resetAllMocks()
		searchResults.users = []
		searchResults.teams = []
		sdk.projectAccessList.mockResolvedValue({data: {items: access}})
	})

	it('lists everyone with access, with where it comes from, each person once', async () => {
		const wrapper = await mountDialog(true)
		const rows = wrapper.findAll('.share-dialog__row').map(row => row.text())
		expect(rows).toHaveLength(4)
		expect(rows[0]).toContain('Me')
		expect(rows[0]).toContain('Owner')
		expect(rows[1]).toContain('Priya Raman')
		expect(rows[1]).toContain('Direct')
		expect(rows[2]).toContain('Design')
		expect(rows[2]).toContain('Team')
		expect(rows[3]).toContain('Arjun')
		expect(rows[3]).toContain('From "Website"')
	})

	it('lets admins change and remove only this project\'s own shares', async () => {
		const wrapper = await mountDialog(true)
		const rows = wrapper.findAll('.share-dialog__row')
		expect(rows.map(row => row.find('.share-dialog__remove').exists())).toEqual([false, true, true, false])

		sdk.projectUsersDelete.mockResolvedValue({})
		await rows[1]!.find('.share-dialog__remove').trigger('click')
		const confirm = wrapper.findAll('.share-dialog__confirm button').at(-1)!
		await confirm.trigger('click')
		await flushPromises()
		expect(sdk.projectUsersDelete).toHaveBeenCalledWith({path: {project: 7, user: 'priya'}})
	})

	it('shows the list read-only to non-admins, without the search', async () => {
		const wrapper = await mountDialog(false)
		expect(wrapper.find('input[type="search"]').exists()).toBe(false)
		expect(wrapper.find('.share-dialog__remove').exists()).toBe(false)
		expect(wrapper.findAll('.share-dialog__row')).toHaveLength(4)
	})

	it('searches people and teams together and shares with the role picked inline', async () => {
		searchResults.users = [{id: 2, username: 'priya', name: 'Priya Raman'}, {id: 4, username: 'meera', name: 'Meera Iyer'}]
		searchResults.teams = [{id: 5, name: 'Design'}, {id: 6, name: 'Engineering'}]
		sdk.projectTeamsCreate.mockResolvedValue({data: {}})
		const wrapper = await mountDialog(true)
		// Pick the option itself: happy-dom's select.value setter only matches value attributes, and
		// Vue binds :value as a property.
		await wrapper.find('select').findAll('option')[2]!.setSelected()
		await wrapper.find('input[type="search"]').setValue('e')
		const results = wrapper.findAll('.share-dialog__result').map(result => result.text())
		// already shared directly: Priya and Design are not offered again
		expect(results).toEqual([expect.stringContaining('Meera Iyer'), expect.stringContaining('Engineering')])

		await wrapper.findAll('.share-dialog__result')[1]!.trigger('click')
		await flushPromises()
		expect(sdk.projectTeamsCreate).toHaveBeenCalledWith({path: {project: 7}, body: {team_id: 6, permission: 2}})
	})
})
