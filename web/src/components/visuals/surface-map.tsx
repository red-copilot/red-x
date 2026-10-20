import * as echarts from 'echarts/core'
import { MapChart, ScatterChart } from 'echarts/charts'
import { GeoComponent, GridComponent, TooltipComponent } from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'
import ReactECharts from 'echarts-for-react'
import { useMemo, useRef } from 'react'
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
  const chartRef = useRef<any>(null)
  const dragRef = useRef<{ x: number; y: number } | null>(null)

  function onPointerDown(event: React.PointerEvent<HTMLDivElement>) {
    if (event.button !== 0) return
    const chart = chartRef.current
    if (!chart) return
    const rect = event.currentTarget.getBoundingClientRect()
    const point = [event.clientX - rect.left, event.clientY - rect.top]
    try {
      if (chart.containPixel?.({ geoIndex: 0 }, point)) return
    } catch {
      // Fall through to the panel-level drag handler if ECharts is not ready.
    }
    dragRef.current = { x: event.clientX, y: event.clientY }
    event.currentTarget.setPointerCapture(event.pointerId)
    event.preventDefault()
  }

  function onPointerMove(event: React.PointerEvent<HTMLDivElement>) {
    const last = dragRef.current
    if (!last) return
    const dx = event.clientX - last.x
    const dy = event.clientY - last.y
    if (dx !== 0 || dy !== 0) {
      chartRef.current?.dispatchAction({ type: 'geoRoam', componentType: 'geo', geoIndex: 0, dx, dy })
      dragRef.current = { x: event.clientX, y: event.clientY }
    }
    event.preventDefault()
  }

  function endDrag(event: React.PointerEvent<HTMLDivElement>) {
    if (!dragRef.current) return
    dragRef.current = null
    event.currentTarget.releasePointerCapture(event.pointerId)
  }

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
    <div
      data-testid="surface-map"
      className="data-grid relative h-full min-h-[260px] w-full cursor-grab overflow-hidden rounded-lg border bg-[#fbfcfe] active:cursor-grabbing"
      onPointerDown={onPointerDown}
      onPointerMove={onPointerMove}
      onPointerUp={endDrag}
      onPointerCancel={endDrag}
    >
      <ReactECharts
        option={option}
        notMerge
        style={{ height: '100%', width: '100%' }}
        onChartReady={(chart) => {
          chartRef.current = chart
        }}
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
