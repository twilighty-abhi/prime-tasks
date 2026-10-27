import {describe, expect, it} from 'vitest'

import {hasSharedBadge, isSharedWithMe, isSidebarProjectFilter, sharedViaDescription, splitRootProjects} from './projectSharing'

const own = {id: 1, shared_via: undefined}
const direct = {id: 2, shared_via: {type: 'direct' as const, permission: 1}, owner_display_name: 'Priya'}
const team = {id: 3, shared_via: {type: 'team' as const, permission: 0, team_name: 'Design'}, owner_display_name: 'Priya'}
const parent = {id: 4, shared_via: {type: 'parent' as const, permission: 0, parent_project_title: 'Website'}, owner_display_name: ''}

const t = (key: string, values?: Record<string, unknown>) => `${key} ${JSON.stringify(values ?? {})}`

describe('projectSharing', () => {
	it('tells shared projects from your own', () => {
		expect(isSharedWithMe(own)).toBe(false)
		expect(isSharedWithMe({shared_via: null} as never)).toBe(false)
		expect(isSharedWithMe(direct)).toBe(true)
		expect(isSharedWithMe(parent)).toBe(true)
	})

	it('badges direct and team shares, not projects that come with a shared parent', () => {
		expect(hasSharedBadge(direct)).toBe(true)
		expect(hasSharedBadge(team)).toBe(true)
		expect(hasSharedBadge(parent)).toBe(false)
		expect(hasSharedBadge(own)).toBe(false)
	})

	it('splits roots for each filter, keeping their order', () => {
		const roots = [direct, own, team]
		expect(splitRootProjects(roots, 'all')).toEqual({personal: [own], shared: [direct, team]})
		expect(splitRootProjects(roots, 'personal')).toEqual({personal: [own], shared: []})
		expect(splitRootProjects(roots, 'shared')).toEqual({personal: [], shared: [direct, team]})
	})

	it('validates a stored filter', () => {
		expect(isSidebarProjectFilter('shared')).toBe(true)
		expect(isSidebarProjectFilter('everything')).toBe(false)
	})

	it('describes where the access comes from', () => {
		expect(sharedViaDescription(direct, t)).toBe('project.sharedVia.direct {"owner":"Priya"}')
		expect(sharedViaDescription(team, t)).toBe('project.sharedVia.team {"owner":"Priya","team":"Design"}')
		expect(sharedViaDescription(parent, t)).toBe('project.sharedVia.parent {"owner":"project.sharedVia.someone {}","parent":"Website"}')
		expect(sharedViaDescription(own, t)).toBe('')
	})
})
