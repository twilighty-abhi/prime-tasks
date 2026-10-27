// prime-tasks F3: which projects are shared with the current user, for the sidebar.
import type {ProjectResponse} from '@/client/queries/projects'

export const SIDEBAR_PROJECT_FILTERS = ['all', 'personal', 'shared'] as const
export type SidebarProjectFilter = typeof SIDEBAR_PROJECT_FILTERS[number]

export function isSidebarProjectFilter(value: unknown): value is SidebarProjectFilter {
	return SIDEBAR_PROJECT_FILTERS.includes(value as SidebarProjectFilter)
}

// shared_via is set by /api/v2 only for projects outside the caller's own tree.
export function isSharedWithMe(project: Pick<ProjectResponse, 'shared_via'>): boolean {
	return project.shared_via !== undefined && project.shared_via !== null
}

// Projects shared with you through a parent sit under that parent; only the shared root gets a badge.
export function hasSharedBadge(project: Pick<ProjectResponse, 'shared_via'>): boolean {
	return isSharedWithMe(project) && project.shared_via?.type !== 'parent'
}

export function splitRootProjects<T extends Pick<ProjectResponse, 'shared_via'>>(roots: T[], filter: SidebarProjectFilter) {
	return {
		personal: filter === 'shared' ? [] : roots.filter(project => !isSharedWithMe(project)),
		shared: filter === 'personal' ? [] : roots.filter(isSharedWithMe),
	}
}

type Translate = (key: string, values?: Record<string, unknown>) => string

// "Shared by Priya with the team Design", for the badge's tooltip and accessible label.
export function sharedViaDescription(project: Pick<ProjectResponse, 'shared_via' | 'owner_display_name'>, t: Translate): string {
	const via = project.shared_via
	if (!via) {
		return ''
	}
	const owner = project.owner_display_name || t('project.sharedVia.someone')
	switch (via.type) {
		case 'team':
			return t('project.sharedVia.team', {owner, team: via.team_name ?? ''})
		case 'parent':
			return t('project.sharedVia.parent', {owner, parent: via.parent_project_title ?? ''})
		default:
			return t('project.sharedVia.direct', {owner})
	}
}
