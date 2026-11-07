<template>
	<p
		v-if="isLive"
		v-cy="'taskClaimNotice'"
		class="task-claim"
		role="status"
	>
		<Icon
			icon="clock"
			class="task-claim__icon"
		/>
		<i18n-t
			keypath="task.detail.claimedBy"
			scope="global"
		>
			<template #bot>
				<strong
					class="task-claim__bot"
					:title="claim.bot?.username"
				>{{ botName }}</strong>
			</template>
			<template #until>
				<time
					v-tooltip="formatDateLong(claim.expires_at)"
					:datetime="formatISO(claim.expires_at)"
				>{{ formatDateShort(claim.expires_at) }}</time>
			</template>
		</i18n-t>
	</p>
</template>

<script setup lang="ts">
import {computed} from 'vue'

import type {TaskClaim} from '@/client/generated'
import {useGlobalNow} from '@/composables/useGlobalNow'
import {formatDateLong, formatDateShort, formatISO} from '@/helpers/time/formatDate'
import {getDisplayName} from '@/helpers/user'

const props = defineProps<{
	claim: TaskClaim,
}>()

const {now} = useGlobalNow()

// The server only sends live claims, but an open task outlives the lease.
const isLive = computed(() => new Date(props.claim.expires_at ?? 0).getTime() > now.value.getTime())
const botName = computed(() => getDisplayName(props.claim.bot) || `#${props.claim.bot_id}`)
</script>

<style lang="scss" scoped>
.task-claim {
	display: inline-flex;
	align-items: center;
	gap: var(--yy-space-2);
	margin-block: var(--yy-space-2) 0;
	padding: var(--yy-space-1) var(--yy-space-3);
	border-radius: var(--yy-radius-full);
	background: var(--yy-warning-bg);
	color: var(--yy-warning-fg);
	font-size: .875rem;
	line-height: 1.4;
}

.task-claim__bot {
	color: inherit;
	font-weight: 600;
}
</style>
