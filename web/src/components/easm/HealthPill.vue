<template>
  <div class="health">
    <span v-for="item in items" :key="item.label" :class="['pill', item.ok ? 'ok' : 'bad']">
      {{ item.label }} · {{ item.ok ? 'ok' : item.value }}
    </span>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { Health } from './types'

const props = defineProps<{ health: Health }>()
const items = computed(() => [
  { label: 'ICP', value: props.health.chinaz, ok: props.health.chinaz === 'ok' },
  { label: '反查', value: props.health.icpQuery, ok: props.health.icpQuery === 'ok' },
  { label: 'GeoIP', value: props.health.geoip, ok: props.health.geoip === 'ok' },
  { label: '图谱', value: props.health.neo4j, ok: props.health.neo4j === 'ok' },
])
</script>

<style scoped>
.health { display: flex; gap: 6px; flex-wrap: wrap; }
.pill { font-size: 11px; padding: 3px 8px; border-radius: 999px; border: 1px solid #1d3f66; color: #8fb4ff; }
.pill.ok { color: #7ef0b2; border-color: #1d6b4a; background: rgba(30, 120, 80, .15); }
.pill.bad { color: #ffb3c1; border-color: #7a2a3a; background: rgba(160, 40, 60, .15); }
</style>
