import { create } from 'zustand'
import { persist, createJSONStorage } from 'zustand/middleware'

interface ProjectState {
  activeProjectSlug: string | null
  setActiveProject: (slug: string | null) => void
}

export const useProjectStore = create<ProjectState>()(
  persist(
    (set) => ({
      activeProjectSlug: null,
      setActiveProject: (slug) => set({ activeProjectSlug: slug }),
    }),
    {
      name: 'flowy-project',
      storage: createJSONStorage(() => sessionStorage),
    }
  )
)
