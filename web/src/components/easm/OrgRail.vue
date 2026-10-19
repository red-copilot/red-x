<template>
  <aside class="rail">
    <div class="rail-head"><span>组织</span><span class="count">{{ orgs.length }}</span></div>
    <input v-model="filter" class="rail-search" placeholder="筛选组织" />
    <div class="org-list">
      <button
        v-for="org in filtered"
        :key="org.id"
        :class="['org', { active: org.id === selectedId }]"
        @click="$emit('select', org.id)"
      >
        <i class="dot" :class="riskOf(org.id)" />
        <span class="name">{{ org.canonicalName }}</span>
        <span class="badge">{{ countOf(org.id) }}</span>
      </button>
      <div v-if="!filtered.length" class="empty">暂无组织</div>
    </div>
  </aside>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import type { OrgBubble, Organization } from './types'

const props = defineProps<{ orgs: Organization[]; selectedId: string; bubbles: OrgBubble[] }>()
defineEmits<{ (e: 'select', id: string): void }>()
const filter = ref('')

const filtered = computed(() => {
  const q = filter.value.trim().toLowerCase()
  if (!q) return props.orgs
  return props.orgs.filter((o) => `${o.canonicalName} ${(o.aliases || []).join(' ')}`.toLowerCase().includes(q))
})
const bubbleMap = computed(() => new Map(props.bubbles.map((b) => [b.orgId, b])))
function countOf(id: string) { return bubbleMap.value.get(id)?.assetCount ?? 0 }
function riskOf(id: string) { return bubbleMap.value.get(id)?.risk || 'low' }
</script>

<style scoped>
.rail { width: 250px; flex: 0 0 250px; border-right: 1px solid #12294a; background: #081226; display: flex; flex-direction: column; min-height: 0; }
.rail-head { display: flex; justify-content: space-between; padding: 14px 16px 8px; font-size: 12px; letter-spacing: .12em; color: #8fb4ff; }
.count { color: #5f7aa8; }
.rail-search { margin: 0 12px 10px; background: #0b1830; border: 1px solid #1d3f66; border-radius: 6px; color: #cfe0ff; padding: 7px 10px; outline: none; }
.org-list { overflow: auto; flex: 1; padding: 0 8px 12px; }
.org { width: 100%; display: flex; align-items: center; gap: 8px; text-align: left; background: transparent; border: 1px solid transparent; color: #cfe0ff; padding: 8px 8px; border-radius: 6px; cursor: pointer; }
.org:hover { background: rgba(61, 123, 255, .08); }
.org.active { background: rgba(61, 123, 255, .18); border-color: #2b5fae; }
.dot { width: 7px; height: 7px; border-radius: 50%; background: #4fc3ff; flex: 0 0 auto; }
.dot.critical { background: #ff4d6d; box-shadow: 0 0 6px #ff4d6d; }
.dot.high { background: #ff8a4c; }
.dot.medium { background: #ffc857; }
.name { flex: 1; font-size: 12px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.badge { font-size: 10px; color: #7f9fd6; }
.empty { color: #5f7aa8; font-size: 12px; padding: 20px 12px; }
</style>
