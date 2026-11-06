import {describe, expect, it} from 'vitest'

import {hasSharedBadge, isSharedWithMe, isSidebarProjectFilter, leaveBlocker, sharedViaDescription, splitRootProjects} from './projectSharing'

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

	describe('leaveBlocker', () => {
		it('lets a member with a direct share leave, even when a team also gives access', () => {
			expect(leaveBlocker({shared_via: {type: 'team', permission: 1, team_id: 5, team_name: 'Design'}}, true)).toBeNull()
		})
		it('names the team when the access comes only from a team', () => {
			expect(leaveBlocker({shared_via: {type: 'team', permission: 0, team_id: 5, team_name: 'Design'}}, false))
				.toEqual({kind: 'team', teamId: 5, teamName: 'Design'})
		})
		it('names the team when a parent project is shared with one of your teams', () => {
			expect(leaveBlocker({shared_via: {type: 'parent', permission: 0, team_id: 6, team_name: 'Ops', parent_project_id: 9, parent_project_title: 'Website'}}, false))
				.toEqual({kind: 'team', teamId: 6, teamName: 'Ops'})
		})
		it('names the parent project when that is shared with you directly', () => {
			expect(leaveBlocker({shared_via: {type: 'parent', permission: 0, parent_project_id: 9, parent_project_title: 'Website'}}, false))
				.toEqual({kind: 'parent', projectId: 9, projectTitle: 'Website'})
		})
		it('leaves the decision to the server when it can\'t tell', () => {
			expect(leaveBlocker({shared_via: undefined}, false)).toBeNull()
			expect(leaveBlocker({shared_via: {type: 'direct', permission: 0}}, false)).toBeNull()
		})
	})
})
