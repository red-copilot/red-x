import * as echarts from 'echarts/core'
import { GraphChart } from 'echarts/charts'
import { TooltipComponent } from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'
import ReactECharts from 'echarts-for-react'
import { useMemo } from 'react'
import type { GraphSnapshot } from '@/api'

echarts.use([GraphChart, TooltipComponent, CanvasRenderer])

const colors: Record<string, string> = {
  critical: '#b42318',
  high: '#d9480f',
  medium: '#a15c00',
  low: '#2563a5',
  info: '#64748b',
}

export function AssetGraph({ snapshot, onSelect }: { snapshot?: GraphSnapshot; onSelect?: (node: any) => void }) {
  const option = useMemo(() => {
    const nodes = snapshot?.nodes || []
    const edges = snapshot?.edges || []
    return {
      animationDuration: 300,
      tooltip: { trigger: 'item', textStyle: { fontSize: 11 } },
      series: [
        {
          type: 'graph',
          layout: 'force',
          roam: true,
          draggable: true,
          force: { repulsion: 180, edgeLength: [60, 130], gravity: 0.08 },
          label: {
            show: true,
            position: 'right',
            fontSize: 10,
            color: '#475569',
            formatter: (params: any) => (params.data?.showLabel ? params.name : ''),
          },
          lineStyle: { color: '#c9d3df', width: 1, curveness: 0.08 },
          emphasis: { focus: 'adjacency', lineStyle: { width: 2, color: '#7c9fc9' } },
          data: nodes.map((node, index) => ({
            id: node.id,
            name: node.label,
            symbolSize: node.type === 'organization' ? 26 : node.type === 'domain' ? 18 : 12,
            showLabel: index < 20 || Boolean(node.risk),
            itemStyle: { color: node.cdn ? '#8aa0b8' : colors[node.risk || 'info'] || '#64748b' },
            ...node,
          })),
          links: edges.map((edge) => ({ source: edge.source, target: edge.target, name: edge.type })),
        },
      ],
    }
  }, [snapshot])

  return (
    <div data-testid="asset-graph" className="relative h-full min-h-[260px] w-full cursor-grab overflow-hidden rounded-lg border bg-card active:cursor-grabbing">
      <ReactECharts
        option={option}
        notMerge
        style={{ height: '100%', width: '100%' }}
        onEvents={{
          click: (params: any) => {
            if (params?.data) onSelect?.(params.data)
          },
        }}
      />
    </div>
  )
}
