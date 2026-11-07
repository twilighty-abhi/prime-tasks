import {describe, it, expect, vi} from 'vitest'
import {mount} from '@vue/test-utils'

// The real ones reach into vue-query and the API.
vi.mock('@/composables/useProjectBackground', () => ({
	useProjectBackground: () => ({background: {value: null}, blurHashUrl: {value: ''}}),
}))
vi.mock('@/client/queries/projects', () => ({
	usePatchProjectFavoriteMutation: () => ({mutateAsync: vi.fn()}),
}))
vi.mock('@/helpers/getProjectTitle', () => ({
	getProjectTitle: (project: {title: string}) => project.title,
}))

import ProjectCard from './ProjectCard.vue'
import type {ProjectResponse} from '@/client/queries/projects'

function project(overrides: Partial<ProjectResponse> = {}): ProjectResponse {
	return {
		id: 9,
		title: 'Q4 grant report',
		description: '',
		hex_color: '#e8710a',
		is_archived: false,
		is_favorite: false,
		max_permission: 2,
		...overrides,
	} as ProjectResponse
}

function mountCard(p: ProjectResponse) {
	return mount(ProjectCard, {
		props: {project: p},
		global: {
			stubs: {
				Icon: {props: ['icon'], template: '<i class="icon-stub" :data-icon="String(icon)" />'},
				BaseButton: {template: '<a><slot /></a>'},
				ProjectSettingsDropdown: true,
			},
			mocks: {$t: (key: string) => key},
		},
	})
}

// prime-tasks: project cards in the Yuyu event-card style: a colour band with a dot grid, an initial tile
// overlapping it, a Bricolage title and Shared / Archived chips. No coloured start border.
describe('ProjectCard', () => {
	it('shows the project colour as the band, not as a start border', () => {
		const wrapper = mountCard(project())

		const card = wrapper.find('.project-card')
		expect(card.attributes('style') ?? '').not.toContain('border-inline-start')

		const band = wrapper.find('.project-card__band')
		expect(band.exists()).toBe(true)
		expect(band.attributes('style')).toContain('background-color: #e8710a')
		expect(band.classes()).not.toContain('is-neutral')
	})

	it('accepts a colour stored without the leading #', () => {
		const band = mountCard(project({hex_color: '1973ff'})).find('.project-card__band')
		expect(band.attributes('style')).toContain('background-color: #1973ff')
	})

	it('uses the neutral ink band when the project has no colour', () => {
		const band = mountCard(project({title: 'Inbox', hex_color: ''})).find('.project-card__band')
		expect(band.classes()).toContain('is-neutral')
		expect(band.attributes('style') ?? '').not.toContain('background-color')
	})

	it('shows the first letter in the initial tile', () => {
		const mark = mountCard(project()).find('.project-card__mark')
		expect(mark.text()).toBe('Q')
		expect(mark.attributes('aria-hidden')).toBe('true')
		expect(mountCard(project({title: 'Inbox', hex_color: ''})).find('.project-card__mark').text()).toBe('I')
	})

	it('uses the first letter, not leading space or half an emoji', () => {
		expect(mountCard(project({title: '  émigré stories'})).find('.project-card__mark').text()).toBe('É')
		expect(mountCard(project({title: '🚀 Launch'})).find('.project-card__mark').text()).toBe('🚀')
	})

	it('shows the filter icon instead of a letter for a saved filter', () => {
		const mark = mountCard(project({id: -3, title: 'Overdue'})).find('.project-card__mark')
		expect(mark.find('.icon-stub').attributes('data-icon')).toBe('filter')
		expect(mark.text()).toBe('')
	})

	it('keeps the title as its own element', () => {
		expect(mountCard(project()).find('.project-title').text()).toBe('Q4 grant report')
	})

	it('shows Shared and Archived chips only when they apply', () => {
		expect(mountCard(project()).findAll('.project-card__chip')).toHaveLength(0)

		const shared = mountCard(project({shared_via: {type: 'direct'}} as Partial<ProjectResponse>))
		expect(shared.findAll('.project-card__chip').map(c => c.text())).toEqual(['project.sharedBadge'])

		const archived = mountCard(project({is_archived: true}))
		expect(archived.findAll('.project-card__chip').map(c => c.text())).toEqual(['project.archived'])
	})
})
