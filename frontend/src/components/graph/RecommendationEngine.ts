// RecommendationEngine.ts — 图谱推荐状态管理器
import { useReducer, useCallback, useRef } from 'react'
import { api } from '../../api/client'
import type { Recommendation, PathResult, Cluster, PlanningSuggestion } from '../../api/types'

// ---- State ----
export interface GraphAnalysisState {
  // 发现
  recommendations: Recommendation[]
  recommendationsByNode: Map<string, Recommendation[]>  // nodeId -> 该节点的推荐
  loadingRecommendations: boolean

  // 路径追溯
  activePath: PathResult | null
  loadingPath: boolean

  // 聚类
  clusters: Cluster[]
  focusedClusterId: string | null
  loadingClusters: boolean

  // 规划
  planningSuggestions: PlanningSuggestion[]
  loadingPlanning: boolean

  // UI 状态
  activePanel: 'discovery' | 'path' | 'cluster' | 'planning' | null
}

type Action =
  | { type: 'SET_RECOMMENDATIONS'; recommendations: Recommendation[] }
  | { type: 'SET_LOADING_RECOMMENDATIONS'; loading: boolean }
  | { type: 'SET_ACTIVE_PATH'; path: PathResult | null }
  | { type: 'SET_LOADING_PATH'; loading: boolean }
  | { type: 'SET_CLUSTERS'; clusters: Cluster[] }
  | { type: 'SET_FOCUSED_CLUSTER'; id: string | null }
  | { type: 'SET_LOADING_CLUSTERS'; loading: boolean }
  | { type: 'SET_PLANNING'; suggestions: PlanningSuggestion[] }
  | { type: 'SET_LOADING_PLANNING'; loading: boolean }
  | { type: 'SET_ACTIVE_PANEL'; panel: GraphAnalysisState['activePanel'] }
  | { type: 'REMOVE_RECOMMENDATION'; id: string }
  | { type: 'UPDATE_RECOMMENDATION_STATUS'; id: string; status: Recommendation['status'] }

function buildRecommendationsByNode(recs: Recommendation[]): Map<string, Recommendation[]> {
  const map = new Map<string, Recommendation[]>()
  for (const r of recs) {
    if (!map.has(r.source_node_id)) map.set(r.source_node_id, [])
    if (!map.has(r.target_node_id)) map.set(r.target_node_id, [])
    map.get(r.source_node_id)!.push(r)
    map.get(r.target_node_id)!.push(r)
  }
  return map
}

function reducer(state: GraphAnalysisState, action: Action): GraphAnalysisState {
  switch (action.type) {
    case 'SET_RECOMMENDATIONS': {
      const byNode = buildRecommendationsByNode(action.recommendations)
      return { ...state, recommendations: action.recommendations, recommendationsByNode: byNode, loadingRecommendations: false }
    }
    case 'SET_LOADING_RECOMMENDATIONS':
      return { ...state, loadingRecommendations: action.loading }
    case 'SET_ACTIVE_PATH':
      return { ...state, activePath: action.path, loadingPath: false }
    case 'SET_LOADING_PATH':
      return { ...state, loadingPath: action.loading }
    case 'SET_CLUSTERS':
      return { ...state, clusters: action.clusters, loadingClusters: false }
    case 'SET_FOCUSED_CLUSTER':
      return { ...state, focusedClusterId: action.id }
    case 'SET_LOADING_CLUSTERS':
      return { ...state, loadingClusters: action.loading }
    case 'SET_PLANNING':
      return { ...state, planningSuggestions: action.suggestions, loadingPlanning: false }
    case 'SET_LOADING_PLANNING':
      return { ...state, loadingPlanning: action.loading }
    case 'SET_ACTIVE_PANEL':
      return { ...state, activePanel: action.panel }
    case 'REMOVE_RECOMMENDATION': {
      const recs = state.recommendations.filter(r => r.id !== action.id)
      return { ...state, recommendations: recs, recommendationsByNode: buildRecommendationsByNode(recs) }
    }
    case 'UPDATE_RECOMMENDATION_STATUS': {
      const recs = state.recommendations.map(r => r.id === action.id ? { ...r, status: action.status } : r)
      return { ...state, recommendations: recs, recommendationsByNode: buildRecommendationsByNode(recs) }
    }
    default:
      return state
  }
}

const initialState: GraphAnalysisState = {
  recommendations: [],
  recommendationsByNode: new Map(),
  loadingRecommendations: false,
  activePath: null,
  loadingPath: false,
  clusters: [],
  focusedClusterId: null,
  loadingClusters: false,
  planningSuggestions: [],
  loadingPlanning: false,
  activePanel: null,
}

