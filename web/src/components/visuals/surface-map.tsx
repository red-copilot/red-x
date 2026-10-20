import * as echarts from 'echarts/core'
import { MapChart, ScatterChart } from 'echarts/charts'
import { GeoComponent, GridComponent, TooltipComponent } from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'
import ReactECharts from 'echarts-for-react'
import { useMemo } from 'react'
import worldGeo from '@/assets/world.geo.json'
import type { MapSnapshot } from '@/api'

echarts.use([MapChart, ScatterChart, GeoComponent, GridComponent, TooltipComponent, CanvasRenderer])
let registered = false
if (!registered) {
  echarts.registerMap('arl-world', worldGeo as never)
  registered = true
}

const colors: Record<string, string> = {
  critical: '#b42318',
  high: '#d9480f',
  medium: '#a15c00',
  low: '#2563a5',
  info: '#667085',
}

export function SurfaceMap({
  snapshot,
  onSelect,
}: {
  snapshot?: MapSnapshot
  onSelect?: (assetId: string, orgIds: string[]) => void
}) {
  const option = useMemo(() => {
    const points = snapshot?.points || []
    const series = Object.entries(colors).map(([risk, color]) => ({
      name: risk,
      type: 'scatter',
      coordinateSystem: 'geo',
      symbolSize: risk === 'critical' ? 11 : risk === 'high' ? 9 : 7,
      itemStyle: { color, borderColor: '#fff', borderWidth: 1.5, opacity: 0.9 },
      emphasis: { scale: 1.5 },
      data: points
        .filter((point) => (point.risk || 'info') === risk)
        .map((point) => ({
          name: point.key,
          value: [point.lon, point.lat],
          assetId: point.assetId,
          orgIds: point.orgIds || [],
        })),
    }))
    return {
      animationDuration: 250,
      backgroundColor: 'transparent',
      tooltip: {
        trigger: 'item',
        borderColor: '#e1e5eb',
        backgroundColor: '#fff',
        textStyle: { color: '#172033', fontSize: 11 },
        formatter: (params: any) => `<b>${params.name}</b><br/>${params.seriesName || ''}`,
      },
      geo: {
        map: 'arl-world',
        roam: true,
        zoom: 1.15,
        center: [105, 28],
        silent: false,
        itemStyle: { areaColor: '#f2f5f9', borderColor: '#dbe2ea', borderWidth: 0.7 },
        emphasis: { itemStyle: { areaColor: '#e9eff6' }, label: { show: false } },
        select: { disabled: true },
      },
      series,
    }
  }, [snapshot])

  return (
    <div className="relative h-full min-h-[260px] w-full overflow-hidden rounded-lg border bg-[#fbfcfe] data-grid">
      <ReactECharts
        option={option}
        notMerge
        style={{ height: '100%', minHeight: 360 }}
        onEvents={{
          click: (params: any) => {
            if (params?.data?.assetId) onSelect?.(params.data.assetId, params.data.orgIds || [])
          },
        }}
      />
      <div className="pointer-events-none absolute bottom-3 left-3 rounded-md border bg-white/90 px-2.5 py-1.5 text-[10px] text-muted-foreground shadow-sm">
        经纬度来自资产 IP · 未定位资产不绘制
      </div>
    </div>
  )
}
