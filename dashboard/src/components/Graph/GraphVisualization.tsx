import { useEffect, useRef, useState, useCallback, useMemo } from 'react'
import cytoscape, { Core, EdgeDefinition, NodeDefinition, NodeSingular, LayoutOptions } from 'cytoscape'
// @ts-ignore - No type definitions available
import dagre from 'cytoscape-dagre'
// @ts-ignore - No type definitions available
import fcose from 'cytoscape-fcose'
import { useGraph } from '../../hooks/useGraph'
import { useServiceAccount } from '../../hooks/useServiceAccounts'

// Register Cytoscape extensions
cytoscape.use(dagre)
cytoscape.use(fcose)

// Mở rộng module để thêm các thuộc tính shadow (không được Cytoscape hỗ trợ trực tiếp)
// Note: Shadow properties are not actually used in styles, they were removed earlier
// This declaration is kept for potential future use
declare module 'cytoscape' {
  interface Css {
    // Shadow properties are not supported by Cytoscape, so we don't extend StylesheetStyle
    // These are kept as comments for reference only
  }
}

interface GraphVisualizationProps {
  cluster?: string
  namespace?: string
  height?: string
  onLoadingChange?: (loading: boolean) => void // Callback to notify parent of loading state
  nodeTypeFilters?: Record<string, boolean>
  connectionTypeFilters?: Record<string, boolean>
  layout?: GraphLayoutOption
  autoFitEnabled?: boolean
  onNodeSelect?: (nodeId: string, nodeType: string, nodeData: Record<string, any>) => void // Callback when node is selected
  selectedNodeId?: string | null // Currently selected node ID
}

interface TooltipPosition {
  x: number
  y: number
}

// Node configuration interface for better type safety
interface NodeConfig {
  minWidth: number        // Minimum width for the node
  maxWidth: number        // Maximum width for the node
  textMaxWidth: number    // Maximum text width inside node
  fontSize: number        // Font size in pixels
  charWidth: number       // Average character width in pixels
  padding: number         // Padding inside node (applies to all sides)
  heightRatio: number     // Base height ratio relative to width
  textMarginY: number     // Vertical adjustment for text position
  fontWeight: number      // Font weight for text
  lineHeight: number      // Line height multiplier for multi-line text
  extraPadding: number    // Additional padding for complex shapes
  backgroundColor: string // Background color of node
  borderColor: string     // Border color when not selected/hovered
  shadowColor: string     // Shadow color
}

/**
 * Node type configurations - centralized and optimized for different text lengths
 * These values are carefully tuned to handle various scenarios:
 * 1. Short text (1-10 characters)
 * 2. Medium text (11-30 characters)
 * 3. Long text (31+ characters)
 * 
 * Each node type has different shape characteristics that affect text display
 */
const NODE_CONFIGS: Record<string, NodeConfig> = {
  serviceaccount: {
    minWidth: 120,         // Minimum width
    maxWidth: 240,         // Maximum width
    textMaxWidth: 200,     // Max text width before truncation
    fontSize: 12,          // Base font size (will be dynamic)
    charWidth: 7.5,        // Average character width
    padding: 32,           // Padding for ellipse
    heightRatio: 0.9,      // Elliptical shape
    textMarginY: -1,       // Slight adjustment for vertical centering
    fontWeight: 600,       // Semi-bold
    lineHeight: 1.4,       // Line height
    extraPadding: 12,      // Additional padding for ellipse shape
    backgroundColor: '#f97316', // Professional Orange
    borderColor: '#ea580c', // Darker Orange
    shadowColor: '#f9731640', // Semi-transparent orange
  },
  role: {
    minWidth: 120,         // Minimum width
    maxWidth: 240,         // Maximum width
    textMaxWidth: 200,     // Max text width before truncation
    fontSize: 12,          // Base font size (will be dynamic)
    charWidth: 7.5,        // Average character width
    padding: 36,           // Padding for diamond
    heightRatio: 1.0,      // Square base for diamond
    textMarginY: 0,
    fontWeight: 600,
    lineHeight: 1.4,
    extraPadding: 16,      // Diamond needs more padding
    backgroundColor: '#9333ea', // Professional Purple
    borderColor: '#7e22ce', // Darker Purple
    shadowColor: '#9333ea40', // Semi-transparent purple
  },
  clusterrole: {
    minWidth: 120,         // Minimum width
    maxWidth: 240,         // Maximum width
    textMaxWidth: 200,     // Max text width before truncation
    fontSize: 12,          // Base font size (will be dynamic)
    charWidth: 7.5,        // Average character width
    padding: 38,           // Extra padding for diamond
    heightRatio: 1.0,      // Square base for diamond
    textMarginY: 0,
    fontWeight: 600,
    lineHeight: 1.4,
    extraPadding: 18,      // Diamond needs more padding
    backgroundColor: '#7c3aed', // Royal Purple
    borderColor: '#6d28d9', // Darker Royal Purple
    shadowColor: '#7c3aed40', // Semi-transparent royal purple
  },
  namespace: {
    minWidth: 120,         // Minimum width
    maxWidth: 240,         // Maximum width
    textMaxWidth: 200,     // Max text width before truncation
    fontSize: 12,          // Base font size (will be dynamic)
    charWidth: 7.5,        // Average character width
    padding: 30,
    heightRatio: 0.7,      // Rectangle is wider than tall
    textMarginY: 0,
    fontWeight: 600,
    lineHeight: 1.4,
    extraPadding: 8,       // Rectangle needs less extra padding
    backgroundColor: '#2563eb', // Royal Blue
    borderColor: '#1d4ed8', // Darker Royal Blue
    shadowColor: '#2563eb40', // Semi-transparent royal blue
  },
  cluster: {
    minWidth: 120,         // Minimum width
    maxWidth: 240,         // Maximum width
    textMaxWidth: 200,     // Max text width before truncation
    fontSize: 12,          // Base font size (will be dynamic)
    charWidth: 7.5,        // Average character width
    padding: 34,
    heightRatio: 0.65,     // Wide rectangle
    textMarginY: 0,
    fontWeight: 700,       // Bold
    lineHeight: 1.4,
    extraPadding: 8,       // Rectangle needs less extra padding
    backgroundColor: '#1e3a8a', // Navy Blue
    borderColor: '#1e40af', // Darker Navy Blue
    shadowColor: '#1e3a8a40', // Semi-transparent navy blue
  },
}

const LAYOUT_OPTIONS: Record<'cose' | 'fcose' | 'dagre' | 'breadthfirst' | 'circle' | 'concentric' | 'grid', LayoutOptions> = {
  cose: {
    name: 'cose',
    nodeDimensionsIncludeLabels: true,
    idealEdgeLength: 100,
    nodeOverlap: 20,
    refresh: 20,
    padding: 50,
    randomize: false,
    componentSpacing: 120,
    nodeRepulsion: 450000,
    edgeElasticity: 100,
    nestingFactor: 5,
    gravity: 80,
    numIter: 1000,
    initialTemp: 200,
    coolingFactor: 0.95,
    minTemp: 1.0,
    animate: true, // Enable animation for smooth transitions
    animationDuration: 800, // 800ms for smooth easing
    animationEasing: 'ease-out', // Smooth easing
  },
  fcose: {
    name: 'fcose',
    nodeDimensionsIncludeLabels: true,
    quality: 'default',
    randomize: false,
    animate: true, // Enable animation for smooth transitions
    animationDuration: 800, // 800ms for smooth easing
    animationEasing: 'ease-out', // Smooth easing
    fit: false,
    padding: 50,
    nodeRepulsion: 4500,
    idealEdgeLength: 100,
    edgeElasticity: 0.45,
    nestingFactor: 0.1,
    gravity: 0.25,
    numIter: 2500,
  } as any,
  dagre: {
    name: 'dagre',
    nodeDimensionsIncludeLabels: true,
    rankDir: 'TB',
    rankSep: 100,
    nodeSep: 50,
    edgeSep: 50,
    ranker: 'network-simplex',
    animate: true, // Enable animation
    animationDuration: 800,
    animationEasing: 'ease-out',
    padding: 50,
  } as any,
  breadthfirst: {
    name: 'breadthfirst',
    nodeDimensionsIncludeLabels: true,
    padding: 50,
    spacingFactor: 1.2,
    directed: true,
    grid: false,
    animate: true, // Enable animation for smooth transitions
    animationDuration: 800, // 800ms for smooth easing
    animationEasing: 'ease-out', // Smooth easing
  },
  circle: {
    name: 'circle',
    nodeDimensionsIncludeLabels: true,
    padding: 50,
    spacingFactor: 1.1,
    animate: true, // Enable animation for smooth transitions
    animationDuration: 800, // 800ms for smooth easing
    animationEasing: 'ease-out', // Smooth easing
  },
  concentric: {
    name: 'concentric',
    nodeDimensionsIncludeLabels: true,
    padding: 50,
    spacingFactor: 1.2,
    animate: true, // Enable animation for smooth transitions
    animationDuration: 800, // 800ms for smooth easing
    animationEasing: 'ease-out', // Smooth easing
  },
  grid: {
    name: 'grid',
    nodeDimensionsIncludeLabels: true,
    padding: 50,
    avoidOverlap: true,
    spacingFactor: 1.2,
    animate: true, // Enable animation for smooth transitions
    animationDuration: 800, // 800ms for smooth easing
    animationEasing: 'ease-out', // Smooth easing
  },
}