// ---- Hook ----
export function useGraphAnalysis(lakeId: string) {
  const [state, dispatch] = useReducer(reducer, initialState)

  // 用 ref 避免 stale closure：最新 state 始终在 ref 中
  const stateRef = useRef(state)
  // eslint-disable-next-line react-hooks/refs -- stale-closure pattern 必需：每次渲染同步最新 state 到 ref
  stateRef.current = state

  const loadRecommendations = useCallback(async () => {
    dispatch({ type: 'SET_LOADING_RECOMMENDATIONS', loading: true })
    try {
      const { recommendations } = await api.getRecommendations(lakeId)
      dispatch({ type: 'SET_RECOMMENDATIONS', recommendations })
    } catch {
      dispatch({ type: 'SET_LOADING_RECOMMENDATIONS', loading: false })
    }
  }, [lakeId])

  const loadClusters = useCallback(async () => {
    dispatch({ type: 'SET_LOADING_CLUSTERS', loading: true })
    try {
      const { clusters } = await api.getClusters(lakeId)
      dispatch({ type: 'SET_CLUSTERS', clusters })
    } catch {
      dispatch({ type: 'SET_LOADING_CLUSTERS', loading: false })
    }
  }, [lakeId])

  const loadPlanning = useCallback(async () => {
    dispatch({ type: 'SET_LOADING_PLANNING', loading: true })
    try {
      const { suggestions } = await api.getPlanningSuggestions(lakeId)
      dispatch({ type: 'SET_PLANNING', suggestions })
    } catch {
      dispatch({ type: 'SET_LOADING_PLANNING', loading: false })
    }
  }, [lakeId])

  const tracePath = useCallback(async (sourceId: string, targetId: string) => {
    dispatch({ type: 'SET_LOADING_PATH', loading: true })
    try {
      const path = await api.getPath(sourceId, targetId)
      dispatch({ type: 'SET_ACTIVE_PATH', path })
    } catch {
      dispatch({ type: 'SET_LOADING_PATH', loading: false })
    }
  }, [])

  const acceptRecommendation = useCallback(async (rec: Recommendation) => {
    await api.acceptRecommendation(rec.id, rec.source_node_id, rec.target_node_id)
    dispatch({ type: 'UPDATE_RECOMMENDATION_STATUS', id: rec.id, status: 'accepted' })
  }, [])

  const rejectRecommendation = useCallback(async (id: string) => {
    await api.rejectRecommendation(id)
    dispatch({ type: 'REMOVE_RECOMMENDATION', id })
  }, [])

  const ignoreRecommendation = useCallback(async (id: string) => {
    await api.ignoreRecommendation(id)
    dispatch({ type: 'REMOVE_RECOMMENDATION', id })
  }, [])

  const acceptPlanningSuggestion = useCallback(async (s: PlanningSuggestion) => {
    const res = await api.acceptPlanningSuggestion(s)
    if (res.status !== 'accepted') throw new Error('accept failed')
    return res
}, [])

  // 面板切换：使用 stateRef 避免 stale closure
  const setActivePanel = useCallback((panel: GraphAnalysisState['activePanel']) => {
    dispatch({ type: 'SET_ACTIVE_PANEL', panel })
    const s = stateRef.current
    if (panel === 'discovery' && s.recommendations.length === 0 && !s.loadingRecommendations) {
      loadRecommendations()
    } else if (panel === 'cluster' && s.clusters.length === 0 && !s.loadingClusters) {
      loadClusters()
    } else if (panel === 'planning' && s.planningSuggestions.length === 0 && !s.loadingPlanning) {
      loadPlanning()
    }
  }, [loadRecommendations, loadClusters, loadPlanning])

  const focusCluster = useCallback((id: string | null) => {
    dispatch({ type: 'SET_FOCUSED_CLUSTER', id })
  }, [])

  const closePath = useCallback(() => {
    dispatch({ type: 'SET_ACTIVE_PATH', path: null })
  }, [])

  const closePanel = useCallback(() => {
    dispatch({ type: 'SET_ACTIVE_PANEL', panel: null })
  }, [])

  // 获取某节点的 pending 推荐数（用于徽章）
  const getRecommendationCount = useCallback((nodeId: string): number => {
    const recs = stateRef.current.recommendationsByNode.get(nodeId) ?? []
    return recs.filter(r => r.status === 'pending').length
  }, [])

  return {
    state,
    loadRecommendations,
    loadClusters,
    loadPlanning,
    tracePath,
    acceptRecommendation,
    rejectRecommendation,
    ignoreRecommendation,
    acceptPlanningSuggestion,
    setActivePanel,
    focusCluster,
    closePath,
    closePanel,
    getRecommendationCount,
  }
}
