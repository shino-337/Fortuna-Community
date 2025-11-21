/**
 * Force Graph SVG Component
 * Using D3.js with SVG for better control and debugging
 * K8s Fortuna Platform
 */

import React, { useEffect, useRef, useMemo, useCallback } from 'react'
import * as d3 from 'd3'
import { RbacNode, RbacLink, GraphData } from './types'
import { NODE_COLORS, NODE_RADIUS, FORCE_PARAMS, ZOOM_PARAMS } from './constants'

interface ForceGraphSvgProps {
  data: GraphData
  width: number
  height: number
  onNodeClick: (node: RbacNode) => void
  selectedNodeId: string | null
  filter: (node: RbacNode) => boolean
  layout?: 'force' | 'radial' | 'tree' | 'grid'
}

const ForceGraphSvg: React.FC<ForceGraphSvgProps> = ({
  data,
  width,
  height,
  onNodeClick,
  selectedNodeId,
  filter,
  layout = 'force',
}) => {
  const svgRef = useRef<SVGSVGElement>(null)
  const simulationRef = useRef<d3.Simulation<RbacNode, RbacLink> | null>(null)

  // Filter data based on filter function
  const filteredData = useMemo(() => {
    const filteredNodes = data.nodes.filter(filter)
    const filteredNodeIds = new Set(filteredNodes.map(n => n.id))
    const filteredLinks = data.links.filter(l => {
      const sourceId = typeof l.source === 'object' ? (l.source as RbacNode).id : (l.source as string)
      const targetId = typeof l.target === 'object' ? (l.target as RbacNode).id : (l.target as string)
      return filteredNodeIds.has(sourceId) && filteredNodeIds.has(targetId)
    })

    return { nodes: filteredNodes, links: filteredLinks }
  }, [data, filter])

  // Handle selection highlighting
  const relatedNodeIds = useMemo(() => {
    if (!selectedNodeId) return new Set<string>()

    const related = new Set<string>([selectedNodeId])
    filteredData.links.forEach(l => {
      const sId = typeof l.source === 'object' ? (l.source as RbacNode).id : (l.source as string)
      const tId = typeof l.target === 'object' ? (l.target as RbacNode).id : (l.target as string)
      if (sId === selectedNodeId) related.add(tId)
      if (tId === selectedNodeId) related.add(sId)
    })

    return related
  }, [selectedNodeId, filteredData.links])

  // Apply layout-specific forces
  const applyLayoutForces = useCallback(
    (simulation: d3.Simulation<RbacNode, RbacLink>) => {
      // Remove existing forces
      simulation
        .force('link', null)
        .force('charge', null)
        .force('center', null)
        .force('collide', null)
        .force('x', null)
        .force('y', null)

      switch (layout) {
        case 'radial':
          simulation
            .force('link', d3.forceLink<RbacNode, RbacLink>(filteredData.links).id(d => d.id).distance(FORCE_PARAMS.linkDistance))
            .force('charge', d3.forceManyBody().strength(FORCE_PARAMS.chargeStrength))
            .force('r', d3.forceRadial(Math.min(width, height) / 3, width / 2, height / 2).strength(0.5))
            .force('collide', d3.forceCollide().radius(d => ((d as RbacNode).radius || 20) + FORCE_PARAMS.collideRadius))
          break

        case 'tree':
          // Tree layout uses hierarchical positioning
          simulation
            .force('link', d3.forceLink<RbacNode, RbacLink>(filteredData.links).id(d => d.id).distance(150))
            .force('charge', d3.forceManyBody().strength(-300))
            .force('collide', d3.forceCollide().radius(d => ((d as RbacNode).radius || 20) + FORCE_PARAMS.collideRadius))
          break

        case 'grid':
          // Grid layout - fixed positions
          const gridSize = Math.ceil(Math.sqrt(filteredData.nodes.length))
          const cellWidth = width / (gridSize + 1)
          const cellHeight = height / (gridSize + 1)

          filteredData.nodes.forEach((node, i) => {
            const row = Math.floor(i / gridSize)
            const col = i % gridSize
            node.fx = (col + 1) * cellWidth
            node.fy = (row + 1) * cellHeight
          })

          simulation
            .force('collide', d3.forceCollide().radius(d => ((d as RbacNode).radius || 20) + 20))
          simulation.alpha(0.1) // Low alpha for quick settle
          break

        case 'force':
        default:
          simulation
            .force('link', d3.forceLink<RbacNode, RbacLink>(filteredData.links).id(d => d.id).distance(FORCE_PARAMS.linkDistance))
            .force('charge', d3.forceManyBody().strength(FORCE_PARAMS.chargeStrength))
            .force('center', d3.forceCenter(width / 2, height / 2))
            .force('collide', d3.forceCollide().radius(d => ((d as RbacNode).radius || 20) + FORCE_PARAMS.collideRadius))
          break
      }

      simulation.alpha(0.8).restart()
    },
    [layout, filteredData, width, height]
  )

  // Initialize and render graph
  useEffect(() => {
    if (!svgRef.current || filteredData.nodes.length === 0) return

    const svg = d3.select(svgRef.current)
    svg.selectAll('*').remove() // Clear previous render

    // Defs for markers and gradients
    const defs = svg.append('defs')
    
    // Arrow marker
    defs
      .append('marker')
      .attr('id', 'arrowhead')
      .attr('viewBox', '0 -5 10 10')
      .attr('refX', 8)
      .attr('refY', 0)
      .attr('orient', 'auto')
      .attr('markerWidth', 6)
      .attr('markerHeight', 6)
      .append('path')
      .attr('d', 'M0,-5L10,0L0,5')
      .attr('fill', '#9ca3af')

    // Main container group
    const g = svg.append('g').attr('class', 'graph-container')

    // Zoom behavior
    const zoom = d3.zoom<SVGSVGElement, unknown>()
      .scaleExtent(ZOOM_PARAMS.scaleExtent)
      .on('zoom', (event) => {
        g.attr('transform', event.transform)
      })

    svg.call(zoom)

    // Initialize simulation
    const simulation = d3.forceSimulation<RbacNode, RbacLink>(filteredData.nodes)
    simulationRef.current = simulation

    // Draw links
    const link = g
      .append('g')
      .attr('class', 'links')
      .selectAll('line')
      .data(filteredData.links)
      .join('line')
      .attr('stroke', '#cbd5e1') // Gray 300
      .attr('stroke-opacity', 0.6)
      .attr('stroke-width', 2)
      .attr('marker-end', 'url(#arrowhead)')

    // Draw nodes
    const node = g
      .append('g')
      .attr('class', 'nodes')
      .selectAll<SVGGElement, RbacNode>('g')
      .data(filteredData.nodes)
      .join('g')
      .attr('cursor', 'pointer')
      .call(
        d3.drag<SVGGElement, RbacNode>()
          .on('start', (event, d) => {
            if (!event.active) simulation.alphaTarget(0.3).restart()
            d.fx = d.x
            d.fy = d.y
          })
          .on('drag', (event, d) => {
            d.fx = event.x
            d.fy = event.y
          })
          .on('end', (event, d) => {
            if (!event.active) simulation.alphaTarget(0)
            // Keep fixed position for grid layout
            if (layout !== 'grid') {
              d.fx = null
              d.fy = null
            }
          })
      )

    // Node circles
    node
      .append('circle')
      .attr('r', d => {
        const radius = NODE_RADIUS[d.type] || 12
        d.radius = radius // Store radius on node
        return radius
      })
      .attr('fill', d => NODE_COLORS[d.type] || NODE_COLORS.unknown)
      .attr('stroke', '#1e293b') // Slate 800
      .attr('stroke-width', 2)
      .attr('class', 'node-circle')

    // Node labels
    node
      .append('text')
      .text(d => {
        const label = d.label || d.id
        return label.length > 20 ? label.substring(0, 17) + '...' : label
      })
      .attr('x', d => (d.radius || 12) + 6)
      .attr('y', 5)
      .attr('fill', '#f1f5f9') // Slate 100
      .style('font-size', '13px')
      .style('font-weight', '500')
      .style('pointer-events', 'none')
      .style('text-shadow', '0 1px 3px rgba(0,0,0,0.9), 0 0 8px rgba(0,0,0,0.6)')

    // Click handler
    node.on('click', (event, d) => {
      event.stopPropagation()
      onNodeClick(d)
    })

    // Background click to deselect
    svg.on('click', () => {
      onNodeClick({ id: '', label: '', type: '', data: {} } as RbacNode)
    })

    // Tick function
    simulation.on('tick', () => {
      // Update link positions with arrow offset
      link
        .attr('x1', d => (d.source as RbacNode).x!)
        .attr('y1', d => (d.source as RbacNode).y!)
        .attr('x2', d => {
          const target = d.target as RbacNode
          const source = d.source as RbacNode
          const dx = target.x! - source.x!
          const dy = target.y! - source.y!
          const dist = Math.sqrt(dx * dx + dy * dy)
          if (dist === 0) return target.x!
          const r = (target.radius || 12) + 8
          return target.x! - (dx * r) / dist
        })
        .attr('y2', d => {
          const target = d.target as RbacNode
          const source = d.source as RbacNode
          const dx = target.x! - source.x!
          const dy = target.y! - source.y!
          const dist = Math.sqrt(dx * dx + dy * dy)
          if (dist === 0) return target.y!
          const r = (target.radius || 12) + 8
          return target.y! - (dy * r) / dist
        })

      // Update node positions
      node.attr('transform', d => `translate(${d.x},${d.y})`)
    })

    // Apply layout
    applyLayoutForces(simulation)

    // Auto-fit after layout settles
    const fitDelay = layout === 'grid' ? 200 : 1000
    const fitTimer = setTimeout(() => {
      const bounds = g.node()?.getBBox()
      if (bounds) {
        const dx = bounds.width
        const dy = bounds.height
        const x = bounds.x + bounds.width / 2
        const y = bounds.y + bounds.height / 2
        const scale = Math.min(
          0.9 / Math.max(dx / width, dy / height),
          ZOOM_PARAMS.scaleExtent[1]
        )
        const translate = [width / 2 - scale * x, height / 2 - scale * y]

        svg
          .transition()
          .duration(ZOOM_PARAMS.duration)
          .call(
            zoom.transform as any,
            d3.zoomIdentity.translate(translate[0], translate[1]).scale(scale)
          )
      }
    }, fitDelay)

    return () => {
      clearTimeout(fitTimer)
      simulation.stop()
    }
  }, [filteredData, width, height, layout, applyLayoutForces, onNodeClick, filter])

  // Update selection highlighting
  useEffect(() => {
    if (!svgRef.current) return

    const svg = d3.select(svgRef.current)
    const g = svg.select('.graph-container')

    if (!g.empty()) {
      const isNoneSelected = !selectedNodeId

      // Update node styles
      g.selectAll('.node-circle')
        .transition()
        .duration(300)
        .attr('opacity', (d: any) => (isNoneSelected || relatedNodeIds.has(d.id) ? 1 : 0.25))
        .attr('stroke', (d: any) =>
          d.id === selectedNodeId ? '#ffffff' : relatedNodeIds.has(d.id) ? '#f472b6' : '#1e293b'
        )
        .attr('stroke-width', (d: any) =>
          d.id === selectedNodeId ? 4 : relatedNodeIds.has(d.id) ? 3 : 2
        )

      // Update link styles
      g.selectAll('line')
        .transition()
        .duration(300)
        .attr('stroke-opacity', (d: any) => {
          const sId = typeof d.source === 'object' ? d.source.id : d.source
          const tId = typeof d.target === 'object' ? d.target.id : d.target
          return isNoneSelected || (relatedNodeIds.has(sId) && relatedNodeIds.has(tId)) ? 0.8 : 0.15
        })
        .attr('stroke', (d: any) => {
          const sId = typeof d.source === 'object' ? d.source.id : d.source
          const tId = typeof d.target === 'object' ? d.target.id : d.target
          return relatedNodeIds.has(sId) && relatedNodeIds.has(tId) && !isNoneSelected
            ? '#ec4899' // Pink 500
            : '#cbd5e1' // Gray 300
        })
        .attr('stroke-width', (d: any) => {
          const sId = typeof d.source === 'object' ? d.source.id : d.source
          const tId = typeof d.target === 'object' ? d.target.id : d.target
          return relatedNodeIds.has(sId) && relatedNodeIds.has(tId) && !isNoneSelected ? 3 : 2
        })
    }
  }, [selectedNodeId, relatedNodeIds])

  return (
    <svg
      ref={svgRef}
      width={width}
      height={height}
      className="w-full h-full bg-slate-900 rounded-lg shadow-inner cursor-move"
      style={{ backgroundColor: '#0f172a' }} // Slate 900
    />
  )
}

export default ForceGraphSvg

