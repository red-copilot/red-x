<template>
  <v-chart class="map" :option="option" autoresize @click="onClick" />
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { use, registerMap } from 'echarts/core'
import { CanvasRenderer } from 'echarts/renderers'
import { ScatterChart, LinesChart } from 'echarts/charts'
import { GeoComponent, TooltipComponent } from 'echarts/components'
import VChart from 'vue-echarts'
import world from '@/assets/world.geo.json'
import type { GraphSnapshot, MapPoint, OrgBubble, Risk } from './types'

use([CanvasRenderer, ScatterChart, LinesChart, GeoComponent, TooltipComponent])
registerMap('world', world as any)

const props = defineProps<{
  organizations: OrgBubble[]
  points: MapPoint[]
  graph: GraphSnapshot | null
  graphOn: boolean
  selectedOrgId: string
}>()

const emit = defineEmits<{
  (e: 'select-org', id: string): void
  (e: 'select-point', point: MapPoint): void
}>()

const COLORS: Record<string, string> = {
  critical: '#ff4d6d', high: '#ff8a4c', medium: '#ffc857', low: '#4fc3ff', info: '#7f9fd6',
}
function colorOf(risk?: Risk) { return COLORS[risk || ''] || '#4fc3ff' }
function bare(id: string) { return id.replace(/^(domain|ip|service|finding):/, '') }

const orgData = computed(() => props.organizations.map((o) => ({
  name: o.name, value: [o.lon, o.lat, o.assetCount], orgId: o.orgId, risk: o.risk,
})))
const pointData = computed(() => props.points.map((p) => ({
  name: p.key, value: [p.lon, p.lat], assetId: p.assetId, risk: p.risk, cdn: p.cdn, cdnName: p.cdnName,
})))

const coordMap = computed(() => {
  const m = new Map<string, [number, number]>()
  props.organizations.forEach((o) => m.set(o.orgId, [o.lon, o.lat]))
  props.points.forEach((p) => m.set(p.key, [p.lon, p.lat]))
  if (props.graph) {
    for (const edge of props.graph.edges) {
      if (edge.type === 'RESOLVES_TO') {
        const c = m.get(bare(edge.target))
        if (c) m.set(edge.source, c)
      } else if (edge.type === 'EXPOSES') {
        const c = m.get(bare(edge.source))
        if (c) m.set(edge.target, c)
      } else if (edge.type === 'FOUND_ON') {
        const c = m.get(bare(edge.target))
        if (c) m.set(edge.source, c)
      }
    }
  }
  return m
})

const lines = computed(() => {
  if (!props.graphOn || !props.graph) return []
  const out: Array<{ coords: Array<[number, number]> }> = []
  for (const edge of props.graph.edges) {
    const a = coordMap.value.get(edge.source)
    const b = coordMap.value.get(edge.target)
    if (a && b) out.push({ coords: [a, b] })
  }
  return out
})

const option = computed(() => {
  const series: any[] = [
    {
      name: '组织', type: 'scatter', coordinateSystem: 'geo', data: orgData.value,
      symbolSize: (val: any) => 10 + Math.min(26, Math.sqrt(val[2] || 1) * 4),
      itemStyle: { color: (p: any) => colorOf(p.data?.risk), shadowBlur: 12, shadowColor: '#3d7bff' },
      emphasis: { scale: 1.25 },
      tooltip: { formatter: (p: any) => `${p.data?.name}<br/>资产 ${p.data?.value?.[2] ?? 0}` },
    },
    {
      name: '资产', type: 'scatter', coordinateSystem: 'geo', data: pointData.value, symbolSize: 6,
      itemStyle: {
        color: (p: any) => p.data?.cdn ? '#5ad1ff' : colorOf(p.data?.risk),
        borderColor: (p: any) => p.data?.cdn ? '#eaf2ff' : 'transparent',
        borderWidth: (p: any) => p.data?.cdn ? 1 : 0,
      },
      tooltip: { formatter: (p: any) => `${p.data?.name}<br/>${p.data?.cdn ? 'CDN 边缘 · ' : ''}风险 ${p.data?.risk || '无'}` },
    },
  ]
  if (lines.value.length) {
    series.push({
      name: '关系', type: 'lines', coordinateSystem: 'geo', data: lines.value, silent: true,
      lineStyle: { color: '#3d7bff', opacity: 0.45, width: 1, curveness: 0.12 },
    })
  }
  return {
    backgroundColor: 'transparent',
    tooltip: { trigger: 'item', backgroundColor: '#0b1830', borderColor: '#1d3f66', textStyle: { color: '#cfe0ff' } },
    geo: {
      map: 'world', roam: true, zoom: 1.1, silent: false,
      itemStyle: { areaColor: '#0a1526', borderColor: '#1b3553', borderWidth: 0.6 },
      emphasis: { itemStyle: { areaColor: '#122a47' }, label: { show: false } },
      select: { disabled: true },
    },
    series,
  }
})

function onClick(params: any) {
  if (params.seriesName === '组织') emit('select-org', params.data.orgId)
  if (params.seriesName === '资产') {
    const point = props.points.find((p) => p.assetId === params.data.assetId)
    if (point) emit('select-point', point)
  }
}
</script>

<style scoped>
.map { width: 100%; height: 100%; min-height: 420px; }
</style>
