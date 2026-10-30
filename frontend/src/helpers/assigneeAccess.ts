// prime-tasks F2: what picking someone in the assignee picker does.
import {PERMISSIONS} from '@/constants/permissions'

export type AssigneeAction =
	| 'assign'        // they can see the project already
	| 'share-assign'  // ask, then share the project read & write and assign in one call
	| 'admin-needed'  // only a project admin can share it; never share from here

export function assigneeAction(hasProjectAccess: boolean, callerPermission: number | null | undefined): AssigneeAction {
	if (hasProjectAccess) {
		return 'assign'
	}
	return callerPermission === PERMISSIONS.ADMIN ? 'share-assign' : 'admin-needed'
}
