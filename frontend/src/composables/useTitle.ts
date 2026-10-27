import {computed, toValue, type MaybeRef, type MaybeRefOrGetter} from 'vue'

import {PRODUCT_NAME} from '@/constants/product'
import {useTitle as useTitleVueUse, type UseTitleOptions, type ReadonlyRefOrGetter} from '@vueuse/core'

export function useTitle(
	newTitle:
		| ReadonlyRefOrGetter<string | null | undefined>
		| MaybeRef<string | null | undefined>
		| MaybeRefOrGetter<string | null | undefined> = null,
	options?: UseTitleOptions,
) {
	const pageTitle = computed(() => toValue(newTitle))

	const completeTitle = computed(() =>
		(typeof pageTitle.value === 'undefined' || pageTitle.value === '')
			? PRODUCT_NAME
			: `${pageTitle.value} | ${PRODUCT_NAME}`,
	)

	return useTitleVueUse(completeTitle, options)
}
