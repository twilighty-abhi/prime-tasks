import {afterEach, describe, expect, it, vi} from 'vitest'
import {ref} from 'vue'
import {enableAutoUnmount, mount, RouterLinkStub} from '@vue/test-utils'

type TeamStub = {
	id: number,
	name: string,
	description?: string,
	members?: {id: number, username: string}[],
}

const teams = ref<TeamStub[]>([])

vi.mock('@/composables/useTeams', () => ({
	useTeamsPage: () => ({teams, totalPages: ref(1), isFetching: ref(false)}),
}))
vi.mock('@/composables/useClampedPage', () => ({useClampedPage: vi.fn()}))
vi.mock('@/composables/useTitle', () => ({useTitle: vi.fn()}))
vi.mock('@vueuse/router', () => ({useRouteQuery: () => ref(1)}))
vi.mock('vue-i18n', async importOriginal => ({
	...await importOriginal<typeof import('vue-i18n')>(),
	useI18n: () => ({t: (key: string) => key}),
}))

import ListTeams from './ListTeams.vue'

enableAutoUnmount(afterEach)

function mountTeams(list: TeamStub[]) {
	teams.value = list
	return mount(ListTeams, {
		global: {
			stubs: {
				RouterLink: RouterLinkStub,
				XButton: {template: '<button class="x-button-stub"><slot /></button>'},
				Pagination: true,
			},
			mocks: {$t: (key: string, params?: unknown) => params === undefined ? key : `${key}:${JSON.stringify(params)}`},
		},
	})
}

const members = (n: number) => Array.from({length: n}, (_, i) => ({id: i + 1, username: `user${i + 1}`}))

// prime-tasks: the teams page is a grid of Yuyu team cards (initial tile, Bricolage name, meta)
// instead of a bare text list.
describe('ListTeams', () => {
	it('renders each team as a card in ul.teams', () => {
		const wrapper = mountTeams([
			{id: 1, name: 'Design', members: members(3)},
			{id: 2, name: 'Engineering', members: members(1)},
		])

		const cards = wrapper.findAll('ul.teams > li.team-card')
		expect(cards).toHaveLength(2)
		expect(cards[0]!.find('.team-card__name').text()).toBe('Design')
		expect(cards[1]!.find('.team-card__name').text()).toBe('Engineering')
	})

	it('shows the first letter of the name in a decorative tile', () => {
		const wrapper = mountTeams([{id: 1, name: 'engineering', members: []}])

		const mark = wrapper.find('.team-card__mark')
		expect(mark.text()).toBe('E')
		expect(mark.attributes('aria-hidden')).toBe('true')
	})

	it('links only the team name to the team, so its accessible name is the name', () => {
		const wrapper = mountTeams([{id: 4, name: 'Design', members: members(2)}])

		const link = wrapper.findComponent(RouterLinkStub)
		expect(link.props('to')).toEqual({name: 'teams.edit', params: {id: 4}})
		expect(link.text()).toBe('Design')
		expect(link.classes()).toContain('team-card__link')
	})

	it('shows the member count and the description as meta', () => {
		const wrapper = mountTeams([
			{id: 1, name: 'Design', description: '<p>Brand and <em>web</em></p>', members: members(3)},
			{id: 2, name: 'Solo', description: '', members: members(1)},
		])

		const cards = wrapper.findAll('.team-card')
		expect(cards[0]!.find('.team-card__members').text()).toBe('team.memberCount:{"count":3}')
		expect(cards[0]!.find('.team-card__description').text()).toBe('Brand and web')
		expect(cards[1]!.find('.team-card__description').exists()).toBe(false)
	})

	it('leaves out the member count when the team has no members list', () => {
		const wrapper = mountTeams([{id: 1, name: 'Design'}])

		expect(wrapper.find('.team-card__members').exists()).toBe(false)
	})
})
