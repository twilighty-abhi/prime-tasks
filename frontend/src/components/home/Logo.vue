<script setup lang="ts">
import { computed } from 'vue'
import { useColorScheme } from '@/composables/useColorScheme'

import {PRODUCT_NAME} from '@/constants/product'

const { isDark } = useColorScheme()

const CustomLogo = computed(() => {
	const lightLogo = window.CUSTOM_LOGO_URL
	const darkLogo = window.CUSTOM_LOGO_URL_DARK

	if (!lightLogo && !darkLogo) return ''
	if (!darkLogo) return lightLogo
	if (!lightLogo) return darkLogo

	return isDark.value ? darkLogo : lightLogo
})
</script>

<template>
	<div>
		<!-- prime-tasks Y1: a wordmark instead of the Vikunja logo; service.customlogourl still replaces it -->
		<span
			v-if="!CustomLogo"
			class="logo logo-wordmark"
		>{{ PRODUCT_NAME }}</span>
		<img
			v-else
			:src="CustomLogo"
			:alt="PRODUCT_NAME"
			class="logo"
		>
	</div>
</template>

<style lang="scss" scoped>
.logo {
	color: var(--logo-text-color);
	max-inline-size: 168px;
	max-block-size: 48px;
}

.logo-wordmark {
	display: inline-block;
	font-family: var(--yy-font-display);
	font-size: 22px;
	font-weight: 800;
	letter-spacing: -0.02em;
	line-height: 48px;
	white-space: nowrap;
}
</style>
