import { create } from 'zustand';
import { persist } from 'zustand/middleware';

/** Global cluster filter: null = all clusters, string = cluster id */
interface ClusterFilterState {
  selectedClusterId: string | null;
  setSelectedClusterId: (id: string | null) => void;
}

/** Global cluster filter: null = all clusters, string = cluster id */
export const useClusterStore = create<ClusterFilterState>()(
  persist(
    (set) => ({
      selectedClusterId: null,
      setSelectedClusterId: (id) => set({ selectedClusterId: id }),
    }),
    {
      name: 'fortuna-cluster-filter',
      partialize: (s) => ({ selectedClusterId: s.selectedClusterId }),
    }
  )
);
