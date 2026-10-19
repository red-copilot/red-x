<template>
  <aside class="inspector">
    <div class="head">
      <span>{{ title }}</span>
      <button class="close" @click="$emit('close')">×</button>
    </div>

    <template v-if="point">
      <div class="section">
        <div class="label">IP</div>
        <div class="value mono">{{ point.key }}</div>
        <div class="row"><span>风险</span><b :class="['risk', point.risk]">{{ point.risk || '无' }}</b></div>
        <div class="row"><span>类型</span><b>{{ point.cdn ? 'CDN 边缘' : '资产 IP' }}</b></div>
        <div v-if="point.cdnName" class="row"><span>CDN</span><b>{{ point.cdnName }}</b></div>
        <div class="row"><span>所属组织</span><b>{{ point.orgIds.length }}</b></div>
      </div>
    </template>

    <template v-else-if="node">
      <div class="section">
        <div class="label">{{ node.type }}</div>
        <div class="value mono">{{ node.label }}</div>
        <div class="row"><span>风险</span><b :class="['risk', node.risk || '']">{{ node.risk || '无' }}</b></div>
        <div v-if="node.cdn" class="row"><span>标记</span><b>CDN 边缘</b></div>
      </div>
    </template>

    <template v-else-if="organization">
      <div class="section">
        <div class="label">组织</div>
        <div class="value">{{ organization.canonicalName }}</div>
        <div v-if="organization.region" class="row"><span>地区</span><b>{{ organization.region }}</b></div>
        <div v-if="organization.aliases?.length" class="row"><span>别名</span><b>{{ organization.aliases.join('、') }}</b></div>
      </div>

      <div class="section">
        <div class="section-head">
          <span>备案主域</span>
          <button class="mini" :disabled="busy" @click="$emit('refresh')">反查全部</button>
        </div>
        <div v-if="job" class="job">
          反查任务：{{ job.status }}<span v-if="job.resultCount"> · {{ job.resultCount }} 条</span>
          <span v-if="job.error" class="err"> · {{ job.error }}</span>
        </div>
        <div v-if="!domains.length" class="empty">暂无备案域名</div>
        <label v-for="d in domains" :key="d.id" class="domain">
          <template v-if="d.status === 'candidate'">
            <input v-model="selected" type="checkbox" :value="d.rootDomain" />
          </template>
          <i v-else class="ok" />
          <span class="mono">{{ d.rootDomain }}</span>
          <span class="state">{{ statusText(d.status) }}</span>
        </label>
      </div>

      <div v-if="candidates.length" class="section">
        <label class="task"><input v-model="createTask" type="checkbox" /> 同时创建扫描任务</label>
        <button class="confirm" :disabled="busy || !selected.length" @click="confirm">
          确认所选（{{ selected.length }}）
        </button>
      </div>
    </template>
  </aside>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import type { GraphNode, ICPJob, MapPoint, Organization, OrganizationDomain } from './types'

const props = defineProps<{
  organization?: Organization | null
  domains: OrganizationDomain[]
  point?: MapPoint | null
  node?: GraphNode | null
  job?: ICPJob | null
  busy?: boolean
}>()
const emit = defineEmits<{
  (e: 'close'): void
  (e: 'refresh'): void
  (e: 'confirm', domains: string[], createTask: boolean): void
}>()

const selected = ref<string[]>([])
const createTask = ref(false)
const candidates = computed(() => props.domains.filter((d) => d.status === 'candidate'))
const title = computed(() => props.point ? '资产详情' : props.node ? '图谱节点' : '组织详情')

watch(
  () => props.domains,
  (list) => { selected.value = list.filter((d) => d.status === 'candidate').map((d) => d.rootDomain) },
  { immediate: true },
)

function statusText(status: string) {
  return status === 'confirmed' ? '已确认' : status === 'candidate' ? '待确认' : status
}
function confirm() {
  emit('confirm', [...selected.value], createTask.value)
}
</script>

<style scoped>
.inspector { width: 320px; flex: 0 0 320px; border-left: 1px solid #12294a; background: #081226; padding: 14px 16px; overflow: auto; }
.head { display: flex; justify-content: space-between; align-items: center; color: #8fb4ff; font-size: 12px; letter-spacing: .1em; margin-bottom: 14px; }
.close { background: none; border: 0; color: #7f9fd6; font-size: 20px; cursor: pointer; }
.section { border-top: 1px solid #12294a; padding: 12px 0; }
.section:first-of-type { border-top: 0; padding-top: 0; }
.label { color: #5f7aa8; font-size: 10px; letter-spacing: .12em; text-transform: uppercase; margin-bottom: 5px; }
.value { color: #eaf2ff; font-size: 15px; margin-bottom: 8px; word-break: break-all; }
.mono { font-family: ui-monospace, SFMono-Regular, Menlo, monospace; }
.row { display: flex; justify-content: space-between; font-size: 12px; color: #7f9fd6; margin-top: 5px; }
.row b { color: #cfe0ff; font-weight: 500; }
.risk.critical { color: #ff4d6d; } .risk.high { color: #ff8a4c; } .risk.medium { color: #ffc857; } .risk.low { color: #4fc3ff; }
.section-head { display: flex; justify-content: space-between; align-items: center; color: #8fb4ff; font-size: 12px; margin-bottom: 8px; }
.mini { background: #10233d; border: 1px solid #2b5fae; color: #9fc4ff; border-radius: 5px; font-size: 11px; padding: 3px 7px; cursor: pointer; }
.domain { display: flex; align-items: center; gap: 7px; font-size: 12px; color: #cfe0ff; padding: 4px 0; cursor: pointer; }
.domain .ok { width: 7px; height: 7px; border-radius: 50%; background: #7ef0b2; }
.domain .mono { flex: 1; overflow: hidden; text-overflow: ellipsis; }
.state { color: #5f7aa8; font-size: 10px; }
.empty { color: #5f7aa8; font-size: 12px; padding: 8px 0; }
.task { display: flex; gap: 7px; align-items: center; font-size: 12px; color: #9db9e8; margin-bottom: 10px; }
.confirm { width: 100%; background: #2f6fe4; border: 0; color: #fff; padding: 9px 10px; border-radius: 6px; cursor: pointer; }
.confirm:disabled { opacity: .45; cursor: default; }
.job { font-size: 11px; color: #9db9e8; margin-bottom: 8px; }
.err { color: #ffb3c1; }
</style>