type GraphLayoutOption = keyof typeof LAYOUT_OPTIONS

/**
 * Utility function to truncate text with ellipsis if too long
 */
function truncateText(text: string, maxLength: number): string {
  if (!text || text.length <= maxLength) return text
  return text.substring(0, maxLength - 3) + '...'
}

/**
 * Calculate dynamic font size based on text length
 * Shorter text = larger font (up to 14px), longer text = smaller font (down to 10px)
 */
function calculateDynamicFontSize(text: string, baseFontSize: number): number {
  if (!text) return baseFontSize
  
  const textLength = text.length
  
  // Font size range: 10px - 14px
  // Short text (1-10 chars): 14px
  // Medium text (11-30 chars): 12px
  // Long text (31+ chars): 10px
  if (textLength <= 10) {
    return 14
  } else if (textLength <= 30) {
    // Linear interpolation between 14px and 12px
    const ratio = (textLength - 10) / 20 // 0 to 1
    return Math.round(14 - (ratio * 2))
  } else {
    // Linear interpolation between 12px and 10px
    const ratio = Math.min((textLength - 30) / 30, 1) // 0 to 1, capped at 1
    return Math.round(12 - (ratio * 2))
  }
}

/**
 * Calculate node width based on text length (auto-width)
 * Min: 120px, Max: 240px
 */
function calculateNodeWidth(text: string, config: NodeConfig, fontSize: number): number {
  if (!text) return config.minWidth
  
  const charWidth = (fontSize / 12) * config.charWidth // Adjust charWidth based on font size
  const textWidth = text.length * charWidth
  const requiredWidth = textWidth + (config.padding * 2) + config.extraPadding
  
  // Clamp between min and max width
  return Math.max(config.minWidth, Math.min(config.maxWidth, requiredWidth))
}

/**
 * Utility function to estimate text dimensions
 * Now uses truncation instead of wrapping, and dynamic font sizing
 */
function estimateTextDimensions(
  text: string,
  config: NodeConfig
): { width: number; height: number; fontSize: number; truncatedText: string; fullText: string } {
  if (!text) return { width: 0, height: 0, fontSize: config.fontSize, truncatedText: '', fullText: '' }
  
  // Calculate dynamic font size based on text length
  const dynamicFontSize = calculateDynamicFontSize(text, config.fontSize)
  const charWidth = (dynamicFontSize / 12) * config.charWidth
  
  // Calculate available width for text (node width - padding)
  const availableWidth = config.maxWidth - (config.padding * 2) - config.extraPadding
  const maxChars = Math.floor(availableWidth / charWidth)
  
  // Truncate text if needed
  const truncatedText = truncateText(text, maxChars)
  
  // Calculate node width (auto-width based on text)
  const nodeWidth = calculateNodeWidth(truncatedText, config, dynamicFontSize)
  
  // Calculate text height (single line)
  const textHeight = dynamicFontSize * config.lineHeight
  
  return {
    width: nodeWidth,
    height: textHeight,
    fontSize: dynamicFontSize,
    truncatedText,
    fullText: text
  }
}

const GraphVisualization = ({
  cluster,
  namespace,
  height = '600px',
  onLoadingChange,
  nodeTypeFilters,
  connectionTypeFilters,
  layout = 'cose',
  autoFitEnabled = true,
  onNodeSelect,
}: GraphVisualizationProps) => {
  const [container, setContainer] = useState<HTMLDivElement | null>(null)
  const containerRef = useCallback((node: HTMLDivElement | null) => {
    if (node) {
      setContainer(node)
    }
  }, [])
  
  const cyRef = useRef<Core | null>(null)
  const [isCytoscapeReady, setIsCytoscapeReady] = useState(false)
  const { data, isLoading, error, refetch } = useGraph({ cluster, namespace })
  
  
  // Notify parent of loading state changes
  useEffect(() => {
    if (onLoadingChange) {
      onLoadingChange(isLoading)
    }
  }, [isLoading, onLoadingChange])
  const [zoomLevel, setZoomLevel] = useState(1)
  const [selectedNodeId, setSelectedNodeId] = useState<string | null>(null)
  const [selectedNodeType, setSelectedNodeType] = useState<string | null>(null)
  const [selectedNodeData, setSelectedNodeData] = useState<Record<string, any> | null>(null)
  const [tooltipPosition, setTooltipPosition] = useState<TooltipPosition | null>(null)
  const [tooltipVisible, setTooltipVisible] = useState(false)
  const [searchQuery, setSearchQuery] = useState<string>('')
  const [searchResults, setSearchResults] = useState<string[]>([])
  
  // Get ServiceAccount details when ServiceAccount node is selected
  const { data: serviceAccount, isLoading: isLoadingDetails } = useServiceAccount(
    (selectedNodeType === 'serviceaccount' && selectedNodeData?.id) ? String(selectedNodeData.id) : ''
  )

  // Memoize nodes and edges conversion for performance
  const { nodes, edges } = useMemo(() => {
    if (!data || !Array.isArray(data.nodes) || !Array.isArray(data.edges)) {
      return { nodes: [], edges: [] }
    }

    // Convert data to Cytoscape format
    const convertedNodes: NodeDefinition[] = data.nodes.map((node) => {
      const nodeData: any = {
        id: node.id,
        label: node.label || '', // Ensure label is never undefined
        type: node.type,
      }
      
      // Get configuration for this node type, fallback to namespace if unknown
      const nodeConfig = NODE_CONFIGS[node.type] || NODE_CONFIGS.namespace
      
      // Estimate text dimensions with truncation and dynamic font sizing
      const { width: estimatedWidth, height: textHeight, fontSize: dynamicFontSize, truncatedText, fullText } = 
        estimateTextDimensions(node.label || '', nodeConfig)
      
      // Calculate final height based on shape
      let estimatedHeight: number
      
      if (node.type === 'serviceaccount' || node.type === 'role' || node.type === 'clusterrole') {
        // For ellipse/diamond shapes
        const baseHeight = estimatedWidth * nodeConfig.heightRatio
        const textRequiredHeight = textHeight + (nodeConfig.padding * 2)
        estimatedHeight = Math.max(baseHeight, textRequiredHeight)
      } else {
        // For rectangle shapes
        const baseHeight = estimatedWidth * nodeConfig.heightRatio
        const textRequiredHeight = textHeight + (nodeConfig.padding * 2)
        estimatedHeight = Math.max(baseHeight, textRequiredHeight)
      }
      
      // Store all configuration in node data for style access
      nodeData.width = Math.round(estimatedWidth)
      nodeData.height = Math.round(estimatedHeight)
      nodeData.textMaxWidth = Math.round(estimatedWidth - (nodeConfig.padding * 2) - 16) // Safety margin
      nodeData.fontSize = dynamicFontSize // Use dynamic font size
      nodeData.textMarginY = nodeConfig.textMarginY
      nodeData.fontWeight = nodeConfig.fontWeight
      nodeData.truncatedText = truncatedText // Store truncated text for display
      nodeData.fullText = fullText // Store full text for tooltip
      nodeData.backgroundColor = nodeConfig.backgroundColor
      nodeData.borderColor = nodeConfig.borderColor
      nodeData.shadowColor = nodeConfig.shadowColor
      
      // Store additional metadata if available
      if (node.data) {
        Object.keys(node.data).forEach((key) => {
          if (key === 'id' && node.data![key]) {
            nodeData.serviceAccountId = node.data![key]
          } else {
            nodeData[key] = node.data![key]
          }
        })
      }
      
      return { data: nodeData }
    })

    let filteredNodes = convertedNodes

    if (nodeTypeFilters) {
      filteredNodes = convertedNodes.filter((node) => {
        const typeKey = String(node.data.type || '').toLowerCase()
        if (!(typeKey in nodeTypeFilters)) {
          return true
        }
        return nodeTypeFilters[typeKey] !== false
      })
    }

    const nodeIds = new Set(filteredNodes.map((n) => n.data.id))

    let filteredEdges: EdgeDefinition[] = data.edges
      .filter((edge) => nodeIds.has(edge.source) && nodeIds.has(edge.target))
      .map((edge) => ({
        data: {
          id: edge.id,
          source: edge.source,
          target: edge.target,
          type: edge.type,
          ...(edge.data || {}),
        },
      }))

    if (connectionTypeFilters) {
      filteredEdges = filteredEdges.filter((edge) => {
        const typeKey = String(edge.data.type || '').toLowerCase()
        if (!(typeKey in connectionTypeFilters)) {
          return true
        }
        return connectionTypeFilters[typeKey] !== false
      })
    }

    console.log('Converted nodes and edges:', {
      convertedNodes: convertedNodes.length,
      filteredNodes: filteredNodes.length,
      filteredEdges: filteredEdges.length,
      sampleConvertedNode: filteredNodes[0],
      sampleConvertedEdge: filteredEdges[0]
    })

    return { nodes: filteredNodes, edges: filteredEdges }
  }, [data, nodeTypeFilters, connectionTypeFilters])

  const originalNodeCount = data?.nodes?.length ?? 0
  const originalEdgeCount = data?.edges?.length ?? 0

  const runLayout = useCallback(
    (fitOverride = false) => {
      if (!cyRef.current) return
      if (cyRef.current.nodes().length === 0) {
        console.warn('No nodes to layout')
        return
      }

      try {
        const baseOptions = LAYOUT_OPTIONS[layout] ?? LAYOUT_OPTIONS.cose
        const layoutOptions: LayoutOptions = {
          ...baseOptions,
        }

        console.log('Running layout:', layout, 'with options:', layoutOptions)
        const layoutInstance = cyRef.current.layout(layoutOptions)
        
        // Listen for layout completion
        layoutInstance.one('layoutstop', () => {
          console.log('Layout completed')
          if (autoFitEnabled || fitOverride) {
            if (cyRef.current) {
              // Small delay to ensure rendering is complete
              setTimeout(() => {
                if (cyRef.current) {
                  const nodes = cyRef.current.nodes()
                  const edges = cyRef.current.edges()
                  
                  console.log('Before fit:', {
                    zoom: cyRef.current.zoom(),
                    pan: cyRef.current.pan(),
                    nodes: nodes.length,
                    edges: edges.length,
                    containerSize: {
                      width: container?.offsetWidth,
                      height: container?.offsetHeight
                    }
                  })
                  
                  // Check node positions
                  if (nodes.length > 0) {
                    const firstNode = nodes[0]
                    const lastNode = nodes[nodes.length - 1]
                    console.log('Node positions:', {
                      first: firstNode.position(),
                      last: lastNode.position(),
                      firstRendered: firstNode.renderedPosition(),
                      lastRendered: lastNode.renderedPosition()
                    })
                  }
                  
                  // Get bounding box of all elements
                  const extent = cyRef.current.extent()
                  console.log('Graph extent:', extent)
                  
                  // Force resize first
                  cyRef.current.resize()
                  
                  // Fit with padding
                  cyRef.current.fit(cyRef.current.elements(), 50)
                  
                  // Force another resize after fit
                  cyRef.current.resize()
                  
                  setZoomLevel(cyRef.current.zoom())
                  console.log('After fit:', {
                    zoom: cyRef.current.zoom(),
                    pan: cyRef.current.pan(),
                    nodesVisible: nodes.length
                  })
                }
              }, 100)
            }
          }
        })
        
        // Run layout with error handling
        layoutInstance.run()
        
        // Fallback timeout in case layoutstop event doesn't fire
        setTimeout(() => {
          if (autoFitEnabled || fitOverride) {
            if (cyRef.current) {
              const currentZoom = cyRef.current.zoom()
              if (currentZoom === 1 || currentZoom === 0) {
                console.log('Fallback: Fitting to view after timeout')
                cyRef.current.fit(undefined, 50)
                setZoomLevel(cyRef.current.zoom())
              }
            }
          }
        }, 1000)
      } catch (error) {
        console.error('Layout error:', error, 'Falling back to cose layout')
        // Fallback to cose layout if there's an error
        try {
          const fallbackOptions = LAYOUT_OPTIONS.cose
          cyRef.current.layout(fallbackOptions).run()
          if (autoFitEnabled || fitOverride) {
            cyRef.current.fit(undefined, 50)
            setZoomLevel(cyRef.current.zoom())
          }
        } catch (fallbackError) {
          console.error('Fallback layout also failed:', fallbackError)
        }
      }
    },
    [layout, autoFitEnabled]
  )

  // Update graph when nodes/edges change OR when Cytoscape becomes ready
  useEffect(() => {
    if (!cyRef.current || !isCytoscapeReady) {
      return
    }

    if (nodes.length === 0 && edges.length === 0) {
      return
    }
    cyRef.current.batch(() => {
      cyRef.current!.elements().remove()
      if (nodes.length > 0 || edges.length > 0) {
        cyRef.current!.add([...nodes, ...edges])
      }
    })

    // Resize to ensure container is properly sized
    cyRef.current.resize()

    if (nodes.length > 0) {
      // Small delay to ensure elements are rendered
      setTimeout(() => {
        if (cyRef.current && cyRef.current.nodes().length > 0) {
          runLayout()
        }
      }, 50)
    }

    if (selectedNodeId && cyRef.current) {
      const matching = cyRef.current
        .nodes()
        .filter((n) => {
          const saId = n.data('serviceAccountId')
          return saId === selectedNodeId || n.id() === selectedNodeId
        })
      if (matching.length === 0) {
        setSelectedNodeId(null)
        setSelectedNodeType(null)
        setSelectedNodeData(null)
        setTooltipVisible(false)
      }
    }
  }, [nodes, edges, runLayout, selectedNodeId, isCytoscapeReady])

  useEffect(() => {
    if (cyRef.current && cyRef.current.nodes().length > 0) {
      console.log('Layout changed, re-running layout:', layout)
      runLayout()
    } else {
      console.log('Layout changed but no nodes yet, will run when nodes are added')
    }
  }, [layout, autoFitEnabled, runLayout])

  useEffect(() => {
    if (!nodeTypeFilters || !selectedNodeType) return
    const typeKey = selectedNodeType.toLowerCase()
    if (nodeTypeFilters[typeKey] === false && selectedNodeId) {
      setSelectedNodeId(null)
      setSelectedNodeType(null)
      setSelectedNodeData(null)
      setTooltipVisible(false)
    }
  }, [nodeTypeFilters, selectedNodeId, selectedNodeType])

  // Use ResizeObserver to wait for container to have size
  useEffect(() => {
    console.log('🎬 Initialization useEffect triggered', {
      hasContainer: !!container,
      containerElement: container,
      containerSize: container ? {
        width: container.offsetWidth,
        height: container.offsetHeight
      } : null
    })
    
    if (!container) {
      console.log('❌ container is null, skipping initialization')
      return
    }
    let resizeObserver: ResizeObserver | null = null
    let initTimeout: ReturnType<typeof setTimeout> | null = null
    let isCleanedUp = false

    const checkAndInit = () => {
      if (cyRef.current || isCleanedUp) {
        console.log('⏭️ Skipping init:', { 
          alreadyInitialized: !!cyRef.current, 
          cleanedUp: isCleanedUp 
        })
        return
      }
      
      const width = container.offsetWidth || container.clientWidth || container.getBoundingClientRect().width
      const height = container.offsetHeight || container.clientHeight || container.getBoundingClientRect().height
      
      // Also check computed style height
      const computedStyle = window.getComputedStyle(container)
      const computedHeight = parseInt(computedStyle.height) || parseInt(computedStyle.minHeight) || 0
      
      const finalHeight = height || computedHeight
      
      console.log('🔍 Checking container size:', { 
        offsetWidth: container.offsetWidth,
        offsetHeight: container.offsetHeight,
        clientWidth: container.clientWidth,
        clientHeight: container.clientHeight,
        boundingRect: container.getBoundingClientRect(),
        computedHeight,
        finalWidth: width,
        finalHeight,
        containerElement: container,
        parentElement: container.parentElement,
        parentSize: {
          width: container.parentElement?.offsetWidth,
          height: container.parentElement?.offsetHeight
        }
      })
      
      if (width > 0 && finalHeight > 0) {
        console.log('Container has size, initializing Cytoscape:', { width, height: finalHeight })
        initializeCytoscape()
        if (resizeObserver) {
          resizeObserver.disconnect()
          resizeObserver = null
        }
        if (initTimeout) {
          clearTimeout(initTimeout)
          initTimeout = null
        }
      } else {
        console.log('Container has no size yet, waiting...', { width, height: finalHeight })
      }
    }

    // Try to initialize immediately
    checkAndInit()

    // If not initialized, use ResizeObserver to wait for container size
    if (!cyRef.current && !isCleanedUp) {
      resizeObserver = new ResizeObserver(() => {
        if (!isCleanedUp) {
          checkAndInit()
        }
      })
      resizeObserver.observe(container)

      // Fallback timeout in case ResizeObserver doesn't fire
      initTimeout = setTimeout(() => {
        if (!cyRef.current && !isCleanedUp) {
          console.warn('Timeout waiting for container size, forcing init')
          checkAndInit()
        }
      }, 1000)
    }

    function initializeCytoscape() {
      if (!container || cyRef.current) return
      
      const width = container.offsetWidth || container.clientWidth
      const height = container.offsetHeight || container.clientHeight
      
      if (width === 0 || height === 0) {
        console.error('Cannot initialize Cytoscape: container has no size', {
          offsetWidth: container.offsetWidth,
          offsetHeight: container.offsetHeight,
          clientWidth: container.clientWidth,
          clientHeight: container.clientHeight
        })
        return
      }
      
      console.log('🚀 Initializing Cytoscape with container size:', { width, height })
      
      cyRef.current = cytoscape({
        container: container,
        style: [
          // Base node style
          {
            selector: 'node',
            style: {
              'background-color': 'data(backgroundColor)',
              // Use truncatedText if available, otherwise use label
              'label': (ele: NodeSingular) => {
                const truncated = ele.data('truncatedText')
                return truncated || ele.data('label') || ''
              },
              'width': 'data(width)',
              'height': 'data(height)',
              'text-valign': 'center',
              'text-halign': 'center',
              'color': '#ffffff',
              // Fixed font size (no zoom scaling) - dynamic based on text length only
              'font-size': (ele: NodeSingular) => {
                const fontSize = ele.data('fontSize') || 12
                return `${fontSize}px`
              },
              // Fix for fontWeight type
              'font-weight': (ele: NodeSingular) => ele.data('fontWeight'),
              'font-family': 'system-ui, -apple-system, "Segoe UI", Roboto, sans-serif',
              'text-wrap': 'none', // No wrapping - use truncation instead
              'text-max-width': 'data(textMaxWidth)',
              'text-outline-width': 2, // Outline for better readability
              'text-outline-color': '#000000',
              'text-outline-opacity': 0.8,
              // Fix for textMarginY type
              'text-margin-y': (ele: NodeSingular) => ele.data('textMarginY'),
              'padding': 12,
              'shape': 'round-rectangle',
              'border-width': 1.5,
              'border-color': 'data(borderColor)',
              'border-opacity': 0.6,
              // Enhanced shadow effect with type assertions to avoid TypeScript errors
              'shadow-blur': 6,
              'shadow-color': 'data(shadowColor)',
              'shadow-opacity': 0.7,
              'shadow-offset-x': 0,
              'shadow-offset-y': 2.5,
            },
          } as any,
          // Node type-specific styles
          {
            selector: 'node[type="serviceaccount"]',
            style: {
              'shape': 'ellipse',
            },
          },
          {
            selector: 'node[type="role"]',
            style: {
              'shape': 'diamond',
            },
          },
          {
            selector: 'node[type="clusterrole"]',
            style: {
              'shape': 'diamond',
            },
          },
          {
            selector: 'node[type="namespace"]',
            style: {
              'shape': 'round-rectangle',
            },
          },
          {
            selector: 'node[type="cluster"]',
            style: {
              'shape': 'round-rectangle',
            },
          },
          // Enhance text readability for nodes with multiple lines
          {
            selector: 'node[lineCount > 1]',
            style: {
              'text-outline-width': 2, // Stronger outline for multi-line text
              'text-outline-color': '#000000a0',
              'text-outline-opacity': 0.8,
            },
          },
          // Edge styles - curved with gradient effect
          {
            selector: 'edge',
            style: {
              'width': 2,
              'line-color': '#94a3b8',
              'target-arrow-color': '#64748b',
              'target-arrow-shape': 'triangle',
              'target-arrow-size': 6,
              'curve-style': 'unbundled-bezier',
              'control-point-distances': [20, -20],
              'control-point-weights': [0.25, 0.75],
              'arrow-scale': 1,
              'opacity': 0.7,
              'line-cap': 'round',
            },
          },
          // Highlight styles for selection
          {
            selector: 'node:selected',
            style: {
              'border-width': 3,
              'border-color': '#f39c12',
              'border-opacity': 0.9,
              'shadow-blur': 12,
              'shadow-color': '#f39c1280',
              'shadow-opacity': 1,
              'shadow-offset-y': 3,
            } as any,
          },
          {
            selector: 'edge:selected',
            style: {
              'width': 2.5,
              'line-color': '#f39c12',
              'target-arrow-color': '#f39c12',
              'opacity': 1,
            },
          },
          // Hover styles
          {
            selector: 'node:active',
            style: {
              'overlay-color': '#3498db',
              'overlay-opacity': 0.2,
              'overlay-padding': 10,
            },
          },
          // Connected edges and nodes highlight
          {
            selector: '.highlighted',
            style: {
              'line-color': '#f39c12',
              'target-arrow-color': '#f39c12',
              'width': 2.5,
              'opacity': 1,
              'z-index': 9999,
            },
          },
          {
            selector: '.highlighted-node',
            style: {
              'border-width': 2,
              'border-color': '#f39c12',
              'border-opacity': 0.8,
              'shadow-blur': 10,
              'shadow-color': '#f39c1280',
              'shadow-opacity': 0.8,
              'z-index': 9999,
            } as any,
          },
          // Semi-transparent for non-highlighted elements
          {
            selector: '.faded',
            style: {
              'opacity': 0.3,
              'z-index': 1,
            },
          },
          // Search highlight
          {
            selector: '.search-highlight',
            style: {
              'border-width': 3,
              'border-color': '#3498db',
              'border-opacity': 1,
              'shadow-blur': 15,
              'shadow-color': '#3498db80',
              'shadow-opacity': 1,
              'z-index': 10000,
            } as any,
          },
        ],
        // Don't set initial layout here, we'll run it manually after elements are added
        layout: {
          name: 'preset',
        },
        // Interaction settings with pan/zoom inertia
        minZoom: 0.2,
        maxZoom: 3,
        wheelSensitivity: 0.3,
        autoungrabify: false,
        autounselectify: false,
        // Enable pan/zoom inertia for smoother interaction
        panningEnabled: true,
        userPanningEnabled: true,
        boxSelectionEnabled: true,
        // Animation settings for smooth transitions
        motionBlur: false,
        pixelRatio: 'auto',
      })
      
      console.log('✅ Cytoscape initialized successfully!', {
        cyInitialized: !!cyRef.current,
        containerWidth: width,
        containerHeight: height
      })
      
      // Mark as ready to trigger update effect
      setIsCytoscapeReady(true)
      
      // Enable pan inertia (smooth deceleration after pan)
      if (cyRef.current) {
        let panVelocity = { x: 0, y: 0 }
        let lastPanTime = Date.now()
        let lastPanPosition = { x: 0, y: 0 }
        let panAnimationFrame: number | null = null
        
        cyRef.current.on('pan', () => {
          const now = Date.now()
          const currentPan = cyRef.current!.pan()
          const deltaTime = now - lastPanTime
          
          if (deltaTime > 0) {
            panVelocity = {
              x: (currentPan.x - lastPanPosition.x) / deltaTime,
              y: (currentPan.y - lastPanPosition.y) / deltaTime
            }
          }
          
          lastPanTime = now
          lastPanPosition = currentPan
        })
        
        cyRef.current.on('panfree', () => {
          // Apply inertia after pan ends
          const friction = 0.95
          const minVelocity = 0.1
          
          const applyInertia = () => {
            if (Math.abs(panVelocity.x) < minVelocity && Math.abs(panVelocity.y) < minVelocity) {
              panVelocity = { x: 0, y: 0 }
              if (panAnimationFrame) {
                cancelAnimationFrame(panAnimationFrame)
                panAnimationFrame = null
              }
              return
            }
            
            if (cyRef.current) {
              const currentPan = cyRef.current.pan()
              cyRef.current.pan({
                x: currentPan.x + panVelocity.x * 16,
                y: currentPan.y + panVelocity.y * 16
              })
              
              panVelocity.x *= friction
              panVelocity.y *= friction
              
              panAnimationFrame = requestAnimationFrame(applyInertia)
            }
          }
          
          if (Math.abs(panVelocity.x) > minVelocity || Math.abs(panVelocity.y) > minVelocity) {
            panAnimationFrame = requestAnimationFrame(applyInertia)
          }
        })
      }

      // Event handlers for all node types
      cyRef.current.on('tap', 'node', (e) => {
        const node = e.target
        const nodeType = node.data('type')
        const nodeId = node.id()
        
        // Extract node data
        const nodeData: Record<string, any> = {
          id: node.data('serviceAccountId') || nodeId,
          name: node.data('label'),
          type: nodeType,
          cluster: node.data('cluster'),
          namespace: node.data('namespace'),
          uid: node.data('uid'),
        }
        
        // Store all node data properties
        Object.keys(node.data()).forEach((key) => {
          if (!nodeData[key]) {
            nodeData[key] = node.data(key)
          }
        })
        
        setSelectedNodeId(nodeId)
        setSelectedNodeType(nodeType)
        setSelectedNodeData(nodeData)
        
        // Notify parent component
        if (onNodeSelect) {
          onNodeSelect(nodeId, nodeType, nodeData)
        }
        
        // Save position for tooltip
        const position = e.renderedPosition || e.position
        setTooltipPosition({
          x: position.x,
          y: position.y
        })
        setTooltipVisible(true)
      })

      // Event for background click to dismiss tooltip
      cyRef.current.on('tap', (e) => {
        if (e.target === cyRef.current) {
          setTooltipVisible(false)
          setSelectedNodeId(null)
          setSelectedNodeType(null)
          setSelectedNodeData(null)
          
          // Reset all styles
          cyRef.current!.elements().removeClass('highlighted highlighted-node faded')
        }
      })

      // Show tooltip and highlight connected nodes and edges on mouseover
      cyRef.current.on('mouseover', 'node', (e) => {
        const node = e.target
        const fullText = node.data('fullText') || node.data('label') || ''
        const nodeType = node.data('type') || ''
        
        // Show tooltip with full text
        if (fullText && container) {
          const position = e.renderedPosition || e.position
          const containerRect = container.getBoundingClientRect()
          
          setTooltipPosition({
            x: containerRect.left + position.x,
            y: containerRect.top + position.y
          })
          setSelectedNodeData({ name: fullText, type: nodeType })
          setTooltipVisible(true)
        }
        
        // Style the hovered node using 'as any' to bypass TypeScript checks
        node.style({
          'border-width': 2,
          'border-color': '#3498db',
          'border-opacity': 0.8,
          'shadow-blur': 10,
          'shadow-color': node.data('shadowColor'),
          'shadow-opacity': 0.8,
        } as any)
        
        // Highlight connected elements
        const connectedEdges = node.connectedEdges()
        const connectedNodes = connectedEdges.connectedNodes().not(node)
        
        connectedEdges.addClass('highlighted')
        connectedNodes.addClass('highlighted-node')
        
        // Add faded class to elements not connected to this node
        cyRef.current!.elements()
          .not(connectedEdges)
          .not(connectedNodes)
          .not(node)
          .addClass('faded')
      })

      // Reset styles and hide tooltip on mouseout
      cyRef.current.on('mouseout', 'node', (e) => {
        const node = e.target
        
        // Hide tooltip
        setTooltipVisible(false)
        
        // Reset node style if not selected using 'as any' to bypass TypeScript checks
        if (!node.selected()) {
          node.style({
            'border-width': 1.5,
            'border-color': node.data('borderColor'),
            'border-opacity': 0.6,
            'shadow-blur': 6,
            'shadow-color': node.data('shadowColor'),
            'shadow-opacity': 0.7,
          } as any)
        }
        
        // Reset highlight classes
        cyRef.current!.elements().removeClass('highlighted highlighted-node faded')
      })

      // Update zoom level on zoom events (no font scaling)
      cyRef.current.on('zoom', () => {
        if (cyRef.current) {
          setZoomLevel(cyRef.current.zoom())
        }
      })

      // Minimap will be implemented as a simple overview component
      // For now, we'll skip the minimap initialization
    }

    // Setup initial elements - only if Cytoscape is initialized
    if (cyRef.current && (nodes.length > 0 || edges.length > 0)) {
      console.log('Setting up elements:', { 
        nodes: nodes.length, 
        edges: edges.length, 
        layout,
        cytoscapeInitialized: !!cyRef.current,
        containerSize: {
          width: container?.offsetWidth,
          height: container?.offsetHeight
        }
      })
      
      // Validate nodes and edges format
      const validNodes = nodes.filter(n => n.data && n.data.id)
      const validEdges = edges.filter(e => e.data && e.data.source && e.data.target)
      
      console.log('Valid elements:', {
        validNodes: validNodes.length,
        validEdges: validEdges.length,
        invalidNodes: nodes.length - validNodes.length,
        invalidEdges: edges.length - validEdges.length
      })
      
      if (validNodes.length === 0) {
        console.error('No valid nodes to add!', {
          totalNodes: nodes.length,
          sampleNode: nodes[0]
        })
        return
      }
      
      cyRef.current.batch(() => {
        cyRef.current!.elements().remove()
        const added = cyRef.current!.add([...validNodes, ...validEdges])
        console.log('Elements added to Cytoscape:', {
          total: added.length,
          nodes: added.nodes().length,
          edges: added.edges().length
        })
      })
      
      // Verify nodes were added
      const addedNodes = cyRef.current.nodes()
      const addedEdges = cyRef.current.edges()
      console.log('Verification - Nodes in graph:', addedNodes.length, 'Edges in graph:', addedEdges.length)
      
      if (addedNodes.length > 0) {
        const firstNode = addedNodes[0]
        console.log('First node data:', {
          id: firstNode.id(),
          position: firstNode.position(),
          renderedPosition: firstNode.renderedPosition(),
          visible: firstNode.visible(),
          width: firstNode.width(),
          height: firstNode.height(),
          data: firstNode.data()
        })
      }
      
      // Ensure elements are added before running layout
      setTimeout(() => {
        if (cyRef.current && cyRef.current.nodes().length > 0) {
          console.log('Running layout after elements added:', cyRef.current.nodes().length, 'nodes')
          // Force a render before running layout
          cyRef.current.resize()
          runLayout(true)
        } else {
          console.error('No nodes found after adding elements!', {
            expectedNodes: validNodes.length,
            actualNodes: cyRef.current?.nodes().length || 0
          })
        }
      }, 150)
    } else {
      if (!cyRef.current) {
        console.warn('Cytoscape not initialized, cannot add elements')
      } else {
        console.warn('No elements to add:', { nodes: nodes.length, edges: edges.length })
      }
    }

    return () => {
      isCleanedUp = true
      if (resizeObserver) {
        resizeObserver.disconnect()
        resizeObserver = null
      }
      if (initTimeout) {
        clearTimeout(initTimeout)
        initTimeout = null
      }
      // Don't destroy cyRef here, let it persist
      // Only destroy on component unmount
    }
  }, [container]) // Add container as dependency to re-run when it's set

  // Cleanup on component unmount
  useEffect(() => {
    return () => {
      if (cyRef.current) {
        console.log('🧹 Cleaning up Cytoscape on unmount')
        cyRef.current.destroy()
        cyRef.current = null
      }
    }
  }, [])

  // Zoom controls
  const handleZoomIn = useCallback(() => {
    if (!cyRef.current) return
    const newZoom = cyRef.current.zoom() * 1.2
    cyRef.current.zoom({
      level: newZoom,
      renderedPosition: { x: (container?.offsetWidth || 0) / 2, y: (container?.offsetHeight || 0) / 2 }
    })
    setZoomLevel(newZoom)
  }, [container])

  const handleZoomOut = useCallback(() => {
    if (!cyRef.current) return
    const newZoom = cyRef.current.zoom() / 1.2
    cyRef.current.zoom({
      level: newZoom,
      renderedPosition: { x: (container?.offsetWidth || 0) / 2, y: (container?.offsetHeight || 0) / 2 }
    })
    setZoomLevel(newZoom)
  }, [container])

  const handleFit = useCallback(() => {
    if (!cyRef.current) return
    cyRef.current.fit(undefined, 50)
    setZoomLevel(cyRef.current.zoom())
  }, [])

  const handleReset = useCallback(() => {
    if (!cyRef.current) return
    
    // Reset view
    cyRef.current.fit(undefined, 50)
    
    // Reset selection
    cyRef.current.elements().unselect()
    setSelectedNodeId(null)
    setSelectedNodeType(null)
    setSelectedNodeData(null)
    setTooltipVisible(false)
    
    // Reset classes
    cyRef.current.elements().removeClass('highlighted highlighted-node faded')
    
    setZoomLevel(cyRef.current.zoom())
  }, [])

  const handleRefresh = useCallback(() => {
    refetch()
  }, [refetch])

  // Search nodes
  const handleSearch = useCallback((query: string) => {
    setSearchQuery(query)
    if (!cyRef.current || !query.trim()) {
      setSearchResults([])
      cyRef.current?.elements().removeClass('search-highlight')
      return
    }

    const lowerQuery = query.toLowerCase()
    const matchingNodes: string[] = []
    
    cyRef.current.nodes().forEach((node) => {
      const label = node.data('label') || ''
      const nodeId = node.id()
      
      if (label.toLowerCase().includes(lowerQuery) || nodeId.toLowerCase().includes(lowerQuery)) {
        matchingNodes.push(nodeId)
        node.addClass('search-highlight')
      } else {
        node.removeClass('search-highlight')
      }
    })

    setSearchResults(matchingNodes)

    // Focus on first match if any
    if (matchingNodes.length > 0 && cyRef.current) {
      const firstMatch = cyRef.current.$(`#${matchingNodes[0]}`)
      if (firstMatch.length > 0) {
        cyRef.current.animate({
          center: { eles: firstMatch },
          zoom: Math.max(cyRef.current.zoom(), 1.5),
        }, {
          duration: 500,
        })
      }
    }
  }, [])

  // Export screenshot
  const handleExportScreenshot = useCallback((format: 'png' | 'svg' = 'png') => {
    if (!cyRef.current) return

    if (format === 'png') {
      try {
        const png = (cyRef.current as any).png({ 
          output: 'blob',
          bg: 'white',
          full: true,
          scale: 2,
        })
        
        if (png && typeof png.then === 'function') {
          png.then((blob: Blob) => {
            const url = URL.createObjectURL(blob)
            const link = document.createElement('a')
            link.href = url
            link.download = `ksam-graph-${new Date().toISOString().split('T')[0]}.png`
            document.body.appendChild(link)
            link.click()
            document.body.removeChild(link)
            URL.revokeObjectURL(url)
          }).catch((err: Error) => {
            console.error('Failed to export PNG:', err)
          })
        } else {
          // Fallback: use data URL
          const dataUrl = (cyRef.current as any).png({ 
            output: 'base64uri',
            bg: 'white',
            full: true,
            scale: 2,
          })
          const link = document.createElement('a')
          link.href = dataUrl
          link.download = `ksam-graph-${new Date().toISOString().split('T')[0]}.png`
          document.body.appendChild(link)
          link.click()
          document.body.removeChild(link)
        }
      } catch (err) {
        console.error('Failed to export PNG:', err)
      }
    } else {
      try {
        const svg = (cyRef.current as any).svg({ 
          full: true,
          scale: 2,
        })
        
        if (svg) {
          const blob = new Blob([svg], { type: 'image/svg+xml' })
          const url = URL.createObjectURL(blob)
          const link = document.createElement('a')
          link.href = url
          link.download = `ksam-graph-${new Date().toISOString().split('T')[0]}.svg`
          document.body.appendChild(link)
          link.click()
          document.body.removeChild(link)
          URL.revokeObjectURL(url)
        }
      } catch (err) {
        console.error('Failed to export SVG:', err)
      }
    }
  }, [])

  // Keyboard shortcuts
  useEffect(() => {
    const handleKeyPress = (e: KeyboardEvent) => {
      // Skip if inside an input or textarea
      if (
        e.target instanceof HTMLInputElement ||
        e.target instanceof HTMLTextAreaElement
      ) {
        return
      }

      switch (e.key) {
        case '+':
        case '=':
          e.preventDefault()
          handleZoomIn()
          break
        case '-':
        case '_':
          e.preventDefault()
          handleZoomOut()
          break
        case '0':
          e.preventDefault()
          handleFit()
          break
        case 'r':
        case 'R':
          if (e.ctrlKey || e.metaKey) {
            e.preventDefault()
            handleRefresh()
          }
          break
        case 'Escape':
          setTooltipVisible(false)
          setSelectedNodeId(null)
          setSelectedNodeType(null)
          setSelectedNodeData(null)
          if (cyRef.current) {
            cyRef.current.elements().unselect()
            cyRef.current.elements().removeClass('highlighted highlighted-node faded')
          }
          break
      }
    }

    window.addEventListener('keydown', handleKeyPress)
    return () => window.removeEventListener('keydown', handleKeyPress)
  }, [handleZoomIn, handleZoomOut, handleFit, handleRefresh])

  if (isLoading) {
    return (
      <div className="flex items-center justify-center" style={{ height }}>
        <div className="text-gray-500 flex items-center">
          <svg className="animate-spin -ml-1 mr-3 h-5 w-5 text-blue-500" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
            <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
            <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
          </svg>
          <span>Loading graph...</span>
        </div>
      </div>
    )
  }

  if (error) {
    return (
      <div className="flex items-center justify-center" style={{ height }}>
        <div className="text-red-500 flex items-center">
          <svg className="h-5 w-5 mr-2" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
          <span>Error loading graph: {String(error)}</span>
        </div>
      </div>
    )
  }

  if (!data || data.nodes.length === 0) {
    return (
      <div className="flex items-center justify-center border border-gray-300 rounded-lg" style={{ height }}>
        <div className="text-gray-500 flex flex-col items-center">
          <svg className="h-8 w-8 mb-2 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z" />
          </svg>
          <span>No data available</span>
          {namespace && (
            <p className="text-sm mt-2 text-gray-400">Try adjusting the namespace filter</p>
          )}
        </div>
      </div>
    )
  }

  if (data && originalNodeCount > 0 && nodes.length === 0) {
    return (
      <div className="flex items-center justify-center border border-gray-300 rounded-lg" style={{ height }}>
        <div className="text-gray-500 flex flex-col items-center">
          <svg className="h-8 w-8 mb-2 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z" />
          </svg>
          <span>No nodes match the current filters</span>
          <p className="text-sm mt-2 text-gray-400 text-center">Adjust node type or connection filters to see results.</p>
        </div>
      </div>
    )
  }

  return (
    <div className="w-full h-full border border-gray-300 rounded-lg overflow-hidden relative bg-gray-50">
      {/* Search Bar */}
      <div className="absolute top-4 left-4 z-10 bg-white/95 backdrop-blur-sm rounded-lg shadow-md border border-gray-200 p-2 min-w-[280px]">
        <div className="flex items-center gap-2">
          <svg className="w-4 h-4 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
          </svg>
          <input
            type="text"
            value={searchQuery}
            onChange={(e) => handleSearch(e.target.value)}
            placeholder="Search nodes..."
            className="flex-1 text-sm border-0 focus:outline-none focus:ring-0 bg-transparent text-gray-700 placeholder-gray-400"
          />
          {searchQuery && (
            <button
              onClick={() => {
                setSearchQuery('')
                handleSearch('')
              }}
              className="text-gray-400 hover:text-gray-600"
              title="Clear search"
            >
              <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
              </svg>
            </button>
          )}
        </div>
        {searchResults.length > 0 && (
          <div className="mt-2 text-xs text-gray-600">
            Found {searchResults.length} node{searchResults.length !== 1 ? 's' : ''}
          </div>
        )}
      </div>

      {/* Toolbar */}
      <div className="absolute top-4 right-4 z-10 bg-white/95 backdrop-blur-sm rounded-lg shadow-md border border-gray-200 p-2 flex flex-col gap-1.5">
        <button
          onClick={handleZoomIn}
          className="p-2.5 text-xs font-medium text-gray-700 bg-white border border-gray-300 rounded-md hover:bg-gray-50 hover:border-gray-400 focus:outline-none focus:ring-2 focus:ring-blue-500 transition-colors"
          title="Zoom In (+)"
        >
          <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4v16m8-8H4" />
          </svg>
        </button>
        <button
          onClick={handleZoomOut}
          className="p-2.5 text-xs font-medium text-gray-700 bg-white border border-gray-300 rounded-md hover:bg-gray-50 hover:border-gray-400 focus:outline-none focus:ring-2 focus:ring-blue-500 transition-colors"
          title="Zoom Out (-)"
        >
          <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M20 12H4" />
          </svg>
        </button>
        <button
          onClick={handleFit}
          className="p-2.5 text-xs font-medium text-gray-700 bg-white border border-gray-300 rounded-md hover:bg-gray-50 hover:border-gray-400 focus:outline-none focus:ring-2 focus:ring-blue-500 transition-colors"
          title="Fit to View (0)"
        >
          <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 8V4m0 0h4M4 4l5 5m11-1V4m0 0h-4m4 0l-5 5M4 16v4m0 0h4m-4 0l5-5m11 5l-5-5m5 5v-4m0 4h-4" />
          </svg>
        </button>
        <button
          onClick={handleReset}
          className="p-2.5 text-xs font-medium text-gray-700 bg-white border border-gray-300 rounded-md hover:bg-gray-50 hover:border-gray-400 focus:outline-none focus:ring-2 focus:ring-blue-500 transition-colors"
          title="Reset View"
        >
          <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M3 10h10a8 8 0 018 8v2M3 10l6 6m-6-6l6-6" />
          </svg>
        </button>
        <button
          onClick={handleRefresh}
          disabled={isLoading}
          className="p-2.5 text-xs font-medium text-gray-700 bg-white border border-gray-300 rounded-md hover:bg-gray-50 hover:border-gray-400 focus:outline-none focus:ring-2 focus:ring-blue-500 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
          title="Refresh Data (Ctrl+R)"
        >
          <svg className={`w-5 h-5 ${isLoading ? 'animate-spin' : ''}`} fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
          </svg>
        </button>
        
        {/* Minimap toggle - Disabled for now */}
        <button
          disabled
          className="p-2.5 text-xs font-medium border border-gray-300 rounded-md transition-colors bg-gray-100 text-gray-400 cursor-not-allowed opacity-50"
          title="Minimap (Coming soon)"
        >
          <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 20l-5.447-2.724A1 1 0 013 16.382V5.618a1 1 0 011.447-.894L9 7m0 13l6-3m-6 3V7m6 10l4.553 2.276A1 1 0 0021 18.382V7.618a1 1 0 00-.553-.894L15 4m0 13V4m0 0L9 7" />
          </svg>
        </button>

        {/* Export buttons */}
        <div className="border-t border-gray-200 pt-1.5 mt-1 flex flex-col gap-1">
          <button
            onClick={() => handleExportScreenshot('png')}
            className="p-2 text-xs font-medium text-gray-700 bg-white border border-gray-300 rounded-md hover:bg-gray-50 hover:border-gray-400 focus:outline-none focus:ring-2 focus:ring-blue-500 transition-colors"
            title="Export as PNG"
          >
            <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4" />
            </svg>
          </button>
          <button
            onClick={() => handleExportScreenshot('svg')}
            className="p-2 text-xs font-medium text-gray-700 bg-white border border-gray-300 rounded-md hover:bg-gray-50 hover:border-gray-400 focus:outline-none focus:ring-2 focus:ring-blue-500 transition-colors"
            title="Export as SVG"
          >
            <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M7 21a4 4 0 01-4-4V5a2 2 0 012-2h4a2 2 0 012 2v12a4 4 0 01-4 4zm0 0h12a2 2 0 002-2v-4a2 2 0 00-2-2h-2.343M11 7.343l1.657-1.657a2 2 0 012.828 0l2.829 2.829a2 2 0 010 2.828l-8.486 8.485M7 17h.01" />
            </svg>
          </button>
        </div>
        
        <div className="px-3 py-1 text-xs text-gray-500 text-center border-t border-gray-200 mt-1 pt-1">
          {Math.round(zoomLevel * 100)}%
        </div>
      </div>

      {/* Info panel */}
      {data && (
        <div className="absolute bottom-4 left-4 z-10 bg-white/95 backdrop-blur-sm rounded-lg shadow-md border border-gray-200 p-3 text-xs">
          <div className="font-semibold text-gray-800 mb-2">Graph Info</div>
          <div className="text-gray-600 space-y-1">
            <div className="flex items-center gap-1">
              <svg className="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10" />
              </svg>
              <span>
                Nodes:{' '}
                <span className="font-medium text-gray-800">
                  {nodes.length}
                  {originalNodeCount ? ` / ${originalNodeCount}` : ''}
                </span>
              </span>
            </div>
            <div className="flex items-center gap-1">
              <svg className="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 10V3L4 14h7v7l9-11h-7z" />
              </svg>
              <span>
                Edges:{' '}
                <span className="font-medium text-gray-800">
                  {edges.length}
                  {originalEdgeCount ? ` / ${originalEdgeCount}` : ''}
                </span>
              </span>
            </div>
            {namespace && (
              <div className="flex items-center gap-1">
                <svg className="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z" />
                </svg>
                <span>Namespace: <span className="font-medium text-gray-800">{namespace}</span></span>
              </div>
            )}
          </div>
        </div>
      )}

      {/* Legend panel */}
      <div className="absolute bottom-4 right-4 z-10 bg-white/95 backdrop-blur-sm rounded-lg shadow-md border border-gray-200 p-3 text-xs">
        <div className="font-semibold text-gray-800 mb-2 flex items-center">
          <svg className="w-3.5 h-3.5 mr-1" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
          </svg>
          <span>Legend</span>
        </div>
        <div className="text-gray-600 space-y-1.5">
          <div className="flex items-center">
            <div className="w-3 h-3 rounded-sm mr-2" style={{ backgroundColor: '#2563eb' }}></div>
            <span>Namespace</span>
          </div>
          <div className="flex items-center">
            <div className="w-3 h-3 rounded-full mr-2" style={{ backgroundColor: '#f97316' }}></div>
            <span>Service Account</span>
          </div>
          <div className="flex items-center">
            <div className="w-3 h-3 transform rotate-45 mr-2" style={{ backgroundColor: '#9333ea' }}></div>
            <span>Role</span>
          </div>
          <div className="flex items-center">
            <div className="w-3 h-3 transform rotate-45 mr-2" style={{ backgroundColor: '#7c3aed' }}></div>
            <span>Cluster Role</span>
          </div>
          <div className="flex items-center">
            <div className="w-3 h-3 rounded-sm mr-2" style={{ backgroundColor: '#1e3a8a' }}></div>
            <span>Cluster</span>
          </div>
        </div>
        <div className="mt-2 pt-2 border-t border-gray-200 text-xs text-gray-500">
          <div className="flex items-center text-xs gap-1">
            <svg className="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
            <span>Hover over node to see connections</span>
          </div>
        </div>
      </div>

      {/* Tooltip for full text on hover */}
      {tooltipVisible && tooltipPosition && selectedNodeData && (
        <div
          className="fixed z-[9999] bg-gray-900 text-white text-xs px-3 py-2 rounded-lg shadow-lg pointer-events-none"
          style={{
            left: `${tooltipPosition.x + 10}px`,
            top: `${tooltipPosition.y - 30}px`,
            maxWidth: '300px',
            wordBreak: 'break-word',
          }}
        >
          {selectedNodeData.name}
        </div>
      )}

      {/* Tooltip for node details (only show if no right panel) */}
      {!onNodeSelect && tooltipVisible && tooltipPosition && selectedNodeType && selectedNodeData && selectedNodeId && (
        <div
          className="fixed z-[9999] bg-white rounded-lg shadow-xl border border-gray-200 p-4 max-w-md"
          style={{
            left: `${tooltipPosition.x + 10}px`,
            top: `${tooltipPosition.y + 10}px`,
            maxHeight: '80vh',
            overflowY: 'auto',
          }}
          onClick={(e) => e.stopPropagation()}
        >
          <div className="flex justify-between items-start mb-3">
            <h3 className="text-lg font-semibold text-gray-900 flex items-center">
              {selectedNodeType === 'serviceaccount' && (
                <svg className="w-5 h-5 mr-1.5 text-red-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z" />
                </svg>
              )}
              {selectedNodeType === 'role' && (
                <svg className="w-5 h-5 mr-1.5 text-purple-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z" />
                </svg>
              )}
              {selectedNodeType === 'clusterrole' && (
                <svg className="w-5 h-5 mr-1.5 text-purple-800" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z" />
                </svg>
              )}
              {selectedNodeType === 'namespace' && (
                <svg className="w-5 h-5 mr-1.5 text-blue-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z" />
                </svg>
              )}
              {selectedNodeType === 'cluster' && (
                <svg className="w-5 h-5 mr-1.5 text-gray-800" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 3v2m6-2v2M9 19v2m6-2v2M5 9H3m2 6H3m18-6h-2m2 6h-2M7 19h10a2 2 0 002-2V7a2 2 0 00-2-2H7a2 2 0 00-2 2v10a2 2 0 002 2zM9 9h6v6H9V9z" />
                </svg>
              )}
              {selectedNodeType === 'serviceaccount' ? 'ServiceAccount' : 
               selectedNodeType === 'role' ? 'Role' :
               selectedNodeType === 'clusterrole' ? 'ClusterRole' :
               selectedNodeType === 'namespace' ? 'Namespace' :
               selectedNodeType === 'cluster' ? 'Cluster' :
               'Node'} Details
            </h3>
            <button
              onClick={() => {
                setTooltipVisible(false)
                setSelectedNodeId(null)
                setSelectedNodeType(null)
                setSelectedNodeData(null)
              }}
              className="text-gray-400 hover:text-gray-600"
            >
              <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
              </svg>
            </button>
          </div>

          {selectedNodeType === 'serviceaccount' && isLoadingDetails ? (
            <div className="text-gray-500 text-sm flex items-center justify-center p-4">
              <svg className="animate-spin -ml-1 mr-3 h-5 w-5 text-blue-500" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
                <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
                <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
              </svg>
              <span>Loading details...</span>
            </div>
          ) : selectedNodeType === 'serviceaccount' && serviceAccount ? (
            <div className="space-y-2 text-sm">
              <div className="grid grid-cols-2 gap-2">
                <div className="text-gray-600 flex items-center">
                  <svg className="w-3 h-3 mr-1" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z" />
                  </svg>
                  <span>Name:</span>
                </div>
                <div className="font-medium text-gray-900">{serviceAccount.name}</div>
                
                <div className="text-gray-600 flex items-center">
                  <svg className="w-3 h-3 mr-1" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z" />
                  </svg>
                  <span>Namespace:</span>
                </div>
                <div className="font-medium text-gray-900">{serviceAccount.namespace}</div>
                
                <div className="text-gray-600 flex items-center">
                  <svg className="w-3 h-3 mr-1" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 3v2m6-2v2M9 19v2m6-2v2M5 9H3m2 6H3m18-6h-2m2 6h-2M7 19h10a2 2 0 002-2V7a2 2 0 00-2-2H7a2 2 0 00-2 2v10a2 2 0 002 2zM9 9h6v6H9V9z" />
                  </svg>
                  <span>Cluster:</span>
                </div>
                <div className="font-medium text-gray-900">{serviceAccount.clusterId}</div>
                
                <div className="text-gray-600 flex items-center">
                  <svg className="w-3 h-3 mr-1" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10 6H5a2 2 0 00-2 2v9a2 2 0 002 2h14a2 2 0 002-2V8a2 2 0 00-2-2h-5m-4 0V5a2 2 0 114 0v1m-4 0a2 2 0 104 0m-5 8a2 2 0 100-4 2 2 0 000 4zm0 0c1.306 0 2.417.835 2.83 2M9 14a3.001 3.001 0 00-2.83 2M15 11h3m-3 4h2" />
                  </svg>
                  <span>UID:</span>
                </div>
                <div className="font-mono text-xs text-gray-700 break-all">{serviceAccount.uid}</div>
                
                <div className="text-gray-600 flex items-center">
                  <svg className="w-3 h-3 mr-1" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 7V3m8 4V3m-9 8h10M5 21h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z" />
                  </svg>
                  <span>Created:</span>
                </div>
                <div className="text-gray-700">
                  {new Date(serviceAccount.createdAt).toLocaleString()}
                </div>
                
                <div className="text-gray-600 flex items-center">
                  <svg className="w-3 h-3 mr-1" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
                  </svg>
                  <span>Updated:</span>
                </div>
                <div className="text-gray-700">
                  {new Date(serviceAccount.updatedAt).toLocaleString()}
                </div>
              </div>

              {serviceAccount.labels && serviceAccount.labels !== '' && (
                <div className="mt-3 pt-3 border-t border-gray-200">
                  <div className="text-gray-600 mb-1 flex items-center">
                    <svg className="w-3 h-3 mr-1" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M7 7h.01M7 3h5c.512 0 1.024.195 1.414.586l7 7a2 2 0 010 2.828l-7 7a2 2 0 01-2.828 0l-7-7A1.994 1.994 0 013 12V7a4 4 0 014-4z" />
                    </svg>
                    <span>Labels:</span>
                  </div>
                  <div className="font-mono text-xs text-gray-700 break-all bg-gray-50 p-2 rounded">
                    {serviceAccount.labels}
                  </div>
                </div>
              )}

              {serviceAccount.secrets && serviceAccount.secrets !== '' && (
                <div className="mt-3 pt-3 border-t border-gray-200">
                  <div className="text-gray-600 mb-1 flex items-center">
                    <svg className="w-3 h-3 mr-1" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
                    </svg>
                    <span>Secrets:</span>
                  </div>
                  <div className="font-mono text-xs text-gray-700 break-all bg-gray-50 p-2 rounded">
                    {serviceAccount.secrets}
                  </div>
                </div>
              )}
            </div>
          ) : selectedNodeType === 'serviceaccount' && selectedNodeId ? (
            <div className="text-red-500 text-sm flex items-center">
              <svg className="w-4 h-4 mr-1.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
              </svg>
              <span>Failed to load details (ID: {selectedNodeId})</span>
            </div>
          ) : selectedNodeData ? (
            <div className="space-y-2 text-sm">
              <div className="grid grid-cols-2 gap-2">
                <div className="text-gray-600 flex items-center">
                  <svg className="w-3 h-3 mr-1" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M7 7h.01M7 3h5c.512 0 1.024.195 1.414.586l7 7a2 2 0 010 2.828l-7 7a2 2 0 01-2.828 0l-7-7A1.994 1.994 0 013 12V7a4 4 0 014-4z" />
                  </svg>
                  <span>Name:</span>
                </div>
                <div className="font-medium text-gray-900">{selectedNodeData.name || selectedNodeData.label || 'N/A'}</div>
                
                {selectedNodeData.cluster && (
                  <>
                    <div className="text-gray-600 flex items-center">
                      <svg className="w-3 h-3 mr-1" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 3v2m6-2v2M9 19v2m6-2v2M5 9H3m2 6H3m18-6h-2m2 6h-2M7 19h10a2 2 0 002-2V7a2 2 0 00-2-2H7a2 2 0 00-2 2v10a2 2 0 002 2zM9 9h6v6H9V9z" />
                      </svg>
                      <span>Cluster:</span>
                    </div>
                    <div className="font-medium text-gray-900">{selectedNodeData.cluster}</div>
                  </>
                )}
                
                {selectedNodeData.namespace && (
                  <>
                    <div className="text-gray-600 flex items-center">
                      <svg className="w-3 h-3 mr-1" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z" />
                      </svg>
                      <span>Namespace:</span>
                    </div>
                    <div className="font-medium text-gray-900">{selectedNodeData.namespace}</div>
                  </>
                )}
                
                {selectedNodeData.uid && (
                  <>
                    <div className="text-gray-600 flex items-center">
                      <svg className="w-3 h-3 mr-1" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10 6H5a2 2 0 00-2 2v9a2 2 0 002 2h14a2 2 0 002-2V8a2 2 0 00-2-2h-5m-4 0V5a2 2 0 114 0v1m-4 0a2 2 0 104 0m-5 8a2 2 0 100-4 2 2 0 000 4zm0 0c1.306 0 2.417.835 2.83 2M9 14a3.001 3.001 0 00-2.83 2M15 11h3m-3 4h2" />
                      </svg>
                      <span>UID:</span>
                    </div>
                    <div className="font-mono text-xs text-gray-700 break-all">{selectedNodeData.uid}</div>
                  </>
                )}
              </div>
              
              {selectedNodeType === 'role' && (
                <div className="mt-3 pt-3 border-t border-gray-200">
                  <div className="text-gray-600 mb-1 text-xs">
                    Role is scoped to namespace: <span className="font-medium">{selectedNodeData.namespace || 'N/A'}</span>
                  </div>
                </div>
              )}
              
              {selectedNodeType === 'clusterrole' && (
                <div className="mt-3 pt-3 border-t border-gray-200">
                  <div className="text-gray-600 mb-1 text-xs">
                    ClusterRole is cluster-scoped and applies to all namespaces.
                  </div>
                </div>
              )}
              
              {selectedNodeType === 'namespace' && (
                <div className="mt-3 pt-3 border-t border-gray-200">
                  <div className="text-gray-600 mb-1 text-xs">
                    Namespace contains ServiceAccounts, Roles, and RoleBindings.
                  </div>
                </div>
              )}
              
              {selectedNodeType === 'cluster' && (
                <div className="mt-3 pt-3 border-t border-gray-200">
                  <div className="text-gray-600 mb-1 text-xs">
                    Cluster contains all namespaces and cluster-scoped resources.
                  </div>
                </div>
              )}
            </div>
          ) : (
            <div className="text-gray-500 text-sm">No node selected</div>
          )}
        </div>
      )}

      {/* Graph container - Full size responsive */}
      <div 
        ref={containerRef} 
        className="absolute inset-0"
        style={{ 
          width: '100%',
          height: '100%',
          position: 'absolute',
          top: 0,
          left: 0,
          right: 0,
          bottom: 0,
          backgroundColor: 'transparent'
        }} 
      />
    </div>
  )
}

export default GraphVisualization